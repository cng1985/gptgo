package skill

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
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

func TestSkillFlow(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Fatal("测试需要 bash")
	}
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	resource := NewResource(NewRepository(db))
	controller := NewController(resource)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		actor := int64(1)
		if c.GetHeader("X-Actor") == "2" {
			actor = 2
		}
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: actor, RequestID: "skill"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	created := postJSON(t, engine, "/api/skill/create", `{
		"title":"转成大写",
		"name":"upper_text",
		"description":"把 input 转成大写",
		"instructions":"只处理 input 字段",
		"runtime":"bash",
		"script":"cat",
		"timeout_sec":5,
		"enabled":true
	}`, "1")
	if created["code"].(float64) != 200 {
		t.Fatalf("create: %#v", created)
	}
	data := created["data"].(map[string]any)
	id := data["id"].(string)
	if data["parameters"] == "" || data["script"] == "" {
		t.Fatalf("创建结果缺字段: %#v", data)
	}

	page := postJSON(t, engine, "/api/skill/search", `{"keyword":"大写","page":1,"page_size":20}`, "1")
	if page["total"].(float64) != 1 {
		t.Fatalf("搜索结果错误: %#v", page)
	}
	other := postJSON(t, engine, "/api/skill/search", `{"page":1,"page_size":20}`, "2")
	if other["total"].(float64) != 0 {
		t.Fatalf("其他账号不应看到技能: %#v", other)
	}
	missing := postJSON(t, engine, "/api/skill/one", `{"id":"`+id+`"}`, "2")
	if missing["code"].(float64) != 404 {
		t.Fatalf("其他账号读取应返回 404: %#v", missing)
	}

	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 1})
	catalog, err := resource.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range catalog {
		if tool.Name == "upper_text" && strings.Contains(tool.Description, "转成大写") {
			found = true
		}
	}
	if !found {
		t.Fatalf("目录里没有技能: %#v", catalog)
	}

	ran := postJSON(t, engine, "/api/skill/run", `{
		"runtime":"bash",
		"script":"printf ok",
		"input":"{\"input\":\"$(echo hacked)\"}",
		"timeout_sec":5
	}`, "1")
	if ran["code"].(float64) != 200 {
		t.Fatalf("run: %#v", ran)
	}
	runData := ran["data"].(map[string]any)
	if runData["ok"] != true || strings.TrimSpace(runData["stdout"].(string)) != "ok" {
		t.Fatalf("参数不应被 shell 执行: %#v", runData)
	}

	t.Setenv("APP_AI_API_KEY", "secret-value")
	clean := postJSON(t, engine, "/api/skill/run", `{
		"runtime":"bash",
		"script":"if [ -n \"$APP_AI_API_KEY\" ]; then printf leaked; else printf clean; fi",
		"input":"{}",
		"timeout_sec":5
	}`, "1")
	cleanOut := clean["data"].(map[string]any)["stdout"].(string)
	if strings.TrimSpace(cleanOut) != "clean" || strings.Contains(cleanOut, "secret-value") {
		t.Fatalf("脚本不应读到应用密钥: %#v", clean["data"])
	}

	started := time.Now()
	timed := postJSON(t, engine, "/api/skill/run", `{
		"runtime":"bash",
		"script":"sleep 3; printf late",
		"input":"{}",
		"timeout_sec":1
	}`, "1")
	if time.Since(started) > 2500*time.Millisecond {
		t.Fatalf("超时没有停掉脚本: %s", time.Since(started))
	}
	timedData := timed["data"].(map[string]any)
	if timedData["ok"] == true || !strings.Contains(timedData["stderr"].(string), "已停止") {
		t.Fatalf("超时结果不对: %#v", timedData)
	}

	ok, text, err := resource.Invoke(ctx, "upper_text", `{"input":"Ab"}`)
	if err != nil || !ok || !strings.Contains(text, "Ab") {
		t.Fatalf("调用技能: %v %v %s", err, ok, text)
	}

	if _, err := resource.Update(ctx, &SkillRequest{
		ID: mustID(t, id), Title: "转成大写", Name: "upper_text", Description: "把 input 转成大写",
		Runtime: "bash", Script: "printf off", TimeoutSec: 5, Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	ok, text, err = resource.Invoke(ctx, "upper_text", `{}`)
	if err != nil || ok || !strings.Contains(text, "停用") {
		t.Fatalf("停用后仍可调用: %v %v %s", err, ok, text)
	}
	owned, err := resource.Owns(ctx, "upper_text")
	if err != nil || !owned {
		t.Fatalf("停用的技能仍应占用调用名: %v %v", err, owned)
	}

	duplicate := postJSON(t, engine, "/api/skill/create", `{
		"title":"重复","name":"upper_text","description":"重复","runtime":"bash","script":"printf 'x\\n'","enabled":true
	}`, "1")
	if duplicate["code"].(float64) != 409 {
		t.Fatalf("同名技能应被拒绝: %#v", duplicate)
	}
	reserved := postJSON(t, engine, "/api/skill/create", `{
		"title":"时间","name":"current_time","description":"占用","runtime":"bash","script":"printf 'x\\n'","enabled":true
	}`, "1")
	if reserved["code"].(float64) != 400 {
		t.Fatalf("内置工具名应被拒绝: %#v", reserved)
	}
	foreign := postJSON(t, engine, "/api/skill/run", `{"id":"`+id+`","input":"{}"}`, "2")
	if foreign["code"].(float64) != 404 {
		t.Fatalf("其他账号不应运行这个技能: %#v", foreign)
	}
}

func postJSON(t *testing.T, engine *gin.Engine, path, body, actor string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor", actor)
	engine.ServeHTTP(recorder, req)
	if recorder.Code != 200 {
		t.Fatalf("%s http %d %s", path, recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func mustID(t *testing.T, raw string) int64 {
	t.Helper()
	var id int64
	for _, r := range raw {
		if r < '0' || r > '9' {
			t.Fatalf("id %s", raw)
		}
		id = id*10 + int64(r-'0')
	}
	return id
}

func openTestDB(t *testing.T) *gorm.DB {
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
			_ = sqlDB.Close()
		}
	})
	return db
}
