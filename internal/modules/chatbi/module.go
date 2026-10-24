package chatbi

import (
	"path/filepath"

	"go.uber.org/fx"

	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/config"
	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"chatbi",
	fx.Provide(NewRepository),
	fx.Provide(NewSQLEngine),
	fx.Provide(func(cfg *config.Config) (*Cipher, error) {
		return OpenCipher(filepath.Join(cfg.App.DataDir, "datasource.key"))
	}),
	fx.Provide(func(repo *Repository, engine *SQLEngine, models *aiplat.Router, cipher *Cipher) *Resource {
		return NewResource(repo, engine, models, cipher)
	}),
	fx.Provide(NewController),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
