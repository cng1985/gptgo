package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIStreamsToolCalls(t *testing.T) {
	var sawToolRole bool
	var sawTools bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Errorf("payload: %v", err)
		}
		if _, ok := payload["tools"]; ok {
			sawTools = true
		}
		messages, _ := payload["messages"].([]any)
		for _, item := range messages {
			msg, _ := item.(map[string]any)
			if msg["role"] == "tool" && msg["tool_call_id"] == "call_1" {
				sawToolRole = true
			}
			if msg["role"] == "assistant" {
				if _, ok := msg["tool_calls"]; !ok && strings.Contains(string(raw), "call_1") {
					t.Errorf("助手工具调用没有保留: %s", raw)
				}
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_clock\",\"function\":{\"name\":\"clock\",\"arguments\":\"\"}}]}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"timezone\\\":\\\"Asia/Shanghai\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	client := &Client{baseURL: server.URL, apiKey: "test", client: server.Client()}
	ch, err := client.Stream(context.Background(), Request{
		Model: "deepseek-chat",
		Tools: []Tool{{Name: "clock", Description: "时间", Parameters: json.RawMessage(`{"type":"object","properties":{"timezone":{"type":"string"}}}`)}},
		Messages: []Message{
			{Role: "user", Content: "现在几点"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_1", Name: "clock", Arguments: "{}"}}},
			{Role: "tool", ToolCallID: "call_1", Content: "12:00"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls []ToolCall
	var content string
	for delta := range ch {
		if delta.Err != nil {
			t.Fatal(delta.Err)
		}
		content += delta.Content
		calls = append(calls, delta.ToolCalls...)
	}
	if !sawTools || !sawToolRole {
		t.Fatalf("请求没有带上工具定义或工具结果 sawTools=%v sawToolRole=%v", sawTools, sawToolRole)
	}
	if content != "" {
		t.Fatalf("参数碎片不应该变成正文: %s", content)
	}
	if len(calls) != 1 || calls[0].Name != "clock" || calls[0].ID != "call_clock" || !strings.Contains(calls[0].Arguments, "Asia/Shanghai") {
		t.Fatalf("工具调用没有拼完整: %#v", calls)
	}
}
