package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func testServer(t *testing.T) *Server {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	store.cost = bcrypt.MinCost
	server, err := NewServer(store)
	if err != nil {
		t.Fatal(err)
	}
	server.loginTries = newSliding(100, time.Minute)
	server.registerTries = newSliding(100, time.Minute)
	return server
}

func testClient() *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &http.Client{Jar: jar, Timeout: 5 * time.Second}
}

func noRedirect(base *http.Client) *http.Client {
	return &http.Client{
		Jar:     base.Jar,
		Timeout: base.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func readBody(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func csrfFrom(t *testing.T, body string) string {
	t.Helper()
	const needle = `name="csrf" value="`
	index := strings.Index(body, needle)
	if index < 0 {
		t.Fatalf("csrf token not found:\n%s", body)
	}
	rest := body[index+len(needle):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatal("csrf token unterminated")
	}
	return rest[:end]
}

func getPage(t *testing.T, client *http.Client, rawURL string) string {
	t.Helper()
	res, err := client.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status %d\n%s", rawURL, res.StatusCode, body)
	}
	return body
}

func postForm(t *testing.T, client *http.Client, rawURL string, form url.Values) (*http.Response, string) {
	t.Helper()
	res, err := client.PostForm(rawURL, form)
	if err != nil {
		t.Fatal(err)
	}
	return res, readBody(t, res)
}

func TestRegisterLoginLogout(t *testing.T) {
	server := testServer(t)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := testClient()

	registerPage := getPage(t, client, ts.URL+"/register")
	if !strings.Contains(registerPage, "创建账号") {
		t.Fatal("register page missing submit label")
	}
	res, _ := postForm(t, noRedirect(client), ts.URL+"/register", url.Values{
		"csrf":     {csrfFrom(t, registerPage)},
		"name":     {"林夏"},
		"email":    {"Lin@Example.com"},
		"password": {"correct-horse"},
		"confirm":  {"correct-horse"},
	})
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/" {
		t.Fatalf("register redirect = %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	var session *http.Cookie
	for _, cookie := range res.Cookies() {
		if cookie.Name == sessionCookieName {
			session = cookie
		}
	}
	if session == nil || !session.HttpOnly || session.Value == "" {
		t.Fatalf("session cookie = %#v", session)
	}

	var hash string
	if err := server.store.db.QueryRow(`SELECT password_hash FROM users WHERE email = ?`, "lin@example.com").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "correct-horse") || !strings.HasPrefix(hash, "$2") {
		t.Fatalf("password was not hashed: %s", hash)
	}

	home := getPage(t, client, ts.URL+"/")
	if !strings.Contains(home, "你好，林夏") || !strings.Contains(home, "lin@example.com") {
		t.Fatalf("home missing account:\n%s", home)
	}
	if strings.Contains(home, "correct-horse") {
		t.Fatal("home echoed the password")
	}

	loginPage, err := noRedirect(client).Get(ts.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	loginPage.Body.Close()
	if loginPage.StatusCode != http.StatusSeeOther || loginPage.Header.Get("Location") != "/" {
		t.Fatalf("logged-in login page status %d loc %q", loginPage.StatusCode, loginPage.Header.Get("Location"))
	}

	logoutRes, logoutBody := postForm(t, client, ts.URL+"/logout", url.Values{
		"csrf": {csrfFrom(t, home)},
	})
	if logoutRes.Request.URL.Path != "/login" || !strings.Contains(logoutBody, "登录") {
		t.Fatalf("logout landed on %s\n%s", logoutRes.Request.URL.Path, logoutBody)
	}

	wrongRes, wrongBody := postForm(t, client, ts.URL+"/login", url.Values{
		"csrf":     {csrfFrom(t, logoutBody)},
		"email":    {"lin@example.com"},
		"password": {"wrong-password"},
	})
	if wrongRes.StatusCode != http.StatusUnauthorized || !strings.Contains(wrongBody, "邮箱或密码不正确") {
		t.Fatalf("bad login = %d\n%s", wrongRes.StatusCode, wrongBody)
	}
	if strings.Contains(wrongBody, "wrong-password") {
		t.Fatal("login page echoed the password")
	}

	okRes, okBody := postForm(t, client, ts.URL+"/login", url.Values{
		"csrf":     {csrfFrom(t, wrongBody)},
		"email":    {"LIN@example.com"},
		"password": {"correct-horse"},
	})
	if okRes.StatusCode != http.StatusOK || !strings.Contains(okBody, "你好，林夏") {
		t.Fatalf("login final status %d\n%s", okRes.StatusCode, okBody)
	}
}

func TestRegisterValidationAndDuplicate(t *testing.T) {
	server := testServer(t)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := testClient()

	page := getPage(t, client, ts.URL+"/register")
	res, body := postForm(t, client, ts.URL+"/register", url.Values{
		"csrf": {csrfFrom(t, page)},
	})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "请填写昵称") || !strings.Contains(body, "请填写邮箱") {
		t.Fatalf("empty register = %d\n%s", res.StatusCode, body)
	}

	page = body
	res, body = postForm(t, client, ts.URL+"/register", url.Values{
		"csrf":     {csrfFrom(t, page)},
		"name":     {"林夏"},
		"email":    {"lin@example.com"},
		"password": {"short"},
		"confirm":  {"short"},
	})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "密码至少 8 位") {
		t.Fatalf("short password = %d\n%s", res.StatusCode, body)
	}

	page = body
	res, body = postForm(t, client, ts.URL+"/register", url.Values{
		"csrf":     {csrfFrom(t, page)},
		"name":     {"林夏"},
		"email":    {"lin@example.com"},
		"password": {"correct-horse"},
		"confirm":  {"other-horse"},
	})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "两次密码不一致") {
		t.Fatalf("mismatch = %d\n%s", res.StatusCode, body)
	}

	page = body
	res, body = postForm(t, client, ts.URL+"/register", url.Values{
		"csrf":     {csrfFrom(t, page)},
		"name":     {"林夏"},
		"email":    {"lin@example.com"},
		"password": {"correct-horse"},
		"confirm":  {"correct-horse"},
	})
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "你好，林夏") {
		t.Fatalf("first register = %d\n%s", res.StatusCode, body)
	}

	other := testClient()
	page = getPage(t, other, ts.URL+"/register")
	res, body = postForm(t, other, ts.URL+"/register", url.Values{
		"csrf":     {csrfFrom(t, page)},
		"name":     {"另一个"},
		"email":    {"LIN@example.com"},
		"password": {"correct-horse"},
		"confirm":  {"correct-horse"},
	})
	if res.StatusCode != http.StatusConflict || !strings.Contains(body, "该邮箱已注册") {
		t.Fatalf("duplicate = %d\n%s", res.StatusCode, body)
	}
}

func TestRegisterEscapesName(t *testing.T) {
	server := testServer(t)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := testClient()
	page := getPage(t, client, ts.URL+"/register")
	_, body := postForm(t, client, ts.URL+"/register", url.Values{
		"csrf":     {csrfFrom(t, page)},
		"name":     {`<script>alert(1)</script>`},
		"email":    {"lin@example.com"},
		"password": {"correct-horse"},
		"confirm":  {"correct-horse"},
	})
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("name was not escaped")
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("escaped name missing:\n%s", body)
	}
}

func TestCSRFAndGuestGuard(t *testing.T) {
	server := testServer(t)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := testClient()

	guest, err := noRedirect(client).Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	guest.Body.Close()
	if guest.StatusCode != http.StatusSeeOther || guest.Header.Get("Location") != "/login" {
		t.Fatalf("guest home = %d %q", guest.StatusCode, guest.Header.Get("Location"))
	}

	res, body := postForm(t, client, ts.URL+"/login", url.Values{
		"email":    {"lin@example.com"},
		"password": {"correct-horse"},
	})
	if res.StatusCode != http.StatusForbidden || !strings.Contains(body, "页面已过期") {
		t.Fatalf("missing csrf = %d\n%s", res.StatusCode, body)
	}

	missingRes, err := client.Get(ts.URL + "/missing")
	if err != nil {
		t.Fatal(err)
	}
	missing := readBody(t, missingRes)
	if missingRes.StatusCode != http.StatusNotFound || !strings.Contains(missing, "没有这个页面") {
		t.Fatalf("404 = %d\n%s", missingRes.StatusCode, missing)
	}
	css := getPage(t, client, ts.URL+"/static/app.css")
	if !strings.Contains(css, ".shell") {
		t.Fatal("stylesheet missing")
	}
}

func TestExpiredSession(t *testing.T) {
	server := testServer(t)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := testClient()
	page := getPage(t, client, ts.URL+"/register")
	_, _ = postForm(t, client, ts.URL+"/register", url.Values{
		"csrf":     {csrfFrom(t, page)},
		"name":     {"林夏"},
		"email":    {"lin@example.com"},
		"password": {"correct-horse"},
		"confirm":  {"correct-horse"},
	})
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := server.store.db.Exec(`UPDATE sessions SET expires_at = ?`, past); err != nil {
		t.Fatal(err)
	}
	res, err := noRedirect(client).Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/login" {
		t.Fatalf("expired session = %d %q", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestRateLimit(t *testing.T) {
	server := testServer(t)
	server.loginTries = newSliding(2, time.Minute)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)
	client := testClient()
	page := getPage(t, client, ts.URL+"/login")
	var lastStatus int
	var lastBody string
	for i := 0; i < 3; i++ {
		res, body := postForm(t, client, ts.URL+"/login", url.Values{
			"csrf":     {csrfFrom(t, page)},
			"email":    {"nobody@example.com"},
			"password": {"correct-horse"},
		})
		lastStatus = res.StatusCode
		lastBody = body
		page = body
	}
	if lastStatus != http.StatusTooManyRequests || !strings.Contains(lastBody, "尝试太频繁") {
		t.Fatalf("rate limit = %d\n%s", lastStatus, lastBody)
	}
}
