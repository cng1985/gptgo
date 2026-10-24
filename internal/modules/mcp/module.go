package mcp

import (
	"go.uber.org/fx"

	"gorm.io/gorm"
	"gptgo/internal/modules/chatbi"
	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"mcp",
	fx.Provide(func(db *gorm.DB, cipher *chatbi.Cipher) *Repository {
		return NewRepository(db, cipher)
	}),
	fx.Provide(NewResource),
	fx.Provide(NewController),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
