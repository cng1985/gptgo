package agent

import (
	"go.uber.org/fx"

	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"agent",
	fx.Provide(NewRepository),
	fx.Provide(func(repo *Repository, models *aiplat.Router) *Resource {
		return NewResource(repo, models)
	}),
	fx.Provide(NewController),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
