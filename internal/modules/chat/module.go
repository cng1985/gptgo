package chat

import (
	"go.uber.org/fx"

	"gptgo/internal/modules/toolkit"
	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/config"
	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"chat",
	fx.Provide(NewRepository),
	fx.Provide(func(repo *Repository, agents AgentSource, models *aiplat.Router, cfg *config.Config) *Resource {
		return NewResource(repo, agents, models, cfg.AI.DefaultModel)
	}),
	fx.Provide(func(resource *Resource, models *aiplat.Router, tools *toolkit.Resource) *Controller {
		return NewController(resource, models, tools)
	}),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
