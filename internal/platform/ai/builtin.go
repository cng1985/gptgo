package ai

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
	Kind        string
	ServerID    int64
	RemoteName  string
}

type Request struct {
	Model        string
	Messages     []Message
	ThinkingMode string
	SearchNotes  string
	AgentName    string
	Instructions string
	Tools        []Tool
}

type Message struct {
	Role       string
	Content    string
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
	if calls := builtinToolCalls(req); len(calls) > 0 {
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
	answer = withToolResults(req.Messages, answer)
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

func builtinToolCalls(req Request) []ToolCall {
	if len(req.Tools) == 0 || toolResultFollowsUser(req.Messages) {
		return nil
	}
	question := lastUser(req.Messages)
	if strings.TrimSpace(question) == "" {
		return nil
	}
	calls := make([]ToolCall, 0, 3)
	for _, tool := range req.Tools {
		if len(calls) >= 3 {
			break
		}
		arguments, ok := matchTool(tool, question)
		if !ok {
			continue
		}
		calls = append(calls, ToolCall{
			ID:        "call_" + tool.Name + "_" + itoa(len(calls)+1),
			Name:      tool.Name,
			Arguments: arguments,
		})
	}
	return calls
}

func toolResultFollowsUser(messages []Message) bool {
	last := -1
	for i := range messages {
		if messages[i].Role == "user" {
			last = i
		}
	}
	if last < 0 {
		return false
	}
	for _, message := range messages[last+1:] {
		if message.Role == "tool" {
			return true
		}
	}
	return false
}

func matchTool(tool Tool, question string) (string, bool) {
	switch tool.Name {
	case "clock":
		if containsAny(question, []string{"时间", "几点", "日期", "星期", "现在"}) {
			return `{"timezone":"Asia/Shanghai"}`, true
		}
	case "ask_catalog":
		if containsAny(question, []string{"问数", "数据表", "多少条", "有哪些表", "字段", "条数"}) {
			raw, err := json.Marshal(map[string]string{"question": question})
			if err != nil {
				return "", false
			}
			return string(raw), true
		}
	default:
		if tool.Name != "" && strings.Contains(strings.ToLower(question), strings.ToLower(tool.Name)) {
			return fillToolArgs(tool, question), true
		}
		if tool.RemoteName != "" && strings.Contains(strings.ToLower(question), strings.ToLower(tool.RemoteName)) {
			return fillToolArgs(tool, question), true
		}
	}
	return "", false
}

func fillToolArgs(tool Tool, question string) string {
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if json.Unmarshal(tool.Parameters, &schema) != nil {
		return "{}"
	}
	wanted := map[string]bool{"query": true, "question": true, "text": true, "input": true}
	payload := map[string]string{}
	for _, name := range schema.Required {
		prop, ok := schema.Properties[name]
		if !ok || (prop.Type != "" && prop.Type != "string") || !wanted[name] {
			continue
		}
		payload[name] = question
	}
	if len(payload) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func containsAny(text string, keys []string) bool {
	for _, key := range keys {
		if strings.Contains(text, key) {
			return true
		}
	}
	return false
}

func withToolResults(messages []Message, answer string) string {
	notes := make([]string, 0)
	for _, message := range messages {
		if message.Role != "tool" {
			continue
		}
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}
		notes = append(notes, text)
	}
	if len(notes) == 0 {
		return answer
	}
	return "根据工具结果：\n" + strings.Join(notes, "\n") + "\n\n" + answer
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
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
