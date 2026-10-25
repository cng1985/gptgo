package chat

import (
	"go.uber.org/fx"

	"gptgo/internal/modules/skill"
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
	fx.Provide(func(resource *Resource, models *aiplat.Router, tools *toolkit.Resource, skills *skill.Resource) *Controller {
		return NewController(resource, models, mergeTools(tools, skills))
	}),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
