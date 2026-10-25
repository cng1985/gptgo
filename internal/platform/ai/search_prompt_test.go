package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"gptgo/internal/platform/search"
)

func TestSearchNotesStayInLeadingSystemPrompt(t *testing.T) {
	client := &Client{upstream: "gpt-4o-mini"}
	raw, err := client.payload(Request{
		Model:       "minimax-m3.1-flash",
		Messages:    []Message{{Role: "user", Content: "上海今天天气"}},
		SearchNotes: "1. 上海天气\nhttps://example.com/weather\n今日晴",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Messages) != 2 || payload.Messages[0].Role != "system" || payload.Messages[1].Role != "user" {
		t.Fatalf("检索材料应该留在第一条系统提示里: %#v", payload.Messages)
	}
	if !strings.Contains(payload.Messages[0].Content, "今日晴") || strings.Contains(payload.Messages[1].Content, "今日晴") {
		t.Fatalf("系统提示: %s", payload.Messages[0].Content)
	}
}

func TestComposeAnswersFromSearchNotes(t *testing.T) {
	_, answer := Compose("上海今天天气", "1. 上海天气\nhttps://example.com/weather\n今日晴", "off")
	if !strings.Contains(answer, "今日晴") || strings.Contains(answer, "围绕「上海今天天气」") {
		t.Fatalf("内置助手应直接依据检索材料回答: %s", answer)
	}
	_, missed := Compose("上海今天天气", search.UnavailableNote, "off")
	if !strings.Contains(missed, "没有检索到") || !strings.Contains(missed, "上海今天天气") {
		t.Fatalf("没有结果时仍应回答问题: %s", missed)
	}
}
