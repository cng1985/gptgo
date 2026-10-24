package agent

import (
	"go.uber.org/fx"

	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"agent",
	fx.Provide(NewRepository),
	fx.Provide(NewResource),
	fx.Provide(NewController),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
