package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

func newHTTPSession(endpoint string, headers map[string]string) *Session {
	var sessionID string
	var mu sync.Mutex
	client := &http.Client{
		Timeout: callTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("重定向过多")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("不允许的重定向")
			}
			return nil
		},
	}
	return &Session{exchange: func(ctx context.Context, payload []byte, _ bool) (rpcMessage, error) {
		ctx, cancel := context.WithTimeout(ctx, callTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return rpcMessage{}, fmt.Errorf("MCP 地址无效")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
		mu.Lock()
		current := sessionID
		mu.Unlock()
		if current != "" {
			req.Header.Set("Mcp-Session-Id", current)
		}
		for key, value := range headers {
			if blockedHeader(key) {
				continue
			}
			req.Header.Set(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			return rpcMessage{}, fmt.Errorf("无法连接 MCP 服务")
		}
		defer resp.Body.Close()
		if id := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); id != "" && len(id) <= 200 {
			mu.Lock()
			sessionID = id
			mu.Unlock()
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, frameLimit))
		if err != nil {
			return rpcMessage{}, fmt.Errorf("读取 MCP 响应失败")
		}
		if resp.StatusCode == http.StatusAccepted && len(bytes.TrimSpace(body)) == 0 {
			return rpcMessage{}, nil
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return rpcMessage{}, fmt.Errorf("MCP 服务返回 %d", resp.StatusCode)
		}
		return parseRPC(resp.Header.Get("Content-Type"), body)
	}}
}

func blockedHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "host", "content-length", "transfer-encoding", "connection", "content-type", "accept", "mcp-session-id", "mcp-protocol-version":
		return true
	default:
		return false
	}
}

func validateEndpoint(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 500 {
		return fmt.Errorf("地址长度需要在 1 到 500 个字符之间")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("MCP 地址无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("只支持 http 和 https")
	}
	if parsed.User != nil {
		return fmt.Errorf("请把认证信息放在请求头里")
	}
	return nil
}
