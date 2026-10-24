package toolkit

import (
	"path/filepath"

	"go.uber.org/fx"

	"gptgo/internal/platform/config"
	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"toolkit",
	fx.Provide(NewRepository),
	fx.Provide(func(cfg *config.Config) (*Cipher, error) {
		return OpenCipher(filepath.Join(cfg.App.DataDir, "toolkit.key"))
	}),
	fx.Provide(NewResource),
	fx.Provide(NewController),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
