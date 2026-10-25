package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/search"
)

type recordingSearch struct {
	captureModels
	calls   int
	query   string
	readURL string
	fail    bool
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

func (s *recordingSearch) Read(_ context.Context, rawURL string) (search.Page, error) {
	s.readURL = rawURL
	if rawURL != "https://example.com/weather" {
		return search.Page{}, context.DeadlineExceeded
	}
	return search.Page{
		Title:   "上海天气",
		URL:     rawURL,
		Excerpt: "今日上海晴，最高 26 度，最低 18 度，东北风 3 级。空气质量为良，适合户外活动。",
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

func TestReadSourceFetchesSavedResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	resource := NewResource(repo, nil, fakeModels{}, "builtin")
	models := &recordingSearch{}
	controller := NewController(resource, models, nil)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: "source"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/stream", strings.NewReader(`{"content":"上海今天天气","model":"builtin","thinking_mode":"off","web_search":true}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	body := recorder.Body.String()
	conversation := conversationID(body)
	message := messageID(body)
	if conversation == "" || message == "" || !strings.Contains(body, "https://example.com/weather") {
		t.Fatalf("检索流没有保存来源: %s", body)
	}

	wrong := httptest.NewRecorder()
	bad := httptest.NewRequest(http.MethodPost, "/api/chat/source", strings.NewReader(`{"message_id":"`+message+`","url":"https://evil.example/secret"}`))
	bad.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(wrong, bad)
	if !strings.Contains(wrong.Body.String(), `"code":400`) || !strings.Contains(wrong.Body.String(), "不在当前回答") {
		t.Fatalf("名单外的地址不应被读取: %d %s", wrong.Code, wrong.Body.String())
	}
	if models.readURL != "" {
		t.Fatal("名单外的地址不应该发起网页读取")
	}

	page := postJSON(t, engine, "/api/chat/source", `{"message_id":"`+message+`","url":"https://example.com/weather"}`)
	data := page["data"].(map[string]any)
	excerpt, _ := data["excerpt"].(string)
	if models.readURL != "https://example.com/weather" || !strings.Contains(excerpt, "26 度") {
		t.Fatalf("明细: url=%s %#v", models.readURL, page)
	}
	messages := postJSON(t, engine, "/api/message/search", `{"conversation_id":"`+conversation+`","page":1,"page_size":20}`)
	saved, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "26 度") {
		t.Fatalf("明细没有写回消息: %s", saved)
	}

	messageNum, err := strconv.ParseInt(message, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	conversationNum, err := strconv.ParseInt(conversation, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	owner := domain.WithScope(context.Background(), domain.Scope{ActorID: 1, RequestID: "finish"})
	if _, err := resource.Complete(owner, &ChatCompleteRequest{
		ConversationID:     conversationNum,
		AssistantMessageID: messageNum,
		Content:            "最终回答",
		Sources:            `[{"title":"天气","url":"https://example.com/weather","snippet":"晴"}]`,
		Status:             "done",
	}); err != nil {
		t.Fatal(err)
	}
	again := postJSON(t, engine, "/api/message/search", `{"conversation_id":"`+conversation+`","page":1,"page_size":20}`)
	raw, err := json.Marshal(again)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "26 度") || !strings.Contains(string(raw), "最终回答") {
		t.Fatalf("收尾保存不应盖掉更长的正文: %s", raw)
	}

	other := domain.WithScope(context.Background(), domain.Scope{ActorID: 2, RequestID: "other"})
	_, err = resource.ReadSource(other, &SourceReadRequest{MessageID: messageNum, URL: "https://example.com/weather"}, models)
	appErr, ok := apperr.As(err)
	if !ok || appErr.Cat != apperr.CategoryNotFound {
		t.Fatalf("其他账号不应读到这条结果: %v", err)
	}
}

func messageID(body string) string {
	const marker = `"message_id":"`
	start := strings.LastIndex(body, marker)
	if start < 0 {
		return ""
	}
	rest := body[start+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
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
