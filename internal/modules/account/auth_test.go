package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gptgo/internal/platform/database"
	"gptgo/internal/platform/domain"
)

type stubProvider struct{}

func (stubProvider) Info() (string, bool) { return "builtin", false }

func TestRegisterLoginAndLogout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAuthDB(t)
	repo := NewRepository(db)
	resource := NewResource(repo, stubProvider{}, true, 8*time.Hour)
	controller := NewController(resource, 8*time.Hour)
	gate := NewGate(true, repo)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("request_id", "test")
		if scope, ok := gate.Resolve(c); ok {
			scope.RequestID = "test"
			c.Request = c.Request.WithContext(domain.WithScope(c.Request.Context(), scope))
		}
		c.Next()
	})
	api := engine.Group("/api")
	controller.Register(api)

	created := postAuth(t, engine, "/api/account/register", `{"username":"Ada","password":"secret1","name":"艾达"}`, nil)
	if created.code != 200 {
		t.Fatalf("register: %d %s", created.code, created.raw)
	}
	if created.body["data"].(map[string]any)["username"] != "ada" {
		t.Fatalf("用户名应转成小写: %#v", created.body)
	}
	if created.body["data"].(map[string]any)["name"] != "艾达" {
		t.Fatalf("显示名称不对: %#v", created.body)
	}
	if strings.Contains(created.raw, "password") {
		t.Fatalf("响应不应包含密码字段: %s", created.raw)
	}
	cookie := created.cookie
	if cookie == nil || cookie.Name != SessionCookie || cookie.Value == "" || !cookie.HttpOnly {
		t.Fatalf("会话 cookie 不正确: %#v", cookie)
	}

	anon := postAuth(t, engine, "/api/account/one", `{}`, nil)
	if anon.body["code"].(float64) != 401 {
		t.Fatalf("未登录应返回 401: %#v", anon.body)
	}
	me := postAuth(t, engine, "/api/account/one", `{}`, cookie)
	if me.body["data"].(map[string]any)["name"] != "艾达" {
		t.Fatalf("会话没有对上账号: %#v", me.body)
	}

	wrong := postAuth(t, engine, "/api/account/login", `{"username":"ada","password":"nope12"}`, nil)
	if wrong.body["code"].(float64) != 401 || wrong.body["msg"] != "用户名或密码错误" {
		t.Fatalf("错误密码: %#v", wrong.body)
	}
	dup := postAuth(t, engine, "/api/account/register", `{"username":"ada","password":"secret1"}`, nil)
	if dup.body["code"].(float64) != 409 {
		t.Fatalf("重复用户名: %#v", dup.body)
	}

	again := postAuth(t, engine, "/api/account/login", `{"username":"ADA","password":"secret1"}`, nil)
	if again.code != 200 {
		t.Fatalf("登录失败: %#v", again.body)
	}
	out := postAuth(t, engine, "/api/account/logout", `{}`, again.cookie)
	if out.body["code"].(float64) != 200 {
		t.Fatalf("退出失败: %#v", out.body)
	}
	after := postAuth(t, engine, "/api/account/one", `{}`, again.cookie)
	if after.body["code"].(float64) != 401 {
		t.Fatalf("退出后会话应失效: %#v", after.body)
	}
}

type authResult struct {
	code   int
	raw    string
	body   map[string]any
	cookie *http.Cookie
}

func postAuth(t *testing.T, engine *gin.Engine, path, body string, cookie *http.Cookie) authResult {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	engine.ServeHTTP(recorder, req)
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	result := authResult{code: recorder.Code, raw: recorder.Body.String(), body: payload}
	for _, item := range recorder.Result().Cookies() {
		if item.Name == SessionCookie && item.Value != "" {
			result.cookie = item
		}
	}
	return result
}

func openAuthDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := (&url.URL{Scheme: "file", Path: filepath.Join(dir, "app.db"), RawQuery: "_pragma=foreign_keys(1)"}).String()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return db
}
