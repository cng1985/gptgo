package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func newSafeClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialSafe(ctx, dialer, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("重定向过多")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("地址不可访问")
			}
			return nil
		},
	}
}

func dialSafe(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("地址无法解析")
	}
	var last error
	for _, ip := range ips {
		if blockedIP(ip.IP) {
			last = fmt.Errorf("地址不可访问")
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("地址不可访问")
	}
	return nil, last
}

func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func parseToolURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("地址需要是 http 或 https")
	}
	return parsed, nil
}

func callHTTPTool(ctx context.Context, client *http.Client, method, rawURL string, headers map[string]string, arguments string) (bool, string) {
	if client == nil {
		client = newSafeClient()
	}
	var args map[string]any
	if err := decodeObject(arguments, &args); err != nil {
		return false, "参数需要是 JSON 对象"
	}
	if args == nil {
		args = map[string]any{}
	}
	target, err := parseToolURL(rawURL)
	if err != nil {
		return false, "工具地址无效"
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var req *http.Request
	switch strings.ToUpper(method) {
	case http.MethodGet:
		query := target.Query()
		for key, value := range args {
			switch typed := value.(type) {
			case string:
				query.Set(key, clipText(typed, 500))
			case float64, bool:
				query.Set(key, fmt.Sprint(typed))
			}
		}
		target.RawQuery = query.Encode()
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	case http.MethodPost:
		body, marshalErr := json.Marshal(args)
		if marshalErr != nil {
			return false, "参数无法发送"
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	default:
		return false, "只支持 GET 和 POST"
	}
	if err != nil {
		return false, "工具请求无法发出"
	}
	for key, value := range headers {
		if strings.EqualFold(key, "Host") || strings.EqualFold(key, "Content-Length") {
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return false, "工具请求超时"
		}
		return false, "工具请求失败"
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	text := strings.TrimSpace(string(raw))
	if text == "" {
		text = "（无输出）"
	}
	text = clipText(text, 4000)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Sprintf("接口返回 %d：%s", resp.StatusCode, text)
	}
	return true, text
}

func clipText(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "…"
}
