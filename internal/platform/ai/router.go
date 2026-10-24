package ai

import (
	"context"
	"log/slog"
	"time"

	"gptgo/internal/platform/config"
)

type Router struct {
	cfg     *config.Config
	builtin *Builtin
	remote  *Client
	log     *slog.Logger
}

func NewRouter(cfg *config.Config, log *slog.Logger) *Router {
	return &Router{
		cfg:     cfg,
		builtin: &Builtin{Delay: 18 * time.Millisecond},
		remote:  NewClient(cfg),
		log:     log,
	}
}

func (r *Router) Info() (string, bool) {
	if r.useRemote("external") {
		return "openai", true
	}
	return "builtin", false
}

func (r *Router) Stream(ctx context.Context, req Request) (<-chan Delta, error) {
	if r.useRemote(req.Model) {
		return r.remote.Stream(ctx, req)
	}
	return r.builtin.Stream(ctx, req)
}

func (r *Router) Optimize(ctx context.Context, content, model string) (string, error) {
	if r.useRemote(model) {
		optimized, err := r.remote.Optimize(ctx, content, model)
		if err == nil && optimized != "" {
			return optimized, nil
		}
		r.log.Warn("prompt optimize fell back", "err", errString(err))
	}
	return r.builtin.Optimize(ctx, content, model)
}

func (r *Router) useRemote(model string) bool {
	if model == "builtin" || r.cfg.AI.Provider == "builtin" || r.remote == nil || !r.remote.Ready() {
		return false
	}
	return r.cfg.AI.Provider == "openai" || r.cfg.AI.Provider == "auto"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
