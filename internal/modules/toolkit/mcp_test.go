package toolkit

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("CHATBI_MCP_HELPER") == "1" {
		mcpHelper()
		return
	}
	os.Exit(m.Run())
}

func mcpHelper() {
	reader := bufio.NewScanner(os.Stdin)
	reader.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(os.Stdout)
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		var msg struct {
			ID     any             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal([]byte(line), &msg) != nil || msg.Method == "" {
			continue
		}
		switch msg.Method {
		case "notifications/initialized":
			continue
		case "initialize":
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]any{"name": "helper", "version": "0"},
				},
			})
		case "tools/list":
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result": map[string]any{"tools": []map[string]any{{
					"name":        "echo",
					"description": "回声",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}},
				}}},
			})
		case "tools/call":
			var params struct {
				Arguments struct {
					Text string `json:"text"`
				} `json:"arguments"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			text := params.Arguments.Text
			if text == "" {
				text = "pong"
			}
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      msg.ID,
				"result":  map[string]any{"content": []map[string]any{{"type": "text", "text": text}}},
			})
		}
	}
}

func TestMCPStdioListAndCall(t *testing.T) {
	spec := mcpSpec{
		Transport: "stdio",
		Command:   os.Args[0],
		Env:       map[string]string{"CHATBI_MCP_HELPER": "1"},
	}
	tools, err := listMCP(context.Background(), nil, spec)
	if err != nil || len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("list: %v %#v", err, tools)
	}
	ok, text, err := callMCP(context.Background(), nil, spec, "echo", `{"text":"你好"}`)
	if err != nil || !ok || text != "你好" {
		t.Fatalf("call: %v %v %s", err, ok, text)
	}
}

func TestMCPHTTPListAndCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Mcp-Session-Id", "s1")
		w.Header().Set("Content-Type", "application/json")
		var msg struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		_ = json.Unmarshal(body, &msg)
		if msg.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		result := map[string]any{}
		switch msg.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "http", "version": "0"}}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{{"name": "ping", "description": "ping", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"content": []map[string]any{{"type": "text", "text": "pong"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": result})
	}))
	defer server.Close()
	spec := mcpSpec{Transport: "http", URL: server.URL}
	tools, err := listMCP(context.Background(), server.Client(), spec)
	if err != nil || len(tools) != 1 || tools[0].Name != "ping" {
		t.Fatalf("list: %v %#v", err, tools)
	}
	ok, text, err := callMCP(context.Background(), server.Client(), spec, "ping", `{}`)
	if err != nil || !ok || text != "pong" {
		t.Fatalf("call: %v %v %s", err, ok, text)
	}
}

func TestReadFrameFormats(t *testing.T) {
	line, err := readFrame(bufio.NewReader(strings.NewReader("{\"id\":1}\n")))
	if err != nil || string(line) != `{"id":1}` {
		t.Fatalf("ndjson: %v %s", err, line)
	}
	framed, err := readFrame(bufio.NewReader(strings.NewReader("Content-Length: 8\r\n\r\n{\"id\":1}")))
	if err != nil || string(framed) != `{"id":1}` {
		t.Fatalf("framed: %v %s", err, framed)
	}
}

func TestBlockedLinkLocal(t *testing.T) {
	ok, text := callHTTPTool(context.Background(), newSafeClient(), http.MethodGet, "http://169.254.169.254/latest", nil, `{}`)
	if ok {
		t.Fatalf("链路本地地址不应访问成功: %s", text)
	}
}
