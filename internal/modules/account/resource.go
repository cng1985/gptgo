package account

import (
	"context"
	"strings"
	"unicode"

	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type ProviderStatus interface {
	Info() (string, bool)
}

type Resource struct {
	repo     *Repository
	provider ProviderStatus
}

func NewResource(repo *Repository, provider ProviderStatus) *Resource {
	return &Resource{repo: repo, provider: provider}
}

func (r *Resource) One(ctx context.Context, request *AccountRequest) (response.Response[AccountResponse], error) {
	var zero response.Response[AccountResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	result, err := r.repo.One(ctx, request)
	if err != nil {
		return zero, err
	}
	r.decorate(&result)
	return result, nil
}

func (r *Resource) Update(ctx context.Context, request *AccountRequest) (response.Response[AccountResponse], error) {
	var zero response.Response[AccountResponse]
	if err := requireActor(ctx); err != nil {
		return zero, err
	}
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	request.Name = strings.TrimSpace(request.Name)
	if !validName(request.Name) {
		return zero, apperr.Invalid("名称需要是 1 到 20 个字符")
	}
	result, err := r.repo.Update(ctx, request)
	if err != nil {
		return zero, err
	}
	r.decorate(&result)
	return result, nil
}

func (r *Resource) decorate(result *response.Response[AccountResponse]) {
	name, ready := r.provider.Info()
	result.Data.Provider = name
	result.Data.ProviderReady = ready
}

func requireActor(ctx context.Context) error {
	if _, ok := domain.ScopeFrom(ctx); !ok {
		return apperr.Unauthorized("未登录")
	}
	return nil
}

func validName(name string) bool {
	runes := []rune(name)
	if len(runes) < 1 || len(runes) > 20 {
		return false
	}
	for _, r := range runes {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
