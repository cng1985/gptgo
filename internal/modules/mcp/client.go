package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	protocolVersion = "2024-11-05"
	frameLimit      = 1 << 20
	callTimeout     = 25 * time.Second
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type RemoteTool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

type exchangeFunc func(ctx context.Context, payload []byte, expectReply bool) (rpcMessage, error)

type Session struct {
	exchange exchangeFunc
	next     int
}

func (s *Session) Initialize(ctx context.Context) error {
	_, err := s.call(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "chatbi", "version": "1.0"},
	})
	if err != nil {
		return err
	}
	return s.notify(ctx, "notifications/initialized", map[string]any{})
}

func (s *Session) ListTools(ctx context.Context) ([]RemoteTool, error) {
	if err := s.Initialize(ctx); err != nil {
		return nil, err
	}
	cursor := ""
	all := make([]RemoteTool, 0)
	for page := 0; page < 10; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		msg, err := s.call(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		var parsed struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(msg.Result, &parsed); err != nil {
			return nil, fmt.Errorf("MCP 工具列表无法解析")
		}
		for _, tool := range parsed.Tools {
			name := strings.TrimSpace(tool.Name)
			if name == "" {
				continue
			}
			schema := tool.InputSchema
			if len(bytes.TrimSpace(schema)) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			all = append(all, RemoteTool{Name: name, Description: tool.Description, InputSchema: schema})
			if len(all) >= 40 {
				return all, nil
			}
		}
		if parsed.NextCursor == "" || parsed.NextCursor == cursor {
			break
		}
		cursor = parsed.NextCursor
	}
	return all, nil
}

func (s *Session) CallTool(ctx context.Context, name, arguments string) (string, bool, error) {
	var args any = map[string]any{}
	if strings.TrimSpace(arguments) != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil || args == nil {
			args = map[string]any{}
		}
	}
	msg, err := s.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", false, err
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(msg.Result, &parsed); err != nil {
		return "", false, fmt.Errorf("MCP 工具结果无法解析")
	}
	parts := make([]string, 0, len(parsed.Content))
	for _, item := range parsed.Content {
		if strings.TrimSpace(item.Text) == "" {
			continue
		}
		parts = append(parts, item.Text)
	}
	text := strings.Join(parts, "\n")
	if text == "" {
		text = strings.TrimSpace(string(msg.Result))
	}
	if parsed.IsError {
		if text == "" {
			text = "MCP 工具返回错误"
		}
		return text, false, nil
	}
	return text, true, nil
}

func (s *Session) call(ctx context.Context, method string, params any) (rpcMessage, error) {
	s.next++
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      s.next,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return rpcMessage{}, err
	}
	msg, err := s.exchange(ctx, body, true)
	if err != nil {
		return rpcMessage{}, err
	}
	if msg.Error != nil {
		text := strings.TrimSpace(msg.Error.Message)
		if text == "" {
			text = "MCP 调用失败"
		}
		return rpcMessage{}, fmt.Errorf("%s", clipText(text, 180))
	}
	return msg, nil
}

func (s *Session) notify(ctx context.Context, method string, params any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	_, err = s.exchange(ctx, body, false)
	return err
}

func parseRPC(contentType string, body []byte) (rpcMessage, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return rpcMessage{}, nil
	}
	if json.Valid(trimmed) && !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		var msg rpcMessage
		if err := json.Unmarshal(trimmed, &msg); err != nil {
			return rpcMessage{}, fmt.Errorf("MCP 响应无法解析")
		}
		return msg, nil
	}
	return parseSSE(trimmed)
}

func parseSSE(body []byte) (rpcMessage, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), frameLimit)
	var last rpcMessage
	found := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			continue
		}
		if msg.Error != nil || len(msg.Result) > 0 {
			last = msg
			found = true
		}
	}
	if !found {
		return rpcMessage{}, fmt.Errorf("MCP 响应无法解析")
	}
	return last, nil
}

func writeFrame(w io.Writer, payload []byte) error {
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			n, err := strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
			if err != nil || n < 0 || n > frameLimit {
				return nil, fmt.Errorf("MCP 帧过大")
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("MCP 帧缺少长度")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func newPipeSession(w io.WriteCloser, r io.Reader) *Session {
	reader := bufio.NewReader(r)
	var mu sync.Mutex
	return &Session{exchange: func(ctx context.Context, payload []byte, expectReply bool) (rpcMessage, error) {
		mu.Lock()
		defer mu.Unlock()
		if err := writeFrame(w, payload); err != nil {
			return rpcMessage{}, fmt.Errorf("无法向 MCP 写入请求")
		}
		if !expectReply {
			return rpcMessage{}, nil
		}
		type frameResult struct {
			msg rpcMessage
			err error
		}
		done := make(chan frameResult, 1)
		go func() {
			for {
				frame, err := readFrame(reader)
				if err != nil {
					done <- frameResult{err: fmt.Errorf("MCP 连接已断开")}
					return
				}
				var msg rpcMessage
				if json.Unmarshal(frame, &msg) != nil {
					continue
				}
				if msg.Method != "" && len(bytes.TrimSpace(msg.ID)) == 0 {
					continue
				}
				done <- frameResult{msg: msg}
				return
			}
		}()
		timer := time.NewTimer(callTimeout)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return rpcMessage{}, ctx.Err()
		case <-timer.C:
			return rpcMessage{}, fmt.Errorf("MCP 响应超时")
		case got := <-done:
			return got.msg, got.err
		}
	}}
}

func clipText(text string, max int) string {
	rs := []rune(text)
	if len(rs) <= max {
		return text
	}
	return string(rs[:max])
}
