package ai

import (
	"context"
	"strings"
	"testing"
)

func TestBuiltinClockCallThenAnswer(t *testing.T) {
	builtin := &Builtin{}
	ctx := context.Background()
	first, err := builtin.Stream(ctx, Request{
		Tools:    []Tool{{Name: "clock", Description: "当前时间"}},
		Messages: []Message{{Role: "user", Content: "现在几点"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	delta := <-first
	if len(delta.ToolCalls) != 1 || delta.ToolCalls[0].Name != "clock" || delta.Content != "" {
		t.Fatalf("应该只发起 clock 调用: %#v", delta)
	}
	if _, ok := <-first; ok {
		t.Fatal("工具调用不应该再带正文")
	}

	second, err := builtin.Stream(ctx, Request{
		AgentName:    "报时",
		Instructions: "直接报时",
		Tools:        []Tool{{Name: "clock"}},
		Messages: []Message{
			{Role: "user", Content: "现在几点"},
			{Role: "assistant", ToolCalls: delta.ToolCalls},
			{Role: "tool", ToolCallID: delta.ToolCalls[0].ID, Content: "2026-10-24 20:00:00 Saturday Asia/Shanghai"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var content string
	for item := range second {
		if len(item.ToolCalls) > 0 {
			t.Fatal("已有工具结果时不应再次调用")
		}
		content += item.Content
	}
	if !strings.Contains(content, "我是「报时」") || !strings.Contains(content, "2026-10-24 20:00:00") {
		t.Fatalf("回答没有带上工具结果: %s", content)
	}
}

func TestBuiltinSkipsUnrelatedTools(t *testing.T) {
	builtin := &Builtin{}
	ch, err := builtin.Stream(context.Background(), Request{
		Tools:        []Tool{{Name: "clock"}, {Name: "m3_echo", RemoteName: "echo"}},
		Messages:     []Message{{Role: "user", Content: "你好"}},
		Instructions: "打招呼",
		AgentName:    "助手",
	})
	if err != nil {
		t.Fatal(err)
	}
	var content string
	for item := range ch {
		if len(item.ToolCalls) > 0 {
			t.Fatal("无关问题不应调用工具")
		}
		content += item.Content
	}
	if !strings.Contains(content, "我是「助手」") {
		t.Fatalf("普通回答丢失了智能体前缀: %s", content)
	}
}
