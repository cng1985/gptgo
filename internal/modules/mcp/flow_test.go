package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gptgo/internal/modules/agent"
	"gptgo/internal/modules/chat"
	"gptgo/internal/modules/chatbi"
	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/database"
	"gptgo/internal/platform/domain"
)

func TestMCPServerFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "secret-token-value"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(body, &msg)
		w.Header().Set("Mcp-Session-Id", "sess-1")
		switch msg.Method {
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"tools":[{"name":"echo","description":"回声"}]}}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"protocolVersion":"2024-11-05","capabilities":{}}}`)
		}
	}))
	defer upstream.Close()

	db := openTestDB(t)
	box, err := chatbi.OpenCipher(filepath.Join(t.TempDir(), "datasource.key"))
	if err != nil {
		t.Fatal(err)
	}
	resource := NewResource(NewRepository(db, box))
	controller := NewController(resource)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		actor := int64(1)
		if c.GetHeader("X-Actor") == "2" {
			actor = 2
		}
		c.Request = c.Request.WithContext(domain.WithScope(c.Request.Context(), domain.Scope{ActorID: actor}))
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	tools := postJSON(t, engine, "/api/tool/search", `{}`, "1")
	if tools["code"].(float64) != 200 {
		t.Fatalf("tools: %#v", tools)
	}
	names := map[string]bool{}
	for _, item := range tools["data"].([]any) {
		names[item.(map[string]any)["name"].(string)] = true
	}
	if !names["clock"] || !names["ask_catalog"] {
		t.Fatalf("内置工具不全: %#v", tools["data"])
	}

	created := postJSON(t, engine, "/api/mcp/create", `{
		"name":"回声",
		"transport":"http",
		"url":"`+upstream.URL+`",
		"headers":{"Authorization":"`+secret+`"},
		"enabled":true
	}`, "1")
	if created["code"].(float64) != 200 {
		t.Fatalf("create: %#v", created)
	}
	raw, _ := json.Marshal(created)
	if strings.Contains(string(raw), secret) {
		t.Fatalf("响应泄露了请求头: %s", raw)
	}
	data := created["data"].(map[string]any)
	id := data["id"].(string)
	keys := data["header_keys"].([]any)
	if len(keys) != 1 || keys[0] != "Authorization" || data["headers_set"] != true {
		t.Fatalf("请求头摘要错误: %#v", data)
	}

	tested := postJSON(t, engine, "/api/mcp/test", `{"id":"`+id+`"}`, "1")
	if tested["code"].(float64) != 200 || len(tested["data"].(map[string]any)["tools"].([]any)) != 1 {
		t.Fatalf("test: %#v", tested)
	}

	kept := postJSON(t, engine, "/api/mcp/update", `{"id":"`+id+`","name":"回声服务","transport":"http","url":"`+upstream.URL+`"}`, "1")
	if kept["code"].(float64) != 200 || kept["data"].(map[string]any)["headers_set"] != true {
		t.Fatalf("省略请求头时应保留: %#v", kept)
	}
	again := postJSON(t, engine, "/api/mcp/test", `{"id":"`+id+`"}`, "1")
	if again["code"].(float64) != 200 {
		t.Fatalf("保留请求头后测试失败: %#v", again)
	}

	cleared := postJSON(t, engine, "/api/mcp/update", `{"id":"`+id+`","name":"回声服务","transport":"http","url":"`+upstream.URL+`","headers":{}}`, "1")
	if cleared["code"].(float64) != 200 || cleared["data"].(map[string]any)["headers_set"] != false {
		t.Fatalf("清空请求头失败: %#v", cleared)
	}

	missing := postJSON(t, engine, "/api/mcp/one", `{"id":"`+id+`"}`, "2")
	if missing["code"].(float64) != 404 {
		t.Fatalf("其他账号应返回 404: %#v", missing)
	}
	rejected := postJSON(t, engine, "/api/mcp/create", `{"name":"坏命令","transport":"stdio","command":"echo;rm"}`, "1")
	if rejected["code"].(float64) != 400 {
		t.Fatalf("危险命令应被拒绝: %#v", rejected)
	}

	for i := 0; i < 19; i++ {
		item := postJSON(t, engine, "/api/mcp/create", `{"name":"服务","transport":"http","url":"`+upstream.URL+`"}`, "1")
		if item["code"].(float64) != 200 {
			t.Fatalf("第 %d 个服务: %#v", i, item)
		}
	}
	limited := postJSON(t, engine, "/api/mcp/create", `{"name":"超出","transport":"http","url":"`+upstream.URL+`"}`, "1")
	if limited["code"].(float64) != 400 {
		t.Fatalf("超过 20 个应被拒绝: %#v", limited)
	}
}

func TestRunnerListsOwnedServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(body, &msg)
		if msg.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if msg.Method == "tools/list" {
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"tools":[{"name":"echo","description":"回声","inputSchema":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}}]}}`)
			return
		}
		if msg.Method == "tools/call" {
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"content":[{"type":"text","text":"pong"}]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"protocolVersion":"2024-11-05"}}`)
	}))
	defer upstream.Close()

	db := openTestDB(t)
	box, err := chatbi.OpenCipher(filepath.Join(t.TempDir(), "datasource.key"))
	if err != nil {
		t.Fatal(err)
	}
	servers := NewRepository(db, box)
	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 1})
	created, err := NewResource(servers).Create(ctx, &ServerRequest{
		Name: "回声", Transport: "http", URL: upstream.URL, Enabled: boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (&agentChecker{repo: servers}).Owns(ctx, []int64{created.Data.ID}); err != nil {
		t.Fatal(err)
	}
	other := domain.WithScope(context.Background(), domain.Scope{ActorID: 2})
	if err := servers.Owns(other, []int64{created.Data.ID}); err == nil {
		t.Fatal("其他账号不应拥有这个 MCP 服务")
	}

	agents := agent.NewRepository(db)
	saved, err := agent.NewResource(agents, servers).Create(ctx, &agent.AgentRequest{
		Name: "回声助手", Instructions: "需要时调用工具。", Model: "builtin",
		Tools: []string{"clock"}, McpIDs: agent.IDList{created.Data.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(agents, chat.NewRepository(db), servers)
	tools, err := runner.ForAgent(ctx, saved.Data.ID)
	if err != nil {
		t.Fatal(err)
	}
	var remote aiplat.Tool
	foundClock := false
	for _, tool := range tools {
		if tool.Name == "clock" {
			foundClock = true
		}
		if tool.RemoteName == "echo" {
			remote = tool
		}
	}
	if !foundClock || remote.Name == "" {
		t.Fatalf("智能体工具不完整: %#v", tools)
	}
	clockText, err := runner.Call(ctx, clockTool(), `{"timezone":"UTC"}`)
	if err != nil || len(clockText) < 19 {
		t.Fatalf("clock: %s %v", clockText, err)
	}
	text, err := runner.Call(ctx, remote, `{"text":"hi"}`)
	if err != nil || text != "pong" {
		t.Fatalf("mcp call: %s %v", text, err)
	}
}

type agentChecker struct{ repo *Repository }

func (c agentChecker) Owns(ctx context.Context, ids []int64) error { return c.repo.Owns(ctx, ids) }

func boolPtr(value bool) *bool { return &value }

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

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "app.db") + "?_pragma=foreign_keys(1)"
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
