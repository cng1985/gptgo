package agent

import (
	"encoding/json"
	"strings"
)

func decodeDraft(raw string) (AgentDraft, bool) {
	body := extractJSONObject(raw)
	if body == "" {
		return AgentDraft{}, false
	}
	var parsed struct {
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		Instructions string          `json:"instructions"`
		Opening      string          `json:"opening"`
		Suggestions  json.RawMessage `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return AgentDraft{}, false
	}
	return AgentDraft{
		Name:         parsed.Name,
		Description:  parsed.Description,
		Instructions: parsed.Instructions,
		Opening:      parsed.Opening,
		Suggestions:  parseSuggestions(parsed.Suggestions),
	}, true
}

func parseSuggestions(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		return items
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		parts := strings.FieldsFunc(text, func(r rune) bool {
			return r == '\n' || r == '；' || r == ';'
		})
		return parts
	}
	return nil
}

func clampDraft(draft AgentDraft, prompt, model string) AgentDraft {
	draft.Name = clipRunes(strings.TrimSpace(draft.Name), 40)
	if draft.Name == "" {
		draft.Name = "新智能体"
	}
	draft.Description = clipRunes(strings.TrimSpace(draft.Description), 200)
	draft.Instructions = clipRunes(strings.TrimSpace(draft.Instructions), 4000)
	if draft.Instructions == "" {
		draft.Instructions = clipRunes("用中文回答。用户要求：\n"+strings.TrimSpace(prompt), 4000)
	}
	draft.Opening = clipRunes(strings.TrimSpace(draft.Opening), 500)
	if draft.Opening == "" {
		draft.Opening = "你好，我是「" + draft.Name + "」。把要处理的内容发给我。"
	}
	draft.Model = chooseModel(prompt, model)
	cleaned := make([]string, 0, 4)
	seen := map[string]struct{}{}
	for _, item := range draft.Suggestions {
		item = clipRunes(strings.TrimSpace(item), 80)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		cleaned = append(cleaned, item)
		if len(cleaned) == 4 {
			break
		}
	}
	if len(cleaned) == 0 {
		cleaned = []string{"按设定处理下面的内容", "先给结论，再说明依据"}
	}
	draft.Suggestions = cleaned
	return draft
}

func chooseModel(prompt, requested string) string {
	picked := strings.TrimSpace(requested)
	lower := strings.ToLower(prompt)
	switch {
	case strings.Contains(lower, "deepseek"):
		picked = "deepseek-chat"
	case strings.Contains(lower, "minimax") && (strings.Contains(lower, "m2.5") || strings.Contains(lower, "2.5")):
		picked = "minimax-m2.5"
	case strings.Contains(lower, "minimax"):
		picked = "minimax-m3.1-flash"
	case strings.Contains(prompt, "内置助手"), strings.Contains(prompt, "内置模型"):
		picked = "builtin"
	}
	if _, ok := Models[picked]; !ok {
		return "builtin"
	}
	return picked
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "\uFEFF"))
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSpace(raw)
		if len(raw) >= 4 && strings.EqualFold(raw[:4], "json") {
			raw = strings.TrimSpace(raw[4:])
		}
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = strings.TrimSpace(raw[:idx])
		}
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return ""
	}
	return raw[start : end+1]
}

func clipRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
