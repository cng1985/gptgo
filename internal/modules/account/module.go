package account

import (
	"go.uber.org/fx"

	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/config"
	"gptgo/internal/platform/middleware"
	"gptgo/internal/platform/routing"
)

var Module = fx.Module(
	"account",
	fx.Provide(NewRepository),
	fx.Provide(func(repo *Repository, models *aiplat.Router, cfg *config.Config) *Resource {
		return NewResource(repo, models, cfg.Auth.Enabled, cfg.Auth.SessionTTL.Std())
	}),
	fx.Provide(func(resource *Resource, cfg *config.Config) *Controller {
		return NewController(resource, cfg.Auth.SessionTTL.Std())
	}),
	fx.Provide(func(repo *Repository, cfg *config.Config) *Gate {
		return NewGate(cfg.Auth.Enabled, repo)
	}),
	fx.Provide(func(gate *Gate) middleware.ActorResolver { return gate }),
	fx.Provide(fx.Annotate(
		func(controller *Controller) routing.Registrar { return controller },
		fx.ResultTags(`group:"registrars"`),
	)),
)
