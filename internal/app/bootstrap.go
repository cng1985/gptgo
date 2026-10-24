package app

import (
	"log/slog"
	"path/filepath"

	"gorm.io/gorm"

	"gptgo/internal/modules/account"
	"gptgo/internal/modules/chat"
	"gptgo/internal/platform/config"
	"gptgo/internal/platform/database"
)

func Bootstrap(db *gorm.DB, cfg *config.Config, log *slog.Logger) error {
	if err := database.Migrate(db, filepath.Join(cfg.App.BaseDir, "migrations")); err != nil {
		return err
	}
	if err := account.Ensure(db); err != nil {
		return err
	}
	if cfg.App.SeedDemo {
		if err := chat.SeedDemo(db); err != nil {
			return err
		}
	}
	log.Info("application ready",
		"profile", cfg.App.Profile,
		"data_dir", cfg.App.DataDir,
		"sqlite", cfg.SQLiteFile(),
		"ai_provider", cfg.AI.Provider,
		"ai_configured", cfg.AI.APIKey != "",
	)
	return nil
}
