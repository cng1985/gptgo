package database

import (
	"strings"
	"testing"
	"time"

	"gptgo/internal/platform/config"
)

func TestSQLiteDSNWindowsDrive(t *testing.T) {
	cfg := testConfig(`C:\Users\cng19\proj\data`, "app.db")
	dsn, err := sqliteDSN(cfg)
	if err != nil {
		t.Fatal(err)
	}
	const want = "file:///C:/Users/cng19/proj/data/app.db?"
	if !strings.HasPrefix(dsn, want) {
		t.Fatalf("DSN = %s", dsn)
	}
	if strings.Contains(dsn, `file://C:`) || strings.Contains(dsn, `%5C`) || strings.Contains(dsn, `\`) {
		t.Fatalf("Windows DSN 会被 SQLite 当成 UNC 路径: %s", dsn)
	}
	if !strings.Contains(dsn, "_pragma=foreign_keys(1)") {
		t.Fatalf("缺少 foreign_keys pragma: %s", dsn)
	}
}

func TestSQLiteDSNUnixPath(t *testing.T) {
	cfg := testConfig("/workspace/data", "app.db")
	dsn, err := sqliteDSN(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dsn, "file:///workspace/data/app.db?") {
		t.Fatalf("DSN = %s", dsn)
	}
}

func TestOpenSQLite(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir, "app.db")
	db, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	var fk int
	if err := db.Raw("PRAGMA foreign_keys").Scan(&fk).Error; err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d", fk)
	}
	var mode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(mode, "delete") {
		t.Fatalf("journal_mode = %s", mode)
	}
}

func testConfig(dataDir, path string) *config.Config {
	cfg := &config.Config{}
	cfg.App.DataDir = dataDir
	cfg.Database.Driver = "sqlite"
	cfg.Database.Enabled = true
	cfg.Database.SQLite.Path = path
	cfg.Database.SQLite.ForeignKeys = true
	cfg.Database.SQLite.BusyTimeout = config.Duration(5 * time.Second)
	cfg.Database.SQLite.JournalMode = "DELETE"
	cfg.Database.MaxOpenConns = 1
	cfg.Database.MaxIdleConns = 1
	cfg.Database.ConnMaxLifetime = config.Duration(time.Minute)
	return cfg
}
