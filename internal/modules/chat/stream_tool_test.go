package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/domain"
)

type scriptedModels struct {
	calls int
}

func (s *scriptedModels) Stream(_ context.Context, _ aiplat.Request) (<-chan aiplat.Delta, error) {
	s.calls++
	ch := make(chan aiplat.Delta, 1)
	if s.calls == 1 {
		ch <- aiplat.Delta{ToolCalls: []aiplat.ToolCall{{
			ID: "call_1", Name: "calculator", Arguments: `{"expression":"1+1"}`,
		}}}
	} else {
		ch <- aiplat.Delta{Content: "结果是 2"}
	}
	close(ch)
	return ch, nil
}

type echoTools struct{}

func (echoTools) Catalog(context.Context) ([]aiplat.Tool, error) {
	return []aiplat.Tool{{Name: "calculator", Description: "算式"}}, nil
}

func (echoTools) Invoke(context.Context, string, string) (bool, string, error) {
	return true, "2", nil
}

func TestStreamRunsToolThenAnswers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	resource := NewResource(repo, nil, fakeModels{}, "builtin")
	models := &scriptedModels{}
	controller := NewController(resource, models, echoTools{})
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: "tool"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/stream", strings.NewReader(`{"content":"计算 1+1","model":"builtin","thinking_mode":"off"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	body := recorder.Body.String()
	if !strings.Contains(body, "event: tool") || !strings.Contains(body, "calculator") || !strings.Contains(body, "结果是 2") || !strings.Contains(body, "event: done") {
		t.Fatalf("工具流不完整: %s", body)
	}
	if models.calls != 2 {
		t.Fatalf("模型调用次数: %d", models.calls)
	}
	messages := postJSON(t, engine, "/api/message/search", `{"conversation_id":"`+conversationID(body)+`","page":1,"page_size":20}`)
	raw, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "calculator") || !strings.Contains(string(raw), `"result":"2"`) {
		t.Fatalf("工具记录没有保存: %s", raw)
	}
}

func conversationID(body string) string {
	const marker = `"conversation_id":"`
	start := strings.Index(body, marker)
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
