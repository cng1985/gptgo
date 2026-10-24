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

func TestSearchProviderValidated(t *testing.T) {
	cfg := validConfig()
	if cfg.Search.Provider != "duckduckgo" {
		t.Fatalf("默认检索来源: %s", cfg.Search.Provider)
	}
	cfg.Search.Provider = "nope"
	if err := cfg.Validate(); err == nil {
		t.Fatal("未知检索来源应该失败")
	}
	cfg.Search.Provider = "bing"
	cfg.Search.APIKey = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("bing 缺少密钥应该失败")
	}
	cfg.Search.APIKey = "secret"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAuthEnabledAccepted(t *testing.T) {
	cfg := validConfig()
	cfg.Auth.Enabled = true
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
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
