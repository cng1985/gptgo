package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, request *AgentRequest) (response.Response[AgentResponse], error) {
	var result response.Response[AgentResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&Agent{}).Where("account_id = ?", owner).Count(&count).Error; err != nil {
		return result, mapDB(err)
	}
	if count >= 50 {
		return result, apperr.Invalid("最多创建 50 个智能体")
	}
	now := time.Now().UTC()
	entity := Agent{
		AccountID:    owner,
		Name:         request.Name,
		Description:  request.Description,
		Instructions: request.Instructions,
		Model:        request.Model,
		Opening:      request.Opening,
		Suggestions:  encodeSuggestions(request.Suggestions),
		Tools:        encodeSuggestions(request.Tools),
		McpIDs:       encodeIDs([]int64(request.McpIDs)),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(toResponse(&entity)), nil
}

func (r *Repository) Update(ctx context.Context, request *AgentRequest) (response.Response[AgentResponse], error) {
	var result response.Response[AgentResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&Agent{}).Where("id = ? AND account_id = ?", request.ID, owner).Updates(map[string]any{
		"name":         request.Name,
		"description":  request.Description,
		"instructions": request.Instructions,
		"model":        request.Model,
		"opening":      request.Opening,
		"suggestions":  encodeSuggestions(request.Suggestions),
		"tools":        encodeSuggestions(request.Tools),
		"mcp_ids":      encodeIDs([]int64(request.McpIDs)),
		"updated_at":   now,
	})
	if res.Error != nil {
		return result, mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return result, apperr.NotFound("智能体不存在")
	}
	return r.One(ctx, request)
}

func (r *Repository) One(ctx context.Context, request *AgentRequest) (response.Response[AgentResponse], error) {
	var result response.Response[AgentResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var entity Agent
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", request.ID, owner).First(&entity).Error
	if err != nil {
		return result, mapDB(err)
	}
	return response.OK(toResponse(&entity)), nil
}

func (r *Repository) Search(ctx context.Context, search *AgentSearch) (response.PageResponse[AgentSimple], error) {
	var result response.PageResponse[AgentSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	offset := int64(search.Page-1) * int64(search.PageSize)
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Agent{}).Where("account_id = ?", owner)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			query = query.Where("name LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\'", likePattern(keyword), likePattern(keyword))
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []Agent
	err = base().Order("updated_at desc").Order("id desc").Offset(int(offset)).Limit(search.PageSize).Find(&rows).Error
	if err != nil {
		return result, mapDB(err)
	}
	data := make([]AgentSimple, 0, len(rows))
	for i := range rows {
		data = append(data, toSimple(&rows[i]))
	}
	return response.PageResponse[AgentSimple]{
		Data: data, Total: total, Page: search.Page, PageSize: search.PageSize,
	}, nil
}

func (r *Repository) Delete(ctx context.Context, request *AgentRequest) (response.Response[AgentResponse], error) {
	var result response.Response[AgentResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var entity Agent
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", request.ID, owner).First(&entity).Error
	if err != nil {
		return result, mapDB(err)
	}
	if err := r.db.WithContext(ctx).Delete(&entity).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(toResponse(&entity)), nil
}

func ownerID(ctx context.Context) (int64, error) {
	scope, ok := domain.ScopeFrom(ctx)
	if !ok || scope.ActorID <= 0 {
		return 0, apperr.Unauthorized("未登录")
	}
	return scope.ActorID, nil
}

func likePattern(keyword string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(keyword) + "%"
}

func mapDB(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("智能体不存在")
	}
	return apperr.Internal(err)
}
