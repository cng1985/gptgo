package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gptgo/internal/platform/config"
	"gptgo/internal/platform/search"
)

type Router struct {
	cfg     *config.Config
	builtin *Builtin
	remote  *Client
	web     *search.Client
	log     *slog.Logger
}

func NewRouter(cfg *config.Config, log *slog.Logger) *Router {
	return &Router{
		cfg:     cfg,
		builtin: &Builtin{Delay: 18 * time.Millisecond},
		remote:  NewClient(cfg),
		web:     search.New(cfg),
		log:     log,
	}
}

func (r *Router) Search(ctx context.Context, query string) (search.Result, error) {
	if r.web == nil {
		return search.Result{}, fmt.Errorf("联网检索不可用")
	}
	result, err := r.web.Search(ctx, query)
	if err != nil && r.log != nil {
		r.log.Warn("web search failed", "err", errString(err))
	}
	return result, err
}

func (r *Router) Read(ctx context.Context, rawURL string) (search.Page, error) {
	if r.web == nil {
		return search.Page{}, fmt.Errorf("联网检索不可用")
	}
	page, err := r.web.Read(ctx, rawURL)
	if err != nil && r.log != nil {
		r.log.Warn("web page read failed", "err", errString(err))
	}
	return page, err
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

func (r *Router) Complete(ctx context.Context, system, user, model string) (string, error) {
	if !r.useRemote(model) {
		return "", fmt.Errorf("使用内置问数")
	}
	return r.remote.Complete(ctx, system, user, model)
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

func (r *Router) Author(ctx context.Context, prompt, model string) (string, error) {
	if r.useRemote(model) {
		raw, err := r.remote.Author(ctx, prompt, model)
		if err == nil && validDraft(raw) {
			return raw, nil
		}
		if err == nil {
			err = fmt.Errorf("模型没有返回智能体配置")
		}
		r.log.Warn("agent draft fell back", "err", errString(err))
	}
	return r.builtin.Author(ctx, prompt, model)
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
