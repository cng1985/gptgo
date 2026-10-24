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

type clockSource struct{}

func (clockSource) ForAgent(context.Context, int64) ([]aiplat.Tool, error) {
	return []aiplat.Tool{{
		Name:        "clock",
		Description: "当前时间",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"timezone":{"type":"string"}}}`),
		Kind:        "builtin",
	}}, nil
}

func (clockSource) Call(context.Context, aiplat.Tool, string) (string, error) {
	return "2026-10-24 20:03:00 Saturday Asia/Shanghai", nil
}

func TestBuiltinClockLoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	repo := NewRepository(db)
	models := &aiplat.Builtin{Delay: 0}
	resource := NewResource(repo, stubAgents{}, models, clockSource{}, "builtin")
	controller := NewController(resource, models)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: "clock"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	controller.Register(engine.Group("/api"))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/stream", strings.NewReader(`{"content":"现在几点","model":"builtin","thinking_mode":"off","agent_id":"7"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	body := recorder.Body.String()
	if recorder.Code != 200 || !strings.Contains(body, "event: tool") || !strings.Contains(body, "2026-10-24 20:03:00") || !strings.Contains(body, "event: done") {
		t.Fatalf("工具循环不完整: %d %s", recorder.Code, body)
	}
	if !strings.Contains(body, "我是「周报助手」") {
		t.Fatalf("回答没有带上智能体: %s", body)
	}

	messages := postJSON(t, engine, "/api/message/search", `{"conversation_id":"`+conversationID(body)+`","page":1,"page_size":20}`)
	raw, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "clock") || !strings.Contains(string(raw), "2026-10-24 20:03:00") {
		t.Fatalf("工具轨迹没有保存: %s", raw)
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
