package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gptgo/internal/platform/database"
	"gptgo/internal/platform/domain"
)

func TestAgentFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	resource := NewResource(repo, nil)
	controller := NewController(resource)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		actor := int64(1)
		if c.GetHeader("X-Actor") == "2" {
			actor = 2
		}
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: actor, RequestID: "test"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	created := postJSON(t, engine, "/api/agent/create", `{
		"name":"周报助手",
		"description":"把一周工作收成三句话",
		"instructions":"只用三句话回答，先给结论。",
		"model":"builtin",
		"opening":"把这周要写的内容发给我。",
		"suggestions":["帮我写本周周报","", "太长的建议会被去掉"]
	}`, "1")
	if created["code"].(float64) != 200 {
		t.Fatalf("create: %#v", created)
	}
	id := created["data"].(map[string]any)["id"].(string)
	suggestions := created["data"].(map[string]any)["suggestions"].([]any)
	if len(suggestions) != 2 {
		t.Fatalf("空建议应该被去掉: %#v", suggestions)
	}

	page := postJSON(t, engine, "/api/agent/search", `{"keyword":"周报","page":1,"page_size":20}`, "1")
	if page["total"].(float64) != 1 {
		t.Fatalf("搜索结果错误: %#v", page)
	}
	other := postJSON(t, engine, "/api/agent/search", `{"page":1,"page_size":20}`, "2")
	if other["total"].(float64) != 0 {
		t.Fatalf("其他账号不应看到智能体: %#v", other)
	}
	missing := postJSON(t, engine, "/api/agent/one", `{"id":"`+id+`"}`, "2")
	if missing["code"].(float64) != 404 {
		t.Fatalf("其他账号读取应返回 404: %#v", missing)
	}

	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 1})
	name, instructions, err := resource.Prompt(ctx, mustID(t, id))
	if err != nil || name != "周报助手" || !strings.Contains(instructions, "三句话") {
		t.Fatalf("prompt name=%s instructions=%s err=%v", name, instructions, err)
	}

	updated := postJSON(t, engine, "/api/agent/update", `{"id":"`+id+`","name":"周报助手","instructions":"先列完成的事。","model":"builtin","suggestions":["重写本周周报"]}`, "1")
	if updated["code"].(float64) != 200 || updated["data"].(map[string]any)["instructions"] != "先列完成的事。" {
		t.Fatalf("update: %#v", updated)
	}
	deleted := postJSON(t, engine, "/api/agent/delete", `{"id":"`+id+`"}`, "1")
	if deleted["code"].(float64) != 200 {
		t.Fatalf("delete: %#v", deleted)
	}
	gone := postJSON(t, engine, "/api/agent/one", `{"id":"`+id+`"}`, "1")
	if gone["code"].(float64) != 404 {
		t.Fatalf("删除后应不存在: %#v", gone)
	}
}

func TestAgentRejectsEmptyInstructions(t *testing.T) {
	db := openTestDB(t)
	resource := NewResource(NewRepository(db), nil)
	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 1})
	if _, err := resource.Create(ctx, &AgentRequest{Name: "空设定", Model: "builtin"}); err == nil {
		t.Fatal("没有设定的智能体应该被拒绝")
	}
}

func TestAgentDraft(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	author := fakeAuthor{raw: "```json\n{\"name\":\"周报助手\",\"description\":\"把一周工作收成三句话\",\"instructions\":\"只用三句话回答，先给结论。\",\"opening\":\"把这周要写的内容发给我。\",\"suggestions\":\"帮我写本周周报\\n先给结论\"}\n```"}
	resource := NewResource(NewRepository(db), author)
	controller := NewController(resource)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: "test"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	drafted := postJSON(t, engine, "/api/agent/draft", `{"prompt":"做一个周报助手，只用三句话","model":"builtin"}`, "1")
	if drafted["code"].(float64) != 200 {
		t.Fatalf("draft: %#v", drafted)
	}
	data := drafted["data"].(map[string]any)
	if data["name"] != "周报助手" || data["model"] != "builtin" {
		t.Fatalf("草稿字段错误: %#v", data)
	}
	if !strings.Contains(data["instructions"].(string), "三句话") {
		t.Fatalf("设定没有保留要求: %#v", data["instructions"])
	}
	suggestions := data["suggestions"].([]any)
	if len(suggestions) != 2 || suggestions[0] != "帮我写本周周报" {
		t.Fatalf("建议问题: %#v", suggestions)
	}
	page := postJSON(t, engine, "/api/agent/search", `{"page":1,"page_size":20}`, "1")
	if page["total"].(float64) != 0 {
		t.Fatalf("草稿不应写入数据库: %#v", page)
	}

	named := postJSON(t, engine, "/api/agent/draft", `{"prompt":"用 DeepSeek 做一个周报助手","model":"builtin"}`, "1")
	if named["data"].(map[string]any)["model"] != "deepseek-chat" {
		t.Fatalf("描述里的模型没有生效: %#v", named)
	}
	empty := postJSON(t, engine, "/api/agent/draft", `{"prompt":"  ","model":"builtin"}`, "1")
	if empty["code"].(float64) != 400 {
		t.Fatalf("空描述应拒绝: %#v", empty)
	}
	unknown := postJSON(t, engine, "/api/agent/draft", `{"prompt":"周报助手","model":"gpt-4"}`, "1")
	if unknown["code"].(float64) != 400 {
		t.Fatalf("未知模型应拒绝: %#v", unknown)
	}

	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 1})
	longPrompt := strings.Repeat("周", 1001)
	if _, err := resource.Draft(ctx, &AgentDraftRequest{Prompt: longPrompt, Model: "builtin"}); err == nil {
		t.Fatal("超长描述应该被拒绝")
	}
	if _, err := resource.Draft(context.Background(), &AgentDraftRequest{Prompt: "周报", Model: "builtin"}); err == nil {
		t.Fatal("未登录不能生成草稿")
	}
	broken := NewResource(NewRepository(db), fakeAuthor{err: errBoom})
	if _, err := broken.Draft(ctx, &AgentDraftRequest{Prompt: "周报助手", Model: "builtin"}); err == nil {
		t.Fatal("生成失败应该返回错误")
	}
}

type fakeAuthor struct {
	raw string
	err error
}

func (f fakeAuthor) Author(context.Context, string, string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.raw, nil
}

var errBoom = boomError{}

type boomError struct{}

func (boomError) Error() string { return "boom" }

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
			sqlDB.Close()
		}
	})
	return db
}
