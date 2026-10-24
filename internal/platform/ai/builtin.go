package ai

import (
	"context"
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
}

type Message struct {
	Role    string
	Content string
}

type Delta struct {
	Thinking string
	Content  string
	Err      error
}

type Builtin struct {
	Delay time.Duration
}

func (b *Builtin) Stream(ctx context.Context, req Request) (<-chan Delta, error) {
	question := lastUser(req.Messages)
	thinking, answer := Compose(question, req.SearchNotes, req.ThinkingMode)
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
