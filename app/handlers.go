package app

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	s.render(w, r, "home", viewData{Title: "工作台", User: &user}, http.StatusOK)
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderLogin(w, r, viewData{}, http.StatusOK)
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := readForm(w, r); err != nil {
		http.Error(w, "无法读取表单", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.PostFormValue("email"))
	password := r.PostFormValue("password")
	if !s.loginTries.allow(clientIP(r)) {
		s.renderLogin(w, r, viewData{Email: email, Error: "尝试太频繁，请稍后再试"}, http.StatusTooManyRequests)
		return
	}
	if !csrfOK(r) {
		s.renderLogin(w, r, viewData{Email: email, Error: "页面已过期，请刷新后重试"}, http.StatusForbidden)
		return
	}
	fields := validateLogin(email, password)
	if len(fields) > 0 {
		s.renderLogin(w, r, viewData{
			Email:       email,
			FieldErrors: fields,
			Error:       "请检查填写内容",
		}, http.StatusBadRequest)
		return
	}
	user, err := s.store.Authenticate(email, password)
	if errors.Is(err, ErrInvalidCredentials) {
		s.renderLogin(w, r, viewData{Email: email, Error: "邮箱或密码不正确"}, http.StatusUnauthorized)
		return
	}
	if err != nil {
		slog.Error("login", "err", err)
		s.renderLogin(w, r, viewData{Email: email, Error: "登录暂时不可用，请稍后再试"}, http.StatusInternalServerError)
		return
	}
	if err := s.startSession(w, r, user.ID); err != nil {
		slog.Error("session", "err", err)
		http.Error(w, "登录状态保存失败", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) registerForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderRegister(w, r, viewData{}, http.StatusOK)
}

func (s *Server) registerSubmit(w http.ResponseWriter, r *http.Request) {
	if err := readForm(w, r); err != nil {
		http.Error(w, "无法读取表单", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	email := strings.TrimSpace(r.PostFormValue("email"))
	password := r.PostFormValue("password")
	confirm := r.PostFormValue("confirm")
	if !s.registerTries.allow(clientIP(r)) {
		s.renderRegister(w, r, viewData{Name: name, Email: email, Error: "尝试太频繁，请稍后再试"}, http.StatusTooManyRequests)
		return
	}
	if !csrfOK(r) {
		s.renderRegister(w, r, viewData{Name: name, Email: email, Error: "页面已过期，请刷新后重试"}, http.StatusForbidden)
		return
	}
	fields := validateRegister(name, email, password, confirm)
	if len(fields) > 0 {
		s.renderRegister(w, r, viewData{
			Name:        name,
			Email:       email,
			FieldErrors: fields,
			Error:       "请检查填写内容",
		}, http.StatusBadRequest)
		return
	}
	user, err := s.store.CreateUser(name, email, password)
	if errors.Is(err, ErrEmailTaken) {
		s.renderRegister(w, r, viewData{
			Name:        name,
			Email:       email,
			Error:       "该邮箱已注册",
			FieldErrors: map[string]string{"email": "该邮箱已注册"},
		}, http.StatusConflict)
		return
	}
	if err != nil {
		slog.Error("register", "err", err)
		s.renderRegister(w, r, viewData{Name: name, Email: email, Error: "注册暂时不可用，请稍后再试"}, http.StatusInternalServerError)
		return
	}
	if err := s.startSession(w, r, user.ID); err != nil {
		slog.Error("session", "err", err)
		http.Error(w, "登录状态保存失败", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if err := readForm(w, r); err != nil {
		http.Error(w, "无法读取表单", http.StatusBadRequest)
		return
	}
	user, ok := s.currentUser(r)
	if !csrfOK(r) {
		if ok {
			s.render(w, r, "home", viewData{Title: "工作台", User: &user, Error: "页面已过期，请刷新后重试"}, http.StatusForbidden)
			return
		}
		s.renderLogin(w, r, viewData{Error: "页面已过期，请刷新后重试"}, http.StatusForbidden)
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := s.store.DeleteSession(cookie.Value); err != nil {
			slog.Error("logout", "err", err)
		}
	}
	clearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "notfound", viewData{Title: "页面不存在"}, http.StatusNotFound)
}

func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, data viewData, status int) {
	if data.Title == "" {
		data.Title = "登录"
	}
	s.render(w, r, "login", data, status)
}

func (s *Server) renderRegister(w http.ResponseWriter, r *http.Request, data viewData, status int) {
	if data.Title == "" {
		data.Title = "注册"
	}
	s.render(w, r, "register", data, status)
}
