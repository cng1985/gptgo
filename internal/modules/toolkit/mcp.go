package toolkit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type mcpSpec struct {
	Transport string
	Command   string
	Args      []string
	Env       map[string]string
	URL       string
	Headers   map[string]string
}

type mcpRemoteTool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

type rpcSession interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params any) error
	Close()
}

func listMCP(ctx context.Context, client *http.Client, spec mcpSpec) ([]mcpRemoteTool, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	session, err := openSession(ctx, client, spec)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	if err := handshake(ctx, session); err != nil {
		return nil, err
	}
	var out []mcpRemoteTool
	var cursor string
	for page := 0; page < 5; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := session.Call(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		var result struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("MCP 工具列表无法读取")
		}
		for _, tool := range result.Tools {
			name := strings.TrimSpace(tool.Name)
			if name == "" {
				continue
			}
			out = append(out, mcpRemoteTool{
				Name:        name,
				Description: clipText(strings.TrimSpace(tool.Description), 400),
				InputSchema: objectSchema(tool.InputSchema),
			})
			if len(out) == 16 {
				return out, nil
			}
		}
		cursor = strings.TrimSpace(result.NextCursor)
		if cursor == "" {
			break
		}
	}
	return out, nil
}

func callMCP(ctx context.Context, client *http.Client, spec mcpSpec, name, arguments string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	var args map[string]any
	if err := decodeObject(arguments, &args); err != nil {
		return false, "参数需要是 JSON 对象", nil
	}
	if args == nil {
		args = map[string]any{}
	}
	session, err := openSession(ctx, client, spec)
	if err != nil {
		if ctx.Err() != nil {
			return false, "", ctx.Err()
		}
		return false, "MCP 服务暂时不可用", nil
	}
	defer session.Close()
	if err := handshake(ctx, session); err != nil {
		if ctx.Err() != nil {
			return false, "", ctx.Err()
		}
		return false, "MCP 服务暂时不可用", nil
	}
	raw, err := session.Call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		if ctx.Err() != nil {
			return false, "", ctx.Err()
		}
		return false, clipText(err.Error(), 300), nil
	}
	return mcpText(raw)
}

func handshake(ctx context.Context, session rpcSession) error {
	var last error
	for _, version := range []string{"2024-11-05", "2025-03-26"} {
		_, err := session.Call(ctx, "initialize", map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"clientInfo":      map[string]any{"name": "ChatBI", "version": "1.0.0"},
		})
		if err == nil {
			_ = session.Notify(ctx, "notifications/initialized", map[string]any{})
			return nil
		}
		last = err
		if !strings.Contains(strings.ToLower(err.Error()), "protocol") {
			return err
		}
	}
	if last == nil {
		last = fmt.Errorf("MCP 初始化失败")
	}
	return last
}

func mcpText(raw json.RawMessage) (bool, string, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return true, clipText(string(raw), 4000), nil
	}
	var b strings.Builder
	for _, item := range result.Content {
		if item.Type != "" && item.Type != "text" {
			continue
		}
		if strings.TrimSpace(item.Text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(item.Text)
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		text = clipText(string(raw), 4000)
	}
	text = clipText(text, 4000)
	if result.IsError {
		return false, text, nil
	}
	return true, text, nil
}

func objectSchema(raw json.RawMessage) json.RawMessage {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || !json.Valid(raw) || raw[0] != '{' {
		return json.RawMessage(`{"type":"object","additionalProperties":true}`)
	}
	return raw
}

func openSession(ctx context.Context, client *http.Client, spec mcpSpec) (rpcSession, error) {
	switch spec.Transport {
	case "stdio":
		return openStdio(ctx, spec)
	case "http":
		if _, err := parseToolURL(spec.URL); err != nil {
			return nil, fmt.Errorf("MCP 地址无效")
		}
		if client == nil {
			client = newSafeClient()
		}
		return &httpSession{client: client, spec: spec, version: "2024-11-05"}, nil
	default:
		return nil, fmt.Errorf("不支持的 MCP 传输")
	}
}

type stdioSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	msgs   chan []byte
	fail   error
	mu     sync.Mutex
	next   int
	once   sync.Once
	closed chan struct{}
}

func openStdio(ctx context.Context, spec mcpSpec) (*stdioSession, error) {
	if !validCommand(spec.Command) {
		return nil, fmt.Errorf("MCP 命令无效")
	}
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Env = append(os.Environ(), envList(spec.Env)...)
	var stderr limitedBuffer
	stderr.max = 800
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("无法启动 MCP 命令")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("无法启动 MCP 命令")
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("无法启动 MCP 命令")
	}
	session := &stdioSession{
		cmd:    cmd,
		stdin:  stdin,
		msgs:   make(chan []byte, 8),
		closed: make(chan struct{}),
	}
	go session.read(stdout, &stderr)
	go func() {
		select {
		case <-ctx.Done():
			session.Close()
		case <-session.closed:
		}
	}()
	return session, nil
}

func (s *stdioSession) read(r io.Reader, stderr *limitedBuffer) {
	defer close(s.msgs)
	reader := bufio.NewReaderSize(r, 64*1024)
	for {
		msg, err := readFrame(reader)
		if err != nil {
			if err != io.EOF {
				text := strings.TrimSpace(stderr.String())
				if text != "" {
					s.fail = fmt.Errorf("%s", clipText(text, 160))
				} else {
					s.fail = fmt.Errorf("MCP 连接已断开")
				}
			}
			return
		}
		select {
		case s.msgs <- msg:
		case <-s.closed:
			return
		}
	}
}

func (s *stdioSession) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	s.mu.Lock()
	s.next++
	id := s.next
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	body = append(body, '\n')
	_, err = s.stdin.Write(body)
	s.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("MCP 命令无法写入")
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case msg, ok := <-s.msgs:
			if !ok {
				if s.fail != nil {
					return nil, s.fail
				}
				return nil, fmt.Errorf("MCP 没有返回结果")
			}
			raw, matched, callErr := matchRPC(msg, id)
			if !matched {
				continue
			}
			return raw, callErr
		}
	}
}

func (s *stdioSession) Notify(ctx context.Context, method string, params any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.stdin.Write(append(body, '\n'))
	return err
}

func (s *stdioSession) Close() {
	s.once.Do(func() {
		close(s.closed)
		_ = s.stdin.Close()
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
		if s.cmd != nil {
			_ = s.cmd.Wait()
		}
	})
}

type httpSession struct {
	client  *http.Client
	spec    mcpSpec
	version string
	session string
}

func (s *httpSession) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	raw, err := s.post(ctx, body, true)
	if err != nil {
		return nil, err
	}
	result, _, callErr := matchRPC(raw, 1)
	return result, callErr
}

func (s *httpSession) Notify(ctx context.Context, method string, params any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		return err
	}
	_, err = s.post(ctx, body, false)
	return err
}

func (s *httpSession) Close() {}

func (s *httpSession) post(ctx context.Context, body []byte, expect bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.spec.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("MCP 地址无效")
	}
	for key, value := range s.spec.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", s.version)
	if s.session != "" {
		req.Header.Set("Mcp-Session-Id", s.session)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 MCP 失败")
	}
	defer resp.Body.Close()
	if id := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); id != "" {
		s.session = id
	}
	if resp.StatusCode == http.StatusAccepted && !expect {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		text := strings.TrimSpace(string(raw))
		if text == "" {
			text = resp.Status
		}
		return nil, fmt.Errorf("%s", clipText(text, 180))
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		return readSSE(resp.Body)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("MCP 没有返回结果")
	}
	if !expect && len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	return raw, nil
}

func readSSE(body io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if json.Valid([]byte(data)) {
			return []byte(data), nil
		}
	}
	return nil, fmt.Errorf("MCP 没有返回结果")
}

func matchRPC(msg []byte, id int) (json.RawMessage, bool, error) {
	var head struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(msg, &head); err != nil || len(head.ID) == 0 {
		return nil, false, nil
	}
	if !idEqual(head.ID, id) {
		return nil, false, nil
	}
	if head.Error != nil && strings.TrimSpace(head.Error.Message) != "" {
		return nil, true, fmt.Errorf("%s", clipText(head.Error.Message, 200))
	}
	if len(head.Result) == 0 {
		head.Result = json.RawMessage(`{}`)
	}
	return head.Result, true, nil
}

func idEqual(raw json.RawMessage, id int) bool {
	var number int
	if json.Unmarshal(raw, &number) == nil {
		return number == id
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text == strconv.Itoa(id)
	}
	return false
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(trim), "content-length:") {
			n, err := strconv.Atoi(strings.TrimSpace(trim[len("content-length:"):]))
			if err != nil || n <= 0 || n > 1<<20 {
				return nil, fmt.Errorf("MCP 响应无效")
			}
			for {
				header, err := r.ReadString('\n')
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(header) == "" {
					break
				}
			}
			buf := make([]byte, n)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			return buf, nil
		}
		if strings.HasPrefix(trim, "{") {
			return []byte(trim), nil
		}
	}
}

func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}

func validCommand(command string) bool {
	if command == "" || len(command) > 200 || strings.Contains(command, "..") {
		return false
	}
	for _, r := range command {
		if r <= 32 || strings.ContainsRune(`;&|$`+"`"+`<>"'`, r) {
			return false
		}
	}
	return true
}

type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (w *limitedBuffer) Write(p []byte) (int, error) {
	remain := w.max - w.buf.Len()
	if remain > 0 {
		if len(p) > remain {
			p = p[:remain]
		}
		_, _ = w.buf.Write(p)
	}
	return len(p), nil
}

func (w *limitedBuffer) String() string { return w.buf.String() }
