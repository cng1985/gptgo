package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"gptgo/internal/platform/domain"
	"gptgo/internal/platform/search"
)

type recordingSearch struct {
	captureModels
	calls int
	query string
	fail  bool
}

func (s *recordingSearch) Search(_ context.Context, query string) (search.Result, error) {
	s.calls++
	s.query = query
	if s.fail {
		return search.Result{}, context.DeadlineExceeded
	}
	return search.Result{
		Notes: "公开结果：上海今天晴",
		Hits:  []search.Hit{{Title: "天气", URL: "https://example.com/weather", Snippet: "晴"}},
	}, nil
}

func TestWebSearchInjectsNotesAndSavesSources(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	resource := NewResource(repo, nil, fakeModels{}, "builtin")
	models := &recordingSearch{}
	controller := NewController(resource, models, nil)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: "search"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/stream", strings.NewReader(`{"content":"上海今天天气\n\n【附件 a.txt】\n密钥","model":"builtin","thinking_mode":"off","web_search":true}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	body := recorder.Body.String()
	if !strings.Contains(body, "event: search") || !strings.Contains(body, "https://example.com/weather") || !strings.Contains(body, "event: done") {
		t.Fatalf("检索流不完整: %s", body)
	}
	if models.calls != 1 || models.query != "上海今天天气" {
		t.Fatalf("检索词: %d %q", models.calls, models.query)
	}
	if models.got.SearchNotes != "公开结果：上海今天晴" {
		t.Fatalf("模型没有收到检索摘要: %#v", models.got.SearchNotes)
	}

	id := conversationID(body)
	one := postJSON(t, engine, "/api/conversation/one", `{"id":"`+id+`"}`)
	if one["data"].(map[string]any)["web_search"] != true {
		t.Fatalf("对话没有记住联网开关: %#v", one)
	}
	messages := postJSON(t, engine, "/api/message/search", `{"conversation_id":"`+id+`","page":1,"page_size":20}`)
	page, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "https://example.com/weather") {
		t.Fatalf("来源没有保存: %s", page)
	}

	models.calls = 0
	again := httptest.NewRecorder()
	plain := httptest.NewRequest(http.MethodPost, "/api/chat/stream", strings.NewReader(`{"conversation_id":"`+id+`","content":"再问一句","model":"builtin","thinking_mode":"off","web_search":false}`))
	plain.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(again, plain)
	if models.calls != 0 {
		t.Fatal("关闭联网时不应检索")
	}
}

func TestWebSearchFailureStillAnswers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	resource := NewResource(repo, nil, fakeModels{}, "builtin")
	models := &recordingSearch{fail: true}
	controller := NewController(resource, models, nil)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: "search-fail"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/stream", strings.NewReader(`{"content":"现在几点","model":"builtin","thinking_mode":"off","web_search":true}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	body := recorder.Body.String()
	if !strings.Contains(body, `"status":"error"`) || !strings.Contains(body, "按设定回答") {
		t.Fatalf("检索失败后应该继续回答: %s", body)
	}
	if models.got.SearchNotes != "本次联网检索没有返回结果。" {
		t.Fatalf("失败摘要: %q", models.got.SearchNotes)
	}
}

func TestSearchQueryUsesEarlierQuestion(t *testing.T) {
	query := searchQuery(ChatStreamRequest{}, []ModelTurn{{Role: "user", Content: "今天天气\n\n【附件 a.txt】\nsecret"}})
	if query != "今天天气" {
		t.Fatalf("检索词: %q", query)
	}
	direct := searchQuery(ChatStreamRequest{Content: "直接用这句"}, []ModelTurn{{Role: "user", Content: "旧的"}})
	if direct != "直接用这句" {
		t.Fatalf("当前问题应优先: %q", direct)
	}
}
