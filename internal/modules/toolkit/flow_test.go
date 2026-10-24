package toolkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gptgo/internal/platform/database"
	"gptgo/internal/platform/domain"
)

func TestToolkitFlow(t *testing.T) {
	db := openTestDB(t)
	cipher := openTestCipher(t)
	resource := NewResource(NewRepository(db), cipher)
	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 4, RequestID: "tool"})
	other := domain.WithScope(context.Background(), domain.Scope{ActorID: 5, RequestID: "other"})

	builtin, err := resource.SearchBuiltin(ctx, &ToolSearch{Page: 1, PageSize: 20})
	if err != nil || builtin.Total != 2 || !builtin.Data[0].Enabled {
		t.Fatalf("内置工具: %v %#v", err, builtin)
	}
	if _, err := resource.UpdateBuiltin(ctx, &BuiltinUpdate{Name: "calculator", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	ok, text, err := resource.Invoke(ctx, "calculator", `{"expression":"1+1"}`)
	if err != nil || ok || !strings.Contains(text, "停用") {
		t.Fatalf("停用后仍可调用: %v %v %s", err, ok, text)
	}
	if _, err := resource.UpdateBuiltin(ctx, &BuiltinUpdate{Name: "calculator", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	ok, text, err = resource.Invoke(ctx, "calculator", `{"expression":"1+2*3"}`)
	if err != nil || !ok || text != "7" {
		t.Fatalf("计算: %v %v %s", err, ok, text)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "yes" {
			http.Error(w, "missing", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("q") != "上海" {
			http.Error(w, "query", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("晴"))
	}))
	defer server.Close()

	created, err := resource.CreateHTTP(ctx, &HTTPToolRequest{
		Name: "weather", Description: "查询天气", Method: "GET", URL: server.URL,
		Headers: `{"X-Test":"yes"}`, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Data.HasHeaders != true || created.Data.Parameters == "" {
		t.Fatalf("创建结果不应丢字段: %#v", created.Data)
	}
	one, err := resource.OneHTTP(ctx, &HTTPToolRequest{ID: created.Data.ID})
	if err != nil || strings.Contains(one.Data.Parameters, "X-Test") {
		t.Fatal(err, one)
	}
	if _, err := resource.OneHTTP(other, &HTTPToolRequest{ID: created.Data.ID}); err == nil {
		t.Fatal("其他账号不应读到这个工具")
	}
	ok, text, err = resource.Invoke(ctx, "weather", `{"q":"上海"}`)
	if err != nil || !ok || text != "晴" {
		t.Fatalf("http 工具: %v %v %s", err, ok, text)
	}
	catalog, err := resource.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range catalog {
		if tool.Name == "weather" {
			found = true
		}
	}
	if !found {
		t.Fatalf("目录里没有 HTTP 工具: %#v", catalog)
	}
	if _, err := resource.CreateHTTP(ctx, &HTTPToolRequest{
		Name: "weather", Description: "重复", Method: "GET", URL: server.URL, Enabled: true,
	}); err == nil {
		t.Fatal("同名工具应被拒绝")
	}
	if _, err := resource.CreateMCP(ctx, &MCPRequest{Name: "坏命令", Transport: "stdio", Command: "echo ok", Enabled: true}); err == nil {
		t.Fatal("带空格的命令应被拒绝")
	}

	mcp, err := resource.CreateMCP(ctx, &MCPRequest{
		Name: "回声", Transport: "stdio", Command: os.Args[0], Args: []string{},
		Env: `{"CHATBI_MCP_HELPER":"1"}`, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	remote, err := resource.RemoteTools(ctx, &MCPRequest{ID: mcp.Data.ID})
	if err != nil || len(remote.Data) != 1 || remote.Data[0].Name != "echo" {
		t.Fatalf("远程工具: %v %#v", err, remote)
	}
	catalog, err = resource.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var publicName string
	for _, tool := range catalog {
		if strings.HasPrefix(tool.Name, "m") && strings.Contains(tool.Name, "_echo") {
			publicName = tool.Name
		}
	}
	if publicName == "" {
		t.Fatalf("目录里没有 MCP 工具: %#v", catalog)
	}
	ok, text, err = resource.Invoke(ctx, publicName, `{"text":"在"}`)
	if err != nil || !ok || text != "在" {
		t.Fatalf("mcp 调用: %v %v %s name=%s", err, ok, text, publicName)
	}
	if _, err := resource.DeleteMCP(other, &MCPRequest{ID: mcp.Data.ID}); err == nil {
		t.Fatal("其他账号不应删除 MCP")
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := (&url.URL{Scheme: "file", Path: filepath.Join(dir, "app.db"), RawQuery: "_pragma=foreign_keys(1)"}).String()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func openTestCipher(t *testing.T) *Cipher {
	t.Helper()
	cipher, err := OpenCipher(filepath.Join(t.TempDir(), "toolkit.key"))
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}
