package app

import (
	"bytes"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	sessionCookieName = "gptgo_session"
	csrfCookieName    = "gptgo_csrf"
)

type viewData struct {
	Title       string
	Name        string
	Email       string
	Error       string
	FieldErrors map[string]string
	CSRF        string
	User        *User
}

// Server 提供登录、注册和登录后的工作台页面。
type Server struct {
	store         *Store
	tmpl          *template.Template
	loginTries    *sliding
	registerTries *sliding
}

// NewServer 解析嵌入的页面模板。
func NewServer(store *Store) (*Server, error) {
	tmpl, err := template.New("root").Funcs(template.FuncMap{
		"zhDate": zhDate,
	}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		store:         store,
		tmpl:          tmpl,
		loginTries:    newSliding(8, time.Minute),
		registerTries: newSliding(5, time.Minute),
	}, nil
}

func zhDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.Local).Format("2006年1月2日 15:04")
}

// Handler 是带安全响应头的 HTTP 处理入口。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", s.static())
	mux.HandleFunc("GET /favicon.ico", s.favicon)
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /login", s.loginForm)
	mux.HandleFunc("POST /login", s.loginSubmit)
	mux.HandleFunc("GET /register", s.registerForm)
	mux.HandleFunc("POST /register", s.registerSubmit)
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("/", s.notFound)
	return s.recover(s.logRequests(s.secureHeaders(mux)))
}

func (s *Server) static() http.Handler {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/static/")
		if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(sub, name); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		http.StripPrefix("/static/", files).ServeHTTP(w, r)
	})
}

func (s *Server) favicon(w http.ResponseWriter, r *http.Request) {
	data, err := assets.ReadFile("static/favicon.svg")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(data)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data viewData, status int) {
	token, err := s.ensureCSRF(w, r)
	if err != nil {
		slog.Error("csrf", "err", err)
		http.Error(w, "页面暂时不可用", http.StatusInternalServerError)
		return
	}
	data.CSRF = token
	if data.FieldErrors == nil {
		data.FieldErrors = map[string]string{}
	}
	if data.Title == "" {
		data.Title = "GPTGO"
	}
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("render", "template", name, "err", err)
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) ensureCSRF(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && validToken(cookie.Value) {
		return cookie.Value, nil
	}
	token, err := newToken()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   12 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
	return token, nil
}

func csrfOK(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || !validToken(cookie.Value) {
		return false
	}
	form := r.PostFormValue("csrf")
	return subtleCompare(form, cookie.Value)
}

func (s *Server) currentUser(r *http.Request) (User, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return User{}, false
	}
	user, err := s.store.UserByToken(cookie.Value)
	if err != nil {
		return User{}, false
	}
	return user, true
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, expires, err := s.store.CreateSession(userID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
	return nil
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return os.Getenv("GPTGO_TRUST_PROXY") == "1" && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func readForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	return r.ParseForm()
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.code, "ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil || rec == http.ErrAbortHandler {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				return
			}
			slog.Error("panic", "err", rec, "path", r.URL.Path)
			http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}
