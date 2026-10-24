package config

import "fmt"

// allowed 是配置键字典。未列出的键会在启动时失败，避免静默忽略拼写错误。
var allowed = map[string]map[string]struct{}{
	"app": {
		"name": {}, "profile": {}, "base_dir": {}, "config_dir": {}, "data_dir": {}, "seed_demo": {},
	},
	"server": {
		"host": {}, "port": {}, "read_header_timeout": {}, "idle_timeout": {}, "max_header_bytes": {},
		"shutdown_timeout": {}, "request_timeout": {}, "sse_timeout": {}, "max_body_bytes": {}, "trusted_proxies": {},
	},
	"database": {
		"enabled": {}, "driver": {}, "dsn": {}, "max_open_conns": {}, "max_idle_conns": {},
		"conn_max_lifetime": {}, "batch_size": {}, "sqlite": {},
	},
	"auth":    {"enabled": {}, "session_ttl": {}},
	"logging": {"level": {}, "format": {}},
	"ai": {
		"provider": {}, "base_url": {}, "api_key": {}, "default_model": {}, "upstream_model": {}, "request_timeout": {},
	},
	"search": {
		"provider": {}, "api_key": {}, "timeout": {},
	},
}

var allowedSQLite = map[string]struct{}{
	"path": {}, "foreign_keys": {}, "busy_timeout": {}, "journal_mode": {},
}

func validateKnownKeys(doc map[string]any) error {
	for key, value := range doc {
		fields, ok := allowed[key]
		if !ok {
			return fmt.Errorf("未知配置段 %q", key)
		}
		child, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("配置段 %q 必须是对象", key)
		}
		for field, raw := range child {
			if _, ok := fields[field]; !ok {
				return fmt.Errorf("未知配置键 %s.%s", key, field)
			}
			if key == "database" && field == "sqlite" {
				sqlite, ok := raw.(map[string]any)
				if !ok {
					return fmt.Errorf("database.sqlite 必须是对象")
				}
				for sk := range sqlite {
					if _, ok := allowedSQLite[sk]; !ok {
						return fmt.Errorf("未知配置键 database.sqlite.%s", sk)
					}
				}
			}
		}
	}
	return nil
}
