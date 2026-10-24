package chat

import (
	"go.uber.org/fx"

	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/config"
	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"chat",
	fx.Provide(NewRepository),
	fx.Provide(func(repo *Repository, search *aiplat.Searcher, models *aiplat.Router, cfg *config.Config) *Resource {
		return NewResource(repo, search, models, cfg.AI.DefaultModel)
	}),
	fx.Provide(func(resource *Resource, models *aiplat.Router) *Controller {
		return NewController(resource, models)
	}),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
