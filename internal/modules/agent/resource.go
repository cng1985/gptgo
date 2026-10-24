package agent

import (
	"context"
	"strings"

	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Resource struct {
	repo *Repository
}

func NewResource(repo *Repository) *Resource {
	return &Resource{repo: repo}
}

func (r *Resource) Create(ctx context.Context, request *AgentRequest) (response.Response[AgentResponse], error) {
	var zero response.Response[AgentResponse]
	if err := prepare(request, false); err != nil {
		return zero, err
	}
	return r.repo.Create(ctx, request)
}

func (r *Resource) Update(ctx context.Context, request *AgentRequest) (response.Response[AgentResponse], error) {
	var zero response.Response[AgentResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.Invalid("智能体不存在")
	}
	if err := prepare(request, true); err != nil {
		return zero, err
	}
	return r.repo.Update(ctx, request)
}

func (r *Resource) One(ctx context.Context, request *AgentRequest) (response.Response[AgentResponse], error) {
	var zero response.Response[AgentResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("智能体不存在")
	}
	return r.repo.One(ctx, request)
}

func (r *Resource) Search(ctx context.Context, search *AgentSearch) (response.PageResponse[AgentSimple], error) {
	var zero response.PageResponse[AgentSimple]
	if search == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 50)
	if err != nil {
		return zero, err
	}
	search.Page, search.PageSize = page, size
	if len([]rune(search.Keyword)) > 100 {
		return zero, apperr.Invalid("搜索关键词过长")
	}
	return r.repo.Search(ctx, search)
}

func (r *Resource) Delete(ctx context.Context, request *AgentRequest) (response.Response[AgentResponse], error) {
	var zero response.Response[AgentResponse]
	if request == nil || request.ID <= 0 {
		return zero, apperr.NotFound("智能体不存在")
	}
	return r.repo.Delete(ctx, request)
}

func (r *Resource) Prompt(ctx context.Context, id int64) (string, string, error) {
	if id <= 0 {
		return "", "", nil
	}
	result, err := r.One(ctx, &AgentRequest{ID: id})
	if err != nil {
		return "", "", err
	}
	return result.Data.Name, result.Data.Instructions, nil
}

func prepare(request *AgentRequest, updating bool) error {
	if request == nil {
		return apperr.Invalid("参数格式错误")
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.Instructions = strings.TrimSpace(request.Instructions)
	request.Opening = strings.TrimSpace(request.Opening)
	request.Model = strings.TrimSpace(request.Model)
	if request.Name == "" || len([]rune(request.Name)) > 40 {
		return apperr.Invalid("名称长度需要在 1 到 40 个字之间")
	}
	if len([]rune(request.Description)) > 200 {
		return apperr.Invalid("简介不能超过 200 个字")
	}
	if request.Instructions == "" || len([]rune(request.Instructions)) > 4000 {
		return apperr.Invalid("设定长度需要在 1 到 4000 个字之间")
	}
	if len([]rune(request.Opening)) > 500 {
		return apperr.Invalid("开场白不能超过 500 个字")
	}
	if request.Model == "" {
		request.Model = "builtin"
	}
	if _, ok := Models[request.Model]; !ok {
		return apperr.Invalid("不支持的模型")
	}
	cleaned := make([]string, 0, 4)
	for _, item := range request.Suggestions {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if len([]rune(item)) > 80 {
			return apperr.Invalid("每条建议不能超过 80 个字")
		}
		cleaned = append(cleaned, item)
		if len(cleaned) == 4 {
			break
		}
	}
	request.Suggestions = cleaned
	if updating && request.ID <= 0 {
		return apperr.Invalid("智能体不存在")
	}
	return nil
}

func normalizePage(page, size, fallback int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = fallback
	}
	if page < 1 || size < 1 || size > 200 {
		return 0, 0, apperr.Invalid("分页参数无效")
	}
	if int64(page-1) > 100000/int64(size) {
		return 0, 0, apperr.Invalid("分页范围超出限制")
	}
	return page, size, nil
}
