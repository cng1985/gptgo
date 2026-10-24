package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPListAndCall(t *testing.T) {
	secret := "secret-token-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept"), "application/json") || !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			t.Errorf("Accept = %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("MCP-Protocol-Version") != protocolVersion {
			t.Errorf("protocol = %s", r.Header.Get("MCP-Protocol-Version"))
		}
		if r.Header.Get("Authorization") != secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			t.Errorf("body: %v", err)
		}
		w.Header().Set("Mcp-Session-Id", "sess-1")
		switch msg.Method {
		case "initialize":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"protocolVersion":"2024-11-05","capabilities":{}}}`)
		case "notifications/initialized":
			if r.Header.Get("Mcp-Session-Id") != "sess-1" {
				t.Errorf("初始化后应带上会话")
			}
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if r.Header.Get("Mcp-Session-Id") != "sess-1" {
				t.Errorf("列工具时应带上会话")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":"+string(msg.ID)+",\"result\":{\"tools\":[{\"name\":\"echo\",\"description\":\"回声\",\"inputSchema\":{\"type\":\"object\",\"properties\":{\"text\":{\"type\":\"string\"}},\"required\":[\"text\"]}}],\"nextCursor\":\"\"}}\n\n")
		case "tools/call":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"content":[{"type":"text","text":"pong"}]}}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	session := newHTTPSession(server.URL, map[string]string{"Authorization": secret})
	tools, err := session.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools: %#v", tools)
	}
	session = newHTTPSession(server.URL, map[string]string{"Authorization": secret})
	if err := session.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	text, ok, err := session.CallTool(context.Background(), "echo", `{"text":"hi"}`)
	if err != nil || !ok || text != "pong" {
		t.Fatalf("call text=%s ok=%v err=%v", text, ok, err)
	}
}

func TestStdioFrames(t *testing.T) {
	serverRead, clientWrite := io.Pipe()
	clientRead, serverWrite := io.Pipe()
	go serveFrames(t, serverRead, serverWrite)
	session := newPipeSession(clientWrite, clientRead)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tools, err := session.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools: %#v", tools)
	}
	if got := advertise(4, "echo tool"); got != "m4_echo_tool" {
		t.Fatalf("advertise = %s", got)
	}
}

func serveFrames(t *testing.T, in io.Reader, out io.Writer) {
	t.Helper()
	reader := bufio.NewReader(in)
	for {
		frame, err := readFrame(reader)
		if err != nil {
			return
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(frame, &msg) != nil {
			return
		}
		if msg.Method == "notifications/initialized" {
			continue
		}
		var payload string
		switch msg.Method {
		case "initialize":
			payload = `{"jsonrpc":"2.0","id":` + string(msg.ID) + `,"result":{"protocolVersion":"2024-11-05"}}`
		case "tools/list":
			payload = `{"jsonrpc":"2.0","id":` + string(msg.ID) + `,"result":{"tools":[{"name":"echo","description":"回声","inputSchema":{"type":"object"}}]}}`
		default:
			return
		}
		if err := writeFrame(out, []byte(payload)); err != nil {
			return
		}
	}
}

func TestClockFallback(t *testing.T) {
	text := ClockText(`{"timezone":"Not/AZone"}`, time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC))
	if !strings.Contains(text, "2026-10-24 20:00:00") || !strings.Contains(text, "CST") {
		t.Fatalf("fallback: %s", text)
	}
}

func TestRejectCommand(t *testing.T) {
	if err := validateCommand("echo;rm", nil); err == nil {
		t.Fatal("带分隔符的命令应该被拒绝")
	}
	if err := validateCommand("../bin/mcp", nil); err == nil {
		t.Fatal("上级目录命令应该被拒绝")
	}
	if err := validateEndpoint("file:///tmp/x"); err == nil {
		t.Fatal("file 地址应该被拒绝")
	}
}
