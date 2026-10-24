package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnknownKeyRejected(t *testing.T) {
	err := validateKnownKeys(map[string]any{
		"app": map[string]any{"name": "chatbi", "extra": true},
	})
	if err == nil {
		t.Fatal("未知键应该失败")
	}
}

func TestProfileOverlayReplacesScalar(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "application.yaml"), []byte("app:\n  name: chatbi\n  data_dir: ./data\n  seed_demo: true\nserver:\n  host: 127.0.0.1\n  port: 8080\n  read_header_timeout: 5s\n  idle_timeout: 60s\n  max_header_bytes: 1048576\n  shutdown_timeout: 30s\n  request_timeout: 30s\n  sse_timeout: 15m\n  max_body_bytes: 2097152\n  trusted_proxies: []\ndatabase:\n  enabled: true\n  driver: sqlite\n  max_open_conns: 1\n  max_idle_conns: 1\n  conn_max_lifetime: 30m\n  batch_size: 100\n  sqlite:\n    path: app.db\n    foreign_keys: true\n    busy_timeout: 5s\n    journal_mode: DELETE\nauth:\n  enabled: false\n  session_ttl: 8h\nlogging:\n  level: info\n  format: json\nai:\n  provider: auto\n  base_url: https://api.openai.com/v1\n  api_key: \"\"\n  default_model: minimax-m3.1-flash\n  upstream_model: \"\"\n  request_timeout: 120s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "application-pro.yaml"), []byte("app:\n  seed_demo: false\nlogging:\n  level: warn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(Options{BaseDir: dir, ConfigDir: dir, Profile: "pro"}, RuntimeInfo{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.SeedDemo {
		t.Fatal("profile 中的 false 应该覆盖基础配置")
	}
	if cfg.Logging.Level != "warn" {
		t.Fatalf("level = %s", cfg.Logging.Level)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("未被覆盖的端口丢失: %d", cfg.Server.Port)
	}
}

func TestFindAppRootFromNestedDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", "application.yaml"), []byte("app: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "cmd", "server")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := findAppRoot(nested)
	if !ok {
		t.Fatal("应该从子目录找到应用根")
	}
	want, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("root = %s, want %s", got, want)
	}
	if _, ok := findAppRoot(t.TempDir()); ok {
		t.Fatal("没有配置文件时不应该找到应用根")
	}
}

func TestLoadFindsConfigFromNestedWorkingDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(wd); chdirErr != nil {
			t.Errorf("恢复工作目录失败: %v", chdirErr)
		}
	})
	nested := filepath.Join(repoRoot(t), "cmd", "server")
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(Options{}, RuntimeInfo{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Name != "chatbi" {
		t.Fatalf("name = %s", cfg.App.Name)
	}
	if _, err := os.Stat(filepath.Join(cfg.App.BaseDir, "configs", "application.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.App.BaseDir, "migrations")); err != nil {
		t.Fatal(err)
	}
}

func TestAuthEnabledRejected(t *testing.T) {
	cfg := validConfig()
	cfg.Auth.Enabled = true
	if err := cfg.Validate(); err == nil {
		t.Fatal("未实现的认证不应该静默启用")
	}
}

func TestNetworkDriverConfigAccepted(t *testing.T) {
	cfg := validConfig()
	cfg.Database.Driver = "mysql"
	cfg.Database.DSN = "user:pass@tcp(127.0.0.1:3306)/app"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func validConfig() *Config {
	cfg, err := Load(Options{BaseDir: repoRoot(nil)}, RuntimeInfo{Version: "test"})
	if err != nil {
		panic(err)
	}
	return cfg
}

func repoRoot(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		if t != nil {
			t.Fatal(err)
		}
		panic(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}
