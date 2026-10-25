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

func TestBuiltinPlansCalculatorThenUsesResult(t *testing.T) {
	builtin := &Builtin{}
	first, err := builtin.Stream(context.Background(), Request{
		Messages: []Message{{Role: "user", Content: "请计算 (12+3)*4"}},
		Tools:    []Tool{{Name: "calculator", Description: "算式"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls []ToolCall
	for delta := range first {
		calls = append(calls, delta.ToolCalls...)
	}
	if len(calls) != 1 || calls[0].Name != "calculator" || !strings.Contains(calls[0].Arguments, "(12+3)*4") {
		t.Fatalf("没有规划计算器: %#v", calls)
	}

	second, err := builtin.Stream(context.Background(), Request{
		Messages: []Message{
			{Role: "user", Content: "请计算 (12+3)*4"},
			{Role: "assistant", ToolCalls: calls},
			{Role: "tool", Name: "calculator", ToolCallID: calls[0].ID, Content: "60"},
		},
		Tools: []Tool{{Name: "calculator", Description: "算式"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var answer strings.Builder
	for delta := range second {
		if len(delta.ToolCalls) > 0 {
			t.Fatalf("已经有结果时不应再次调用: %#v", delta.ToolCalls)
		}
		answer.WriteString(delta.Content)
	}
	if !strings.Contains(answer.String(), "60") {
		t.Fatalf("回答没有带上工具结果: %s", answer.String())
	}
}

func TestBuiltinIgnoresDateAndOrdinaryText(t *testing.T) {
	builtin := &Builtin{}
	stream, err := builtin.Stream(context.Background(), Request{
		Messages: []Message{{Role: "user", Content: "周报覆盖 2024-10-24 到 2024-10-25"}},
		Tools: []Tool{
			{Name: "calculator", Description: "算式"},
			{Name: "current_time", Description: "时间"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for delta := range stream {
		if len(delta.ToolCalls) > 0 {
			t.Fatalf("普通句子不应调用工具: %#v", delta.ToolCalls)
		}
	}
}

func TestBuiltinPlansNamedSkill(t *testing.T) {
	builtin := &Builtin{}
	stream, err := builtin.Stream(context.Background(), Request{
		Messages: []Message{{Role: "user", Content: "请使用技能 pdf-parser 处理附件"}},
		Tools:    []Tool{{Name: "load_skill", Description: "技能"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls []ToolCall
	for delta := range stream {
		calls = append(calls, delta.ToolCalls...)
	}
	if len(calls) != 1 || calls[0].Name != "load_skill" || !strings.Contains(calls[0].Arguments, "pdf-parser") {
		t.Fatalf("没有读取技能: %#v", calls)
	}

	plain, err := builtin.Stream(context.Background(), Request{
		Messages: []Message{{Role: "user", Content: "打开 skillhub 看看"}},
		Tools:    []Tool{{Name: "load_skill", Description: "技能"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for delta := range plain {
		if len(delta.ToolCalls) > 0 {
			t.Fatalf("skillhub 不应被当成技能名: %#v", delta.ToolCalls)
		}
	}
}

func TestPayloadIncludesSkillNotes(t *testing.T) {
	client := &Client{upstream: "upstream-model"}
	raw, err := client.payload(Request{
		Model:      "ignored",
		SkillNotes: "可用技能：pdf-parser。",
		Messages:   []Message{{Role: "user", Content: "你好"}},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "pdf-parser") {
		t.Fatalf("系统提示没有技能: %s", raw)
	}
}

func TestPayloadIncludesToolsAndToolMessages(t *testing.T) {
	client := &Client{upstream: "upstream-model"}
	raw, err := client.payload(Request{
		Model: "ignored",
		Tools: []Tool{{Name: "calculator", Description: "算式", Parameters: json.RawMessage(`{"type":"object"}`)}},
		Messages: []Message{
			{Role: "user", Content: "计算一下"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_1", Name: "calculator", Arguments: `{"expression":"1+1"}`}}},
			{Role: "tool", ToolCallID: "call_1", Content: "2"},
		},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "upstream-model" || payload["tool_choice"] != "auto" {
		t.Fatalf("payload: %s", raw)
	}
	tools, _ := payload["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools: %s", raw)
	}
	messages, _ := payload["messages"].([]any)
	if len(messages) != 4 {
		t.Fatalf("messages: %s", raw)
	}
}

func TestStreamReadsToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"calculator\",\"arguments\":\"\"}}]}}]}\n\n")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"expression\\\":\\\"1+1\\\"}\"}}]}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	client := &Client{baseURL: server.URL, apiKey: "test", client: server.Client()}
	stream, err := client.Stream(context.Background(), Request{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "算一下"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls []ToolCall
	for delta := range stream {
		if delta.Err != nil {
			t.Fatal(delta.Err)
		}
		calls = append(calls, delta.ToolCalls...)
	}
	if len(calls) != 1 || calls[0].ID != "call_1" || calls[0].Name != "calculator" || !strings.Contains(calls[0].Arguments, "1+1") {
		t.Fatalf("tool calls: %#v", calls)
	}
}
