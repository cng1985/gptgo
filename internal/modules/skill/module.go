package skill

import (
	"go.uber.org/fx"

	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"skill",
	fx.Provide(NewRepository),
	fx.Provide(NewResource),
	fx.Provide(NewController),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
