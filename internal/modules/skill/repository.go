package skill

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

func (r *Repository) Create(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var result response.Response[SkillResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&Skill{}).Where("account_id = ?", owner).Count(&count).Error; err != nil {
		return result, mapDB(err)
	}
	if count >= 30 {
		return result, apperr.Invalid("最多创建 30 个技能")
	}
	if err := r.ensureName(ctx, owner, request.Name, 0); err != nil {
		return result, err
	}
	now := time.Now().UTC()
	entity := Skill{
		AccountID:    owner,
		Title:        request.Title,
		Name:         request.Name,
		Description:  request.Description,
		Instructions: request.Instructions,
		Runtime:      request.Runtime,
		Script:       request.Script,
		Parameters:   request.Parameters,
		TimeoutSec:   request.TimeoutSec,
		Enabled:      request.Enabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(toResponse(&entity)), nil
}

func (r *Repository) Update(ctx context.Context, request *SkillRequest) (response.Response[SkillResponse], error) {
	var result response.Response[SkillResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	if _, err := r.Find(ctx, request.ID); err != nil {
		return result, err
	}
	if err := r.ensureName(ctx, owner, request.Name, request.ID); err != nil {
		return result, err
	}
	values := map[string]any{
		"title":        request.Title,
		"name":         request.Name,
		"description":  request.Description,
		"instructions": request.Instructions,
		"runtime":      request.Runtime,
		"script":       request.Script,
		"parameters":   request.Parameters,
		"timeout_sec":  request.TimeoutSec,
		"enabled":      request.Enabled,
		"updated_at":   time.Now().UTC(),
	}
	if err := r.db.WithContext(ctx).Model(&Skill{}).Where("id = ? AND account_id = ?", request.ID, owner).Updates(values).Error; err != nil {
		return result, mapDB(err)
	}
	return r.One(ctx, request.ID)
}

func (r *Repository) One(ctx context.Context, id int64) (response.Response[SkillResponse], error) {
	var result response.Response[SkillResponse]
	entity, err := r.Find(ctx, id)
	if err != nil {
		return result, err
	}
	return response.OK(toResponse(entity)), nil
}

func (r *Repository) Find(ctx context.Context, id int64) (*Skill, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var entity Skill
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("技能不存在")
	}
	if err != nil {
		return nil, mapDB(err)
	}
	return &entity, nil
}

func (r *Repository) FindByName(ctx context.Context, name string) (*Skill, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var entity Skill
	err = r.db.WithContext(ctx).Where("account_id = ? AND name = ?", owner, name).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("技能不存在")
	}
	if err != nil {
		return nil, mapDB(err)
	}
	return &entity, nil
}

func (r *Repository) Search(ctx context.Context, search *SkillSearch) (response.PageResponse[SkillSimple], error) {
	var result response.PageResponse[SkillSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	offset := int64(search.Page-1) * int64(search.PageSize)
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Skill{}).Where("account_id = ?", owner)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			pattern := likePattern(keyword)
			query = query.Where(
				"title LIKE ? ESCAPE '\\' OR name LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\'",
				pattern, pattern, pattern,
			)
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []Skill
	err = base().Order("updated_at desc").Order("id desc").Offset(int(offset)).Limit(search.PageSize).Find(&rows).Error
	if err != nil {
		return result, mapDB(err)
	}
	data := make([]SkillSimple, 0, len(rows))
	for i := range rows {
		data = append(data, toSimple(&rows[i]))
	}
	return response.PageResponse[SkillSimple]{
		Data: data, Total: total, Page: search.Page, PageSize: search.PageSize,
	}, nil
}

func (r *Repository) Enabled(ctx context.Context) ([]Skill, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var rows []Skill
	err = r.db.WithContext(ctx).Where("account_id = ? AND enabled = ?", owner, true).
		Order("updated_at desc").Order("id desc").Limit(12).Find(&rows).Error
	if err != nil {
		return nil, mapDB(err)
	}
	return rows, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) (response.Response[SkillResponse], error) {
	var result response.Response[SkillResponse]
	entity, err := r.Find(ctx, id)
	if err != nil {
		return result, err
	}
	if err := r.db.WithContext(ctx).Delete(entity).Error; err != nil {
		return result, mapDB(err)
	}
	return response.OK(toResponse(entity)), nil
}

func (r *Repository) ensureName(ctx context.Context, owner int64, name string, ignoreID int64) error {
	var count int64
	query := r.db.WithContext(ctx).Model(&Skill{}).Where("account_id = ? AND name = ?", owner, name)
	if ignoreID > 0 {
		query = query.Where("id <> ?", ignoreID)
	}
	if err := query.Count(&count).Error; err != nil {
		return mapDB(err)
	}
	if count > 0 {
		return apperr.Conflict("已经有同名技能")
	}
	return nil
}

func toSimple(entity *Skill) SkillSimple {
	return SkillSimple{
		ID: entity.ID, Title: entity.Title, Name: entity.Name, Description: entity.Description,
		Runtime: entity.Runtime, TimeoutSec: entity.TimeoutSec, Enabled: entity.Enabled, UpdatedAt: entity.UpdatedAt,
	}
}

func toResponse(entity *Skill) SkillResponse {
	return SkillResponse{
		ID: entity.ID, Title: entity.Title, Name: entity.Name, Description: entity.Description,
		Instructions: entity.Instructions, Runtime: entity.Runtime, Script: entity.Script,
		Parameters: entity.Parameters, TimeoutSec: entity.TimeoutSec, Enabled: entity.Enabled,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
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
		return apperr.NotFound("技能不存在")
	}
	return apperr.Internal(err)
}
