package ai

import (
	"encoding/json"
	"strings"
)

type draftBody struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Instructions string   `json:"instructions"`
	Opening      string   `json:"opening"`
	Suggestions  []string `json:"suggestions"`
}

// BuiltinDraft 把一句话需求写成智能体配置 JSON。没有外部模型时仍能生成可编辑的草稿。
func BuiltinDraft(prompt, _ string) string {
	text := strings.TrimSpace(prompt)
	if text == "" {
		text = "一个通用助手"
	}
	name := draftName(text)
	description := draftDescription(text, name)
	body := draftBody{
		Name:         name,
		Description:  description,
		Instructions: draftInstructions(name, description, text),
		Opening:      draftOpening(text, name),
		Suggestions:  draftSuggestions(text),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return `{"name":"新智能体","description":"负责回答问题","instructions":"用中文回答，先给结论，再给依据。","opening":"把要处理的内容发给我。","suggestions":["按设定处理下面的内容","先给结论，再说明依据"]}`
	}
	return string(raw)
}

func draftName(text string) string {
	markers := []string{"叫做", "名为", "名字是", "名称是", "取名为", "做一个", "创建一个", "生成一个", "设计一个", "做个", "建一个", "想要一个", "需要一个", "来一个", "叫"}
	for _, marker := range markers {
		idx := strings.Index(text, marker)
		if idx < 0 {
			continue
		}
		rest := text[idx+len(marker):]
		if marker == "叫" && strings.HasPrefix(rest, "做") {
			continue
		}
		name := cleanDraftName(cutClause(rest))
		if acceptableDraftName(name) {
			return name
		}
	}
	for _, item := range []struct{ key, name string }{
		{"会议纪要", "会议纪要"},
		{"周报", "周报助手"},
		{"日报", "日报助手"},
		{"翻译", "翻译助手"},
		{"合同", "合同助手"},
		{"客服", "客服助手"},
		{"简历", "简历助手"},
		{"邮件", "邮件助手"},
		{"文案", "文案助手"},
		{"问数", "问数助手"},
		{"数据库", "数据助手"},
		{"代码", "编程助手"},
		{"编程", "编程助手"},
	} {
		if strings.Contains(text, item.key) {
			return item.name
		}
	}
	if hasToken(text, "sql") {
		return "SQL 助手"
	}
	line := cleanDraftName(text)
	if line == cleanDraftName(cutClause(text)) && !strings.ContainsAny(line, " \t") && acceptableDraftName(line) && len([]rune(line)) <= 12 {
		return line
	}
	return "新智能体"
}

func cleanDraftName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "「」\"'“” ")
	for _, prefix := range []string{"一个", "一位", "一名"} {
		name = strings.TrimPrefix(name, prefix)
	}
	name = strings.TrimSpace(name)
	for _, suffix := range []string{"助手", "顾问", "老师", "专家"} {
		if idx := strings.Index(name, suffix); idx > 0 {
			name = strings.TrimSpace(name[:idx+len(suffix)])
			break
		}
	}
	return clipRunes(name, 40)
}

func acceptableDraftName(name string) bool {
	n := len([]rune(name))
	if n < 2 || n > 16 {
		return false
	}
	for _, prefix := range []string{"能", "可以", "把", "用", "帮", "请", "一个", "我"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

func cutClause(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	for i, r := range runes {
		if i == 0 {
			continue
		}
		switch r {
		case '，', '。', '、', ',', '.', '；', ';', '：', ':', '\n', '！', '!', '？', '?':
			return strings.TrimSpace(string(runes[:i]))
		}
	}
	return text
}

func draftDescription(text, name string) string {
	line := strings.TrimRight(clipRunes(strings.TrimSpace(text), 80), "，,。.")
	if line == "" || line == name {
		return clipRunes("负责「"+name+"」相关的问题", 200)
	}
	return line
}

func draftInstructions(name, description, text string) string {
	rules := []string{
		"用中文回答。",
		"先给结论，再给依据。",
		"不确定时直接说明，不编造用户没有提供的事实。",
	}
	if strings.Contains(text, "三句话") {
		rules = append([]string{"只用三句话回答。"}, rules...)
	}
	if strings.Contains(text, "代码") || strings.Contains(text, "编程") {
		rules = append(rules, "给出可以直接运行的示例，并说明边界情况。")
	}
	if strings.Contains(text, "表格") {
		rules = append(rules, "适合表格时用表格。")
	}
	if strings.Contains(text, "翻译") {
		rules = append(rules, "保留原文里的术语，拿不准的地方标明。")
	}
	var b strings.Builder
	b.WriteString("你是「" + name + "」。" + description + "\n\n回答时遵守：\n")
	for _, rule := range rules {
		b.WriteString("- " + rule + "\n")
	}
	b.WriteString("\n用户的原始要求：\n")
	b.WriteString(text)
	return clipRunes(strings.TrimSpace(b.String()), 4000)
}

func draftOpening(text, name string) string {
	switch {
	case strings.Contains(text, "周报"):
		return "把这周要写的内容发给我。"
	case strings.Contains(text, "日报"):
		return "把今天要写的内容发给我。"
	case strings.Contains(text, "翻译"):
		return "把需要翻译的文字发给我。"
	case strings.Contains(text, "代码"), strings.Contains(text, "编程"):
		return "把代码或问题发给我。"
	case strings.Contains(text, "会议"):
		return "把会议记录发给我。"
	case strings.Contains(text, "邮件"):
		return "把邮件对象和要说的事发给我。"
	case hasToken(text, "sql"), strings.Contains(text, "问数"), strings.Contains(text, "数据库"):
		return "说出你想查的数据。"
	default:
		return "你好，我是「" + name + "」。把要处理的内容发给我。"
	}
}

func draftSuggestions(text string) []string {
	switch {
	case strings.Contains(text, "周报"), strings.Contains(text, "日报"):
		return []string{"帮我写本周周报", "把下面的内容收成三句话", "先给结论再列依据"}
	case strings.Contains(text, "翻译"):
		return []string{"把这段话翻译成中文", "保留术语并标出不确定的地方", "把译文改得更正式"}
	case strings.Contains(text, "代码"), strings.Contains(text, "编程"):
		return []string{"看一下这段代码的问题", "补一个可运行示例", "说明边界情况"}
	case strings.Contains(text, "会议"):
		return []string{"整理这份会议记录", "列出决定和待办", "写成可以直接发出的纪要"}
	case hasToken(text, "sql"), strings.Contains(text, "问数"), strings.Contains(text, "数据库"):
		return []string{"这段需求该怎么查", "只保留只读查询", "用表格说明结果"}
	case strings.Contains(text, "邮件"):
		return []string{"写一封更清楚的邮件", "把语气改得正式一点", "缩短到三段以内"}
	default:
		return []string{"按设定处理下面的内容", "先给结论，再说明依据", "指出还缺哪些信息"}
	}
}

func hasToken(text, token string) bool {
	lower := strings.ToLower(text)
	token = strings.ToLower(token)
	start := 0
	for {
		i := strings.Index(lower[start:], token)
		if i < 0 {
			return false
		}
		i += start
		beforeOK := i == 0 || !isASCIILetter(rune(lower[i-1]))
		end := i + len(token)
		afterOK := end >= len(lower) || !isASCIILetter(rune(lower[end]))
		if beforeOK && afterOK {
			return true
		}
		start = i + len(token)
		if start >= len(lower) {
			return false
		}
	}
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func clipRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func extractObject(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "\uFEFF"))
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimSpace(raw)
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

func validDraft(raw string) bool {
	body := extractObject(raw)
	if body == "" {
		return false
	}
	var parsed struct {
		Name         string `json:"name"`
		Instructions string `json:"instructions"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return false
	}
	return strings.TrimSpace(parsed.Name) != "" && strings.TrimSpace(parsed.Instructions) != ""
}
