package ai

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"gptgo/internal/platform/config"
)

func TestBuiltinDraftWeeklyReport(t *testing.T) {
	raw := BuiltinDraft("做一个周报助手，只用三句话，先给结论", "builtin")
	var body draftBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	if body.Name != "周报助手" {
		t.Fatalf("name=%s", body.Name)
	}
	if !strings.Contains(body.Instructions, "三句话") || !strings.Contains(body.Instructions, "先给结论") {
		t.Fatalf("instructions=%s", body.Instructions)
	}
	if body.Opening != "把这周要写的内容发给我。" {
		t.Fatalf("opening=%s", body.Opening)
	}
	if len(body.Suggestions) < 3 {
		t.Fatalf("suggestions=%v", body.Suggestions)
	}
	if !validDraft(raw) {
		t.Fatal("内置草稿应该能通过校验")
	}
}

func TestBuiltinDraftNames(t *testing.T) {
	cases := []struct {
		prompt string
		name   string
	}{
		{"帮我创建一个 SQL 审核员", "SQL 审核员"},
		{"叫做会议纪要", "会议纪要"},
		{"翻译一下这段话", "翻译助手"},
		{"我想要一个能把英文合同翻成中文、并标出风险条款的助手", "合同助手"},
		{"连接 mysql 看看表", "新智能体"},
	}
	for _, item := range cases {
		raw := BuiltinDraft(item.prompt, "builtin")
		var body draftBody
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			t.Fatal(err)
		}
		if body.Name != item.name {
			t.Fatalf("%s -> %s，期望 %s", item.prompt, body.Name, item.name)
		}
	}
}

func TestRouterAuthorUsesBuiltinWithoutKey(t *testing.T) {
	router := NewRouter(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	raw, err := router.Author(context.Background(), "做一个周报助手，只用三句话", "minimax-m2.5")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "周报助手") {
		t.Fatalf("raw=%s", raw)
	}
}
