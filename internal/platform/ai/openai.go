package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gptgo/internal/platform/config"
)

type Client struct {
	baseURL  string
	apiKey   string
	upstream string
	client   *http.Client
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL:  strings.TrimRight(cfg.AI.BaseURL, "/"),
		apiKey:   strings.TrimSpace(cfg.AI.APIKey),
		upstream: strings.TrimSpace(cfg.AI.UpstreamModel),
		client:   &http.Client{},
	}
}

func (c *Client) Ready() bool { return c != nil && c.apiKey != "" }

func (c *Client) Stream(ctx context.Context, req Request) (<-chan Delta, error) {
	body, err := c.payload(req, true)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.decorate(httpReq)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("连接模型失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("模型接口返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	ch := make(chan Delta, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		c.readStream(ctx, resp.Body, ch)
	}()
	return ch, nil
}

func (c *Client) Complete(ctx context.Context, system, user, model string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	name := model
	if c.upstream != "" {
		name = c.upstream
	}
	payload := map[string]any{
		"model": name,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.decorate(httpReq)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("模型接口返回 %d", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("模型没有返回内容")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (c *Client) Optimize(ctx context.Context, content, model string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req := Request{Model: model, Messages: []Message{{
		Role:    "user",
		Content: "请把下面的提示词改写成更清晰、可执行的中文提示词，保留原意，不要回答问题本身。\n\n" + content,
	}}}
	body, err := c.payload(req, false)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.decorate(httpReq)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("模型接口返回 %d", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("模型没有返回内容")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (c *Client) payload(req Request, stream bool) ([]byte, error) {
	model := req.Model
	if c.upstream != "" {
		model = c.upstream
	}
	messages := make([]map[string]string, 0, len(req.Messages)+1)
	system := "你是 ChatBI AI。用中文回答，先给结论，再给步骤。代码要能说明问题。不确定时直接说明，不要编造用户没提供的实时数据。"
	if req.ThinkingMode == "light" || req.ThinkingMode == "deep" {
		system += "需要推理时把推理过程放在 reasoning_content。没有该字段时，先输出 <thinking>思考</thinking>，再输出正式回答。"
	}
	messages = append(messages, map[string]string{"role": "system", "content": system})
	for _, message := range req.Messages {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		messages = append(messages, map[string]string{"role": message.Role, "content": message.Content})
	}
	if req.SearchNotes != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": "以下是本次联网检索摘要，只能当参考，不能当成用户原话：\n" + req.SearchNotes,
		})
	}
	payload := map[string]any{"model": model, "messages": messages, "stream": stream}
	return json.Marshal(payload)
}

func (c *Client) decorate(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream, application/json")
}

func (c *Client) readStream(ctx context.Context, body io.Reader, ch chan<- Delta) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var thinkingMode bool
	var pending strings.Builder
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			c.flushTagged(&pending, &thinkingMode, ch)
			return
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					Reasoning string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			select {
			case <-ctx.Done():
			case ch <- Delta{Err: fmt.Errorf("模型返回错误")}:
			}
			return
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Reasoning != "" {
			select {
			case <-ctx.Done():
				return
			case ch <- Delta{Thinking: delta.Reasoning}:
			}
		}
		if delta.Content != "" {
			c.emitContent(delta.Content, &pending, &thinkingMode, ch, ctx)
		}
	}
	c.flushTagged(&pending, &thinkingMode, ch)
}

func (c *Client) emitContent(part string, pending *strings.Builder, thinking *bool, ch chan<- Delta, ctx context.Context) {
	pending.WriteString(part)
	text := pending.String()
	if !*thinking && strings.Contains(text, "<thinking>") {
		before, after, _ := strings.Cut(text, "<thinking>")
		if before != "" {
			send(ctx, ch, Delta{Content: before})
		}
		*thinking = true
		pending.Reset()
		pending.WriteString(after)
		text = pending.String()
	}
	if *thinking {
		if end := strings.Index(text, "</thinking>"); end >= 0 {
			send(ctx, ch, Delta{Thinking: text[:end]})
			*thinking = false
			pending.Reset()
			rest := text[end+len("</thinking>"):]
			if rest != "" {
				send(ctx, ch, Delta{Content: rest})
			}
		}
		return
	}
	pending.Reset()
	if text != "" {
		send(ctx, ch, Delta{Content: text})
	}
}

func (c *Client) flushTagged(pending *strings.Builder, thinking *bool, ch chan<- Delta) {
	text := pending.String()
	pending.Reset()
	if text == "" {
		return
	}
	if *thinking {
		ch <- Delta{Thinking: text}
		return
	}
	ch <- Delta{Content: text}
}

func send(ctx context.Context, ch chan<- Delta, delta Delta) {
	select {
	case <-ctx.Done():
	case ch <- delta:
	}
}
