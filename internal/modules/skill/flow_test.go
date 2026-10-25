package skill

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gptgo/internal/platform/config"
	"gptgo/internal/platform/database"
	"gptgo/internal/platform/domain"
)

const sampleSkill = "---\nname: pdf-parser\ndescription: 解析 PDF 里的文字\nversion: \"1.2.0\"\nmetadata:\n  display-name: PDF 解析\n---\n\n按页提取文字。\n"

func TestSkillPackageAndAccess(t *testing.T) {
	resource := newResource(t, config.SkillHubConfig{})
	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 7})
	other := domain.WithScope(context.Background(), domain.Scope{ActorID: 8})

	created, err := resource.Upload(ctx, "SKILL.md", strings.NewReader(sampleSkill))
	if err != nil {
		t.Fatal(err)
	}
	if created.Data.Name != "pdf-parser" || created.Data.DisplayName != "PDF 解析" || created.Data.Source != "upload" || !created.Data.Enabled {
		t.Fatalf("上传结果: %#v", created.Data)
	}
	if !strings.Contains(created.Data.Body, "按页提取文字") {
		t.Fatalf("正文丢失: %s", created.Data.Body)
	}
	if _, err := resource.One(other, &SkillRequest{ID: created.Data.ID}); err == nil {
		t.Fatal("其他账号不应读到这个技能")
	}

	ok, text, err := resource.Load(ctx, `{"name":"pdf-parser"}`)
	if err != nil || !ok || !strings.Contains(text, "按页提取文字") {
		t.Fatalf("读取技能: %v %v %s", err, ok, text)
	}
	ok, text, err = resource.Load(ctx, `{"name":"pdf-parser","path":"../secret"}`)
	if err != nil || ok || !strings.Contains(text, "路径") {
		t.Fatalf("路径穿越应被拒绝: %v %v %s", err, ok, text)
	}

	updated, err := resource.Update(ctx, &SkillRequest{ID: created.Data.ID, Enabled: false})
	if err != nil || updated.Data.Enabled {
		t.Fatal(err, updated.Data.Enabled)
	}
	entries, err := resource.Enabled(ctx)
	if err != nil || len(entries) != 0 {
		t.Fatalf("停用后仍出现在目录: %v %#v", err, entries)
	}
	ok, text, err = resource.Load(ctx, `{"name":"PDF 解析"}`)
	if err != nil || ok || !strings.Contains(text, "停用") {
		t.Fatalf("停用后仍可读取: %v %v %s", err, ok, text)
	}

	nested := zipBytes(t, map[string]string{
		"pdf-parser/SKILL.md":        strings.Replace(sampleSkill, "按页提取文字。", "改成提取表格。", 1),
		"pdf-parser/references/a.md": "补充说明",
		"pdf-parser/__MACOSX/skip":   "nope",
	})
	replaced, err := resource.Upload(ctx, "pdf-parser.zip", bytes.NewReader(nested))
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Data.ID != created.Data.ID || !strings.Contains(replaced.Data.Body, "提取表格") {
		t.Fatalf("同名技能应覆盖: %#v", replaced.Data)
	}
	enabled, err := resource.Update(ctx, &SkillRequest{ID: created.Data.ID, Enabled: true})
	if err != nil || !enabled.Data.Enabled {
		t.Fatal(err)
	}
	okLoad, body, err := resource.Load(ctx, `{"name":"pdf-parser","path":"references/a.md"}`)
	if err != nil || !okLoad || !strings.Contains(body, "补充说明") {
		t.Fatalf("读取包内文件: %v %v %s", err, okLoad, body)
	}

	bad := zipBytes(t, map[string]string{"../SKILL.md": sampleSkill})
	if _, err := resource.Upload(ctx, "bad.zip", bytes.NewReader(bad)); err == nil {
		t.Fatal("穿越路径应该失败")
	}
	binary := zipBytes(t, map[string]string{"SKILL.md": sampleSkill, "run.exe": "MZ"})
	if _, err := resource.Upload(ctx, "bin.zip", bytes.NewReader(binary)); err == nil {
		t.Fatal("可执行文件应该失败")
	}

	description, notes := ToolText(entries)
	if description != "" || notes != "" {
		t.Fatal("没有启用技能时不应生成提示")
	}
	description, notes = ToolText([]Entry{{Name: "pdf-parser", DisplayName: "PDF 解析", Description: "解析 PDF"}})
	if !strings.Contains(description, "pdf-parser") || !strings.Contains(notes, "load_skill") {
		t.Fatalf("提示: %s %s", description, notes)
	}
}

func TestHubSearchAndInstall(t *testing.T) {
	pack := zipBytes(t, map[string]string{"SKILL.md": sampleSkill})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/search":
			if r.URL.Query().Get("q") != "pdf" {
				http.NotFound(w, r)
				return
			}
			_, _ = io.WriteString(w, `{"results":[{"slug":"global--pdf-parser","name":"PDF Parser","description":"解析 PDF","version":"1.2.0"}],"total":1}`)
		case "/api/v1/skills/global/pdf-parser/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(pack)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resource := newResource(t, config.SkillHubConfig{BaseURL: server.URL, Token: "secret", Timeout: config.Duration(5 * time.Second)})
	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 7})
	found, err := resource.SearchHub(ctx, &HubSearch{Keyword: "pdf", Page: 1, PageSize: 10})
	if err != nil || found.Total != 1 || found.Data[0].Namespace != "global" || found.Data[0].Slug != "pdf-parser" {
		t.Fatalf("搜索: %v %#v", err, found)
	}
	installed, err := resource.InstallHub(ctx, &HubInstall{Namespace: found.Data[0].Namespace, Slug: found.Data[0].Slug, Version: found.Data[0].Version})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Data.Source != "skillhub" || installed.Data.Name != "pdf-parser" || installed.Data.Registry == "" {
		t.Fatalf("安装: %#v", installed.Data)
	}
	ok, text, err := resource.Load(ctx, `{"name":"pdf-parser"}`)
	if err != nil || !ok || !strings.Contains(text, "按页提取文字") {
		t.Fatalf("安装后读取: %v %v %s", err, ok, text)
	}
}

func TestHubSearchFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/skills" && r.URL.Query().Get("page") == "0" {
			_, _ = io.WriteString(w, `{"code":0,"data":{"items":[{"namespace":"team","slug":"weekly-report","displayName":"周报","summary":"写周报","latestVersion":"2.0.0"}],"total":3}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	resource := newResource(t, config.SkillHubConfig{BaseURL: server.URL, Timeout: config.Duration(5 * time.Second)})
	ctx := domain.WithScope(context.Background(), domain.Scope{ActorID: 3})
	found, err := resource.SearchHub(ctx, &HubSearch{Keyword: "周报", Page: 1, PageSize: 10})
	if err != nil || len(found.Data) != 1 || found.Data[0].Namespace != "team" || found.Data[0].Version != "2.0.0" || found.Total != 3 {
		t.Fatalf("回退搜索: %v %#v", err, found)
	}
}

func TestHubRejectsPrivateAddress(t *testing.T) {
	if !blockedSkillIP(net.ParseIP("10.1.2.3"), true) || !blockedSkillIP(net.ParseIP("127.0.0.1"), false) {
		t.Fatal("私网和回环地址应被拒绝")
	}
	if blockedSkillIP(net.ParseIP("127.0.0.1"), true) {
		t.Fatal("本机测试注册中心需要允许回环")
	}
	hub, err := NewHub(config.SkillHubConfig{BaseURL: "http://10.1.1.1", Timeout: config.Duration(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := hub.Search(context.Background(), "pdf", 1, 5); err == nil {
		t.Fatal("私网注册中心应该失败")
	}
}

func TestParseSearchShapes(t *testing.T) {
	items, total, err := parseSearch([]byte(`{"skills":[{"slug":"python-expert","name":"Python Expert","description":"帮助写 Python","version":"2.0.0"}]}`))
	if err != nil || total != 1 || items[0].Slug != "python-expert" || items[0].Namespace != "global" {
		t.Fatalf("skills: %v %d %#v", err, total, items)
	}
	items, _, err = parseSearch([]byte(`{"results":[{"slug":"special--pdf","displayName":"pdf","summary":"解析 PDF","version":"20260716.073212"}]}`))
	if err != nil || items[0].Namespace != "special" || items[0].Slug != "pdf" || items[0].Name != "pdf" || items[0].Version != "20260716.073212" {
		t.Fatalf("注册中心结果: %v %#v", err, items)
	}
	items, _, err = parseSearch([]byte(`{"code":404,"msg":"没有权限"}`))
	if err == nil {
		t.Fatal("错误码应该失败")
	}
}

func newResource(t *testing.T, hub config.SkillHubConfig) *Resource {
	t.Helper()
	dir := t.TempDir()
	resource, err := NewResource(newRepo(t, dir), &config.Config{
		App:      config.AppConfig{DataDir: dir},
		SkillHub: hub,
	})
	if err != nil {
		t.Fatal(err)
	}
	return resource
}

func newRepo(t *testing.T, dir string) *Repository {
	t.Helper()
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
	return NewRepository(db)
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, body := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
