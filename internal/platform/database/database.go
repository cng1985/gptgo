package database

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/fx"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gptgo/internal/platform/config"
)

func Open(cfg *config.Config) (*gorm.DB, error) {
	if err := os.MkdirAll(cfg.App.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	if cfg.Database.Driver != "sqlite" {
		return nil, fmt.Errorf("当前构建只完成了 SQLite 验证，拒绝把 driver=%s 静默回退到 SQLite", cfg.Database.Driver)
	}
	dsn, err := sqliteDSN(cfg)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime.Std())
	return db, nil
}

func sqliteDSN(cfg *config.Config) (string, error) {
	path := cfg.SQLiteFile()
	fk := 0
	if cfg.Database.SQLite.ForeignKeys {
		fk = 1
	}
	ms := cfg.Database.SQLite.BusyTimeout.Std().Milliseconds()
	if ms < 1 {
		ms = 1
	}
	mode := strings.ToUpper(cfg.Database.SQLite.JournalMode)
	raw := fmt.Sprintf("_pragma=foreign_keys(%d)&_pragma=busy_timeout(%d)&_pragma=journal_mode(%s)", fk, ms, mode)
	u := url.URL{Scheme: "file", Path: path, RawQuery: raw}
	return u.String(), nil
}

func Hook(lc fx.Lifecycle, db *gorm.DB) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
	})
}
