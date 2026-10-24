package app

import (
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"gptgo/internal/modules/account"
	"gptgo/internal/modules/agent"
	"gptgo/internal/modules/chat"
	"gptgo/internal/modules/chatbi"
	"gptgo/internal/modules/skill"
	"gptgo/internal/modules/toolkit"
	"gptgo/internal/platform/ai"
	"gptgo/internal/platform/config"
	"gptgo/internal/platform/database"
	"gptgo/internal/platform/logging"
	"gptgo/internal/platform/server"
	"gptgo/web"
)

func New(cfg *config.Config) *fx.App {
	return fx.New(
		fx.Supply(cfg),
		fx.Provide(logging.New),
		fx.Provide(database.Open),
		fx.Provide(ai.NewRouter),
		fx.Provide(func(resource *agent.Resource) chat.AgentSource { return resource }),
		fx.Provide(server.NewEngine),
		fx.Provide(server.NewHTTPServer),
		chat.Module,
		account.Module,
		agent.Module,
		chatbi.Module,
		skill.Module,
		toolkit.Module,
		fx.Invoke(Bootstrap),
		fx.Invoke(server.Mount),
		fx.Invoke(database.Hook),
		fx.Invoke(server.Hook),
		web.Module,
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	)
}
