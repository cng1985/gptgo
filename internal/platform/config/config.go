package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration time.Duration

func (d Duration) Std() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("无效的时长 %q", raw)
	}
	*d = Duration(parsed)
	return nil
}

type Options struct {
	BaseDir   string
	ConfigDir string
	DataDir   string
	Profile   string
}

type RuntimeInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
	AI       AIConfig       `yaml:"ai"`
	Runtime  RuntimeInfo    `yaml:"-"`
}

type AppConfig struct {
	Name      string `yaml:"name"`
	Profile   string `yaml:"profile"`
	BaseDir   string `yaml:"base_dir"`
	ConfigDir string `yaml:"config_dir"`
	DataDir   string `yaml:"data_dir"`
	SeedDemo  bool   `yaml:"seed_demo"`
}

type ServerConfig struct {
	Host              string   `yaml:"host"`
	Port              int      `yaml:"port"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	IdleTimeout       Duration `yaml:"idle_timeout"`
	MaxHeaderBytes    int      `yaml:"max_header_bytes"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
	RequestTimeout    Duration `yaml:"request_timeout"`
	SSETimeout        Duration `yaml:"sse_timeout"`
	MaxBodyBytes      int64    `yaml:"max_body_bytes"`
	TrustedProxies    []string `yaml:"trusted_proxies"`
}

type DatabaseConfig struct {
	Enabled         bool         `yaml:"enabled"`
	Driver          string       `yaml:"driver"`
	DSN             string       `yaml:"dsn"`
	MaxOpenConns    int          `yaml:"max_open_conns"`
	MaxIdleConns    int          `yaml:"max_idle_conns"`
	ConnMaxLifetime Duration     `yaml:"conn_max_lifetime"`
	BatchSize       int          `yaml:"batch_size"`
	SQLite          SQLiteConfig `yaml:"sqlite"`
}

type SQLiteConfig struct {
	Path        string   `yaml:"path"`
	ForeignKeys bool     `yaml:"foreign_keys"`
	BusyTimeout Duration `yaml:"busy_timeout"`
	JournalMode string   `yaml:"journal_mode"`
}

type AuthConfig struct {
	Enabled    bool     `yaml:"enabled"`
	SessionTTL Duration `yaml:"session_ttl"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type AIConfig struct {
	Provider       string   `yaml:"provider"`
	BaseURL        string   `yaml:"base_url"`
	APIKey         string   `yaml:"api_key"`
	DefaultModel   string   `yaml:"default_model"`
	UpstreamModel  string   `yaml:"upstream_model"`
	RequestTimeout Duration `yaml:"request_timeout"`
}

func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func Load(opts Options, info RuntimeInfo) (*Config, error) {
	baseDir := strings.TrimSpace(opts.BaseDir)
	if baseDir == "" {
		baseDir = strings.TrimSpace(os.Getenv("APP_BASE_DIR"))
	}
	if baseDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("无法确定工作目录: %w", err)
		}
		baseDir = wd
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("无法解析 base-dir: %w", err)
	}

	profile := strings.TrimSpace(opts.Profile)
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("APP_PROFILE"))
	}
	if profile != "" && profile != "dev" && profile != "pro" {
		return nil, fmt.Errorf("未知 profile %q，只允许 dev 或 pro", profile)
	}

	configDir := strings.TrimSpace(opts.ConfigDir)
	if configDir == "" {
		configDir = strings.TrimSpace(os.Getenv("APP_CONFIG_DIR"))
	}
	if configDir == "" {
		configDir = "configs"
	}
	if !filepath.IsAbs(configDir) {
		configDir = filepath.Join(absBase, configDir)
	}

	baseFile := filepath.Join(configDir, "application.yaml")
	merged, err := readYAMLMap(baseFile)
	if err != nil {
		return nil, err
	}
	if profile != "" {
		envFile := filepath.Join(configDir, "application-"+profile+".yaml")
		overlay, err := readYAMLMap(envFile)
		if err != nil {
			return nil, err
		}
		merged = deepMerge(merged, overlay)
	}
	if err := validateKnownKeys(merged); err != nil {
		return nil, err
	}
	raw, err := yaml.Marshal(merged)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if err := applyEnv(cfg); err != nil {
		return nil, err
	}

	dataDir := strings.TrimSpace(opts.DataDir)
	if dataDir == "" {
		dataDir = strings.TrimSpace(os.Getenv("APP_DATA_DIR"))
	}
	if dataDir != "" {
		cfg.App.DataDir = dataDir
	}
	if strings.TrimSpace(cfg.App.DataDir) == "" {
		cfg.App.DataDir = "./data"
	}
	if !filepath.IsAbs(cfg.App.DataDir) {
		cfg.App.DataDir = filepath.Join(absBase, cfg.App.DataDir)
	}
	cfg.App.BaseDir = absBase
	cfg.App.ConfigDir = configDir
	if yamlProfile, ok := nestedString(merged, "app", "profile"); ok && yamlProfile != "" && yamlProfile != profile {
		return nil, fmt.Errorf("配置文件中的 app.profile=%s 与启动 profile=%s 不一致", yamlProfile, profile)
	}
	cfg.App.Profile = profile
	cfg.Runtime = info
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.App.Name) == "" {
		return fmt.Errorf("app.name 不能为空")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 超出范围")
	}
	if strings.TrimSpace(c.Server.Host) == "" {
		return fmt.Errorf("server.host 不能为空")
	}
	if c.Server.ReadHeaderTimeout.Std() <= 0 || c.Server.IdleTimeout.Std() <= 0 || c.Server.ShutdownTimeout.Std() <= 0 {
		return fmt.Errorf("服务器超时必须为正数")
	}
	if c.Server.RequestTimeout.Std() <= 0 || c.Server.SSETimeout.Std() <= 0 {
		return fmt.Errorf("请求超时必须为正数")
	}
	if c.Server.MaxHeaderBytes <= 0 || c.Server.MaxBodyBytes <= 0 {
		return fmt.Errorf("请求大小限制必须为正数")
	}
	if !c.Database.Enabled {
		return fmt.Errorf("当前应用依赖数据库保存对话，database.enabled 不能为 false")
	}
	switch c.Database.Driver {
	case "sqlite", "mysql", "postgres", "sqlserver":
	default:
		return fmt.Errorf("未知 database.driver %q", c.Database.Driver)
	}
	if c.Database.Driver != "sqlite" && strings.TrimSpace(c.Database.DSN) == "" {
		return fmt.Errorf("网络数据库必须提供 database.dsn")
	}
	if c.Database.Driver == "sqlite" && strings.TrimSpace(c.Database.SQLite.Path) == "" {
		return fmt.Errorf("database.sqlite.path 不能为空")
	}
	if c.Database.MaxOpenConns < 1 || c.Database.MaxIdleConns < 1 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("数据库连接池参数无效")
	}
	if c.Database.ConnMaxLifetime.Std() <= 0 || c.Database.BatchSize < 1 {
		return fmt.Errorf("数据库连接生命周期或 batch_size 无效")
	}
	if c.Database.SQLite.BusyTimeout.Std() <= 0 {
		return fmt.Errorf("database.sqlite.busy_timeout 必须为正数")
	}
	switch strings.ToUpper(c.Database.SQLite.JournalMode) {
	case "DELETE", "WAL", "TRUNCATE", "PERSIST", "MEMORY", "OFF":
	default:
		return fmt.Errorf("不支持的 journal_mode %q", c.Database.SQLite.JournalMode)
	}
	if c.Auth.Enabled {
		return fmt.Errorf("当前版本是本地单用户，auth.enabled 必须为 false；会话认证尚未实现")
	}
	if c.Auth.SessionTTL.Std() <= 0 {
		return fmt.Errorf("auth.session_ttl 必须为正数")
	}
	switch strings.ToLower(c.Logging.Level) {
	case "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("未知 logging.level %q", c.Logging.Level)
	}
	switch strings.ToLower(c.Logging.Format) {
	case "json", "text":
	default:
		return fmt.Errorf("未知 logging.format %q", c.Logging.Format)
	}
	switch c.AI.Provider {
	case "auto", "builtin", "openai":
	default:
		return fmt.Errorf("未知 ai.provider %q", c.AI.Provider)
	}
	if c.AI.Provider == "openai" && strings.TrimSpace(c.AI.APIKey) == "" {
		return fmt.Errorf("ai.provider=openai 时必须提供 api_key")
	}
	if strings.TrimSpace(c.AI.BaseURL) == "" {
		return fmt.Errorf("ai.base_url 不能为空")
	}
	if strings.TrimSpace(c.AI.DefaultModel) == "" {
		return fmt.Errorf("ai.default_model 不能为空")
	}
	if c.AI.RequestTimeout.Std() <= 0 {
		return fmt.Errorf("ai.request_timeout 必须为正数")
	}
	return nil
}

func (c *Config) SQLiteFile() string {
	path := c.Database.SQLite.Path
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.App.DataDir, path)
}

func readYAMLMap(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s 失败: %w", path, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("解析配置 %s 失败: %w", path, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func deepMerge(base, overlay map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		existing, ok := out[k]
		child, childOK := v.(map[string]any)
		current, currentOK := existing.(map[string]any)
		if ok && childOK && currentOK {
			out[k] = deepMerge(current, child)
			continue
		}
		out[k] = v
	}
	return out
}

func nestedString(doc map[string]any, group, key string) (string, bool) {
	raw, ok := doc[group]
	if !ok {
		return "", false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return "", false
	}
	value, ok := m[key]
	if !ok || value == nil {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return text, true
}

func applyEnv(cfg *Config) error {
	bindings := []struct {
		env   string
		apply func(string) error
	}{
		{"APP_NAME", func(v string) error { cfg.App.Name = v; return nil }},
		{"APP_DATA_DIR", func(v string) error { cfg.App.DataDir = v; return nil }},
		{"APP_SEED_DEMO", func(v string) error {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("APP_SEED_DEMO: %w", err)
			}
			cfg.App.SeedDemo = b
			return nil
		}},
		{"APP_SERVER_HOST", func(v string) error { cfg.Server.Host = v; return nil }},
		{"APP_SERVER_PORT", func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("APP_SERVER_PORT: %w", err)
			}
			cfg.Server.Port = n
			return nil
		}},
		{"APP_SERVER_READ_HEADER_TIMEOUT", durationEnv(func(d Duration) { cfg.Server.ReadHeaderTimeout = d })},
		{"APP_SERVER_IDLE_TIMEOUT", durationEnv(func(d Duration) { cfg.Server.IdleTimeout = d })},
		{"APP_SERVER_MAX_HEADER_BYTES", func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("APP_SERVER_MAX_HEADER_BYTES: %w", err)
			}
			cfg.Server.MaxHeaderBytes = n
			return nil
		}},
		{"APP_SERVER_SHUTDOWN_TIMEOUT", durationEnv(func(d Duration) { cfg.Server.ShutdownTimeout = d })},
		{"APP_SERVER_REQUEST_TIMEOUT", durationEnv(func(d Duration) { cfg.Server.RequestTimeout = d })},
		{"APP_SERVER_SSE_TIMEOUT", durationEnv(func(d Duration) { cfg.Server.SSETimeout = d })},
		{"APP_SERVER_MAX_BODY_BYTES", func(v string) error {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("APP_SERVER_MAX_BODY_BYTES: %w", err)
			}
			cfg.Server.MaxBodyBytes = n
			return nil
		}},
		{"APP_SERVER_TRUSTED_PROXIES", func(v string) error {
			var items []string
			if err := json.Unmarshal([]byte(v), &items); err != nil {
				return fmt.Errorf("APP_SERVER_TRUSTED_PROXIES 必须是 JSON 数组: %w", err)
			}
			cfg.Server.TrustedProxies = items
			return nil
		}},
		{"APP_DATABASE_ENABLED", func(v string) error {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("APP_DATABASE_ENABLED: %w", err)
			}
			cfg.Database.Enabled = b
			return nil
		}},
		{"APP_DATABASE_DRIVER", func(v string) error { cfg.Database.Driver = v; return nil }},
		{"APP_DATABASE_SQLITE_PATH", func(v string) error { cfg.Database.SQLite.Path = v; return nil }},
		{"APP_DATABASE_SQLITE_FOREIGN_KEYS", func(v string) error {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("APP_DATABASE_SQLITE_FOREIGN_KEYS: %w", err)
			}
			cfg.Database.SQLite.ForeignKeys = b
			return nil
		}},
		{"APP_DATABASE_SQLITE_BUSY_TIMEOUT", durationEnv(func(d Duration) { cfg.Database.SQLite.BusyTimeout = d })},
		{"APP_DATABASE_SQLITE_JOURNAL_MODE", func(v string) error { cfg.Database.SQLite.JournalMode = v; return nil }},
		{"APP_DATABASE_DSN", func(v string) error {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("APP_DATABASE_DSN 已设置但为空")
			}
			cfg.Database.DSN = v
			return nil
		}},
		{"APP_DATABASE_MAX_OPEN_CONNS", intEnv(func(n int) { cfg.Database.MaxOpenConns = n })},
		{"APP_DATABASE_MAX_IDLE_CONNS", intEnv(func(n int) { cfg.Database.MaxIdleConns = n })},
		{"APP_DATABASE_CONN_MAX_LIFETIME", durationEnv(func(d Duration) { cfg.Database.ConnMaxLifetime = d })},
		{"APP_DATABASE_BATCH_SIZE", intEnv(func(n int) { cfg.Database.BatchSize = n })},
		{"APP_AUTH_ENABLED", func(v string) error {
			b, err := parseBool(v)
			if err != nil {
				return fmt.Errorf("APP_AUTH_ENABLED: %w", err)
			}
			cfg.Auth.Enabled = b
			return nil
		}},
		{"APP_AUTH_SESSION_TTL", durationEnv(func(d Duration) { cfg.Auth.SessionTTL = d })},
		{"APP_LOGGING_LEVEL", func(v string) error { cfg.Logging.Level = v; return nil }},
		{"APP_LOGGING_FORMAT", func(v string) error { cfg.Logging.Format = v; return nil }},
		{"APP_AI_PROVIDER", func(v string) error { cfg.AI.Provider = v; return nil }},
		{"APP_AI_BASE_URL", func(v string) error { cfg.AI.BaseURL = v; return nil }},
		{"APP_AI_API_KEY", func(v string) error {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("APP_AI_API_KEY 已设置但为空")
			}
			cfg.AI.APIKey = v
			return nil
		}},
		{"APP_AI_DEFAULT_MODEL", func(v string) error { cfg.AI.DefaultModel = v; return nil }},
		{"APP_AI_UPSTREAM_MODEL", func(v string) error { cfg.AI.UpstreamModel = v; return nil }},
		{"APP_AI_REQUEST_TIMEOUT", durationEnv(func(d Duration) { cfg.AI.RequestTimeout = d })},
	}
	for _, binding := range bindings {
		value, ok := os.LookupEnv(binding.env)
		if !ok {
			continue
		}
		if err := binding.apply(value); err != nil {
			return err
		}
	}
	return nil
}

func durationEnv(set func(Duration)) func(string) error {
	return func(v string) error {
		parsed, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		set(Duration(parsed))
		return nil
	}
}

func intEnv(set func(int)) func(string) error {
	return func(v string) error {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return err
		}
		set(n)
		return nil
	}
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("只接受 true 或 false，收到 %q", v)
	}
}
