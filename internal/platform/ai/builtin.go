package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Request struct {
	Model        string
	Messages     []Message
	ThinkingMode string
	SearchNotes  string
	AgentName    string
	Instructions string
	SkillNotes   string
	Tools        []Tool
}

type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type Message struct {
	Role       string
	Content    string
	Name       string
	ToolCallID string
	ToolCalls  []ToolCall
}

type Delta struct {
	Thinking  string
	Content   string
	ToolCalls []ToolCall
	Err       error
}

type Builtin struct {
	Delay time.Duration
}

func (b *Builtin) Stream(ctx context.Context, req Request) (<-chan Delta, error) {
	if calls := planToolCalls(req); len(calls) > 0 {
		ch := make(chan Delta, 1)
		go func() {
			defer close(ch)
			select {
			case <-ctx.Done():
			case ch <- Delta{ToolCalls: calls}:
			}
		}()
		return ch, nil
	}
	question := lastUser(req.Messages)
	thinking, answer := Compose(question, req.SearchNotes, req.ThinkingMode)
	if notes := toolResultNotes(req.Messages); notes != "" {
		answer = "结论来自这次工具调用。\n\n" + notes
	}
	if strings.TrimSpace(req.Instructions) != "" {
		name := strings.TrimSpace(req.AgentName)
		if name == "" {
			name = "智能体"
		}
		answer = "我是「" + name + "」，按这个智能体的设定来回答。\n\n" + answer
	}
	ch := make(chan Delta, 16)
	go func() {
		defer close(ch)
		if thinking != "" && !writeChunks(ctx, ch, thinking, true, b.Delay) {
			return
		}
		writeChunks(ctx, ch, answer, false, b.Delay)
	}()
	return ch, nil
}

func (b *Builtin) Optimize(_ context.Context, content, _ string) (string, error) {
	return optimizeText(content), nil
}

func (b *Builtin) Author(_ context.Context, prompt, model string) (string, error) {
	return BuiltinDraft(prompt, model), nil
}

func planToolCalls(req Request) []ToolCall {
	if len(req.Tools) == 0 {
		return nil
	}
	for _, message := range req.Messages {
		if message.Role == "tool" || len(message.ToolCalls) > 0 {
			return nil
		}
	}
	available := make(map[string]struct{}, len(req.Tools))
	for _, tool := range req.Tools {
		available[tool.Name] = struct{}{}
	}
	question := lastUser(req.Messages)
	var calls []ToolCall
	seen := map[string]struct{}{}
	add := func(name, args string) {
		if _, ok := available[name]; !ok {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		args = strings.TrimSpace(args)
		if args == "" {
			args = "{}"
		}
		seen[name] = struct{}{}
		calls = append(calls, ToolCall{
			ID:        fmt.Sprintf("call_%s_%d", name, len(calls)+1),
			Name:      name,
			Arguments: args,
		})
	}
	object := firstJSONObject(question)
	for _, match := range invokePattern.FindAllStringSubmatch(question, 4) {
		args := "{}"
		if object != "" {
			args = object
		}
		add(match[1], args)
	}
	if _, ok := available["current_time"]; ok {
		if containsAny(question, "几点", "当前时间", "现在时间", "什么时间", "今天几号", "今天星期", "当前日期", "现在日期") ||
			containsAny(strings.ToLower(question), "what time", "current time", "current date") {
			tz := timezonePattern.FindString(question)
			if tz == "" && strings.Contains(strings.ToUpper(question), "UTC") {
				tz = "UTC"
			}
			if tz == "" {
				add("current_time", "{}")
			} else if raw, err := json.Marshal(map[string]string{"timezone": tz}); err == nil {
				add("current_time", string(raw))
			}
		}
	}
	if _, ok := available["load_skill"]; ok {
		if name := skillMention(question); name != "" {
			if raw, err := json.Marshal(map[string]string{"name": name}); err == nil {
				add("load_skill", string(raw))
			}
		}
	}
	if _, ok := available["calculator"]; ok {
		if expr := extractExpr(question); expr != "" && wantsCalc(question, expr) {
			if raw, err := json.Marshal(map[string]string{"expression": expr}); err == nil {
				add("calculator", string(raw))
			}
		}
	}
	return calls
}

func toolResultNotes(messages []Message) string {
	var b strings.Builder
	for _, message := range messages {
		if message.Role != "tool" {
			continue
		}
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}
		name := strings.TrimSpace(message.Name)
		if name == "" {
			name = "工具"
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(name)
		b.WriteString("：")
		b.WriteString(text)
	}
	return b.String()
}

func wantsCalc(question, expr string) bool {
	if strings.TrimSpace(question) == expr {
		return true
	}
	lower := strings.ToLower(question)
	return strings.Contains(question, "计算") || strings.Contains(question, "算一下") || strings.Contains(lower, "calculate")
}

func extractExpr(text string) string {
	best := ""
	for i := 0; i < len(text); i++ {
		if !isExprByte(text[i]) {
			continue
		}
		j := i
		for j < len(text) && isExprByte(text[j]) {
			j++
		}
		cand := strings.TrimSpace(text[i:j])
		if validExprShape(cand) && len(cand) > len(best) {
			best = cand
		}
		if j == i {
			i++
		} else {
			i = j - 1
		}
	}
	return best
}

func isExprByte(b byte) bool {
	return (b >= '0' && b <= '9') || b == '.' || b == '+' || b == '-' || b == '*' || b == '/' || b == '(' || b == ')' || b == ' '
}

func validExprShape(expr string) bool {
	if len(expr) < 3 || len(expr) > 80 {
		return false
	}
	if dateLike.MatchString(expr) {
		return false
	}
	digit := false
	op := false
	depth := 0
	for _, r := range expr {
		switch r {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			digit = true
		case '+', '-', '*', '/':
			op = true
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		case '.', ' ':
		default:
			return false
		}
	}
	return digit && op && depth == 0
}

func firstJSONObject(text string) string {
	start := strings.IndexByte(text, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(text); i++ {
		c := text[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				cand := text[start : i+1]
				var obj map[string]json.RawMessage
				if json.Unmarshal([]byte(cand), &obj) == nil {
					return cand
				}
				return ""
			}
		}
	}
	return ""
}

func containsAny(text string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}

var (
	invokePattern       = regexp.MustCompile(`(?:调用|使用|use)\s+([A-Za-z_][A-Za-z0-9_]{0,60})`)
	skillMentionPattern = regexp.MustCompile(`(?i)(?:技能\s*[:：]?\s*|skill\s*[:：]\s*|skill\s+)([A-Za-z0-9][A-Za-z0-9_-]{0,63})`)
	skillSuffixPattern  = regexp.MustCompile(`([A-Za-z0-9][A-Za-z0-9_-]*[-_][A-Za-z0-9_-]*)\s*技能`)
	timezonePattern     = regexp.MustCompile(`[A-Za-z]+/[A-Za-z0-9_+\-]+(?:/[A-Za-z0-9_+\-]+)?`)
	dateLike            = regexp.MustCompile(`^\d{4}-\d{1,2}-\d{1,2}$`)
)

func skillMention(question string) string {
	if match := skillMentionPattern.FindStringSubmatch(question); len(match) == 2 {
		return strings.ToLower(match[1])
	}
	if match := skillSuffixPattern.FindStringSubmatch(question); len(match) == 2 {
		return strings.ToLower(match[1])
	}
	return ""
}

func lastUser(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && strings.TrimSpace(messages[i].Content) != "" {
			return messages[i].Content
		}
	}
	return ""
}

func writeChunks(ctx context.Context, ch chan<- Delta, text string, thinking bool, delay time.Duration) bool {
	runes := []rune(text)
	if len(runes) == 0 {
		return true
	}
	var ticker *time.Ticker
	if delay > 0 {
		ticker = time.NewTicker(delay)
		defer ticker.Stop()
	}
	for i := 0; i < len(runes); i += 8 {
		if delay > 0 {
			select {
			case <-ctx.Done():
				return false
			case <-ticker.C:
			}
		} else if ctx.Err() != nil {
			return false
		}
		end := i + 8
		if end > len(runes) {
			end = len(runes)
		}
		delta := Delta{}
		part := string(runes[i:end])
		if thinking {
			delta.Thinking = part
		} else {
			delta.Content = part
		}
		select {
		case <-ctx.Done():
			return false
		case ch <- delta:
		}
	}
	return true
}

func optimizeText(content string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	if strings.Contains(text, "目标：") && strings.Contains(text, "原始需求：") {
		return text
	}
	line := text
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		line = strings.TrimSpace(text[:idx])
	}
	return "请围绕下面的需求给出可以直接执行的结果。\n\n目标：" + line + "\n\n要求：\n- 先给结论，再给步骤\n- 步骤写清输入、动作和完成标准\n- 涉及代码时给出可运行示例和边界情况\n- 用中文，条理清楚，不编造没有给出的事实\n\n原始需求：\n" + text
}
