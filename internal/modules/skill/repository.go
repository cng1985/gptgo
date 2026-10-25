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

func (r *Repository) Search(ctx context.Context, search *SkillSearch) (response.PageResponse[SkillSimple], error) {
	var result response.PageResponse[SkillSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Skill{}).Where("account_id = ?", owner)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			pattern := likePattern(keyword)
			query = query.Where(
				"name LIKE ? ESCAPE '\\' OR display_name LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\'",
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
	err = base().Order("updated_at desc, id desc").Offset((search.Page - 1) * search.PageSize).Limit(search.PageSize).Find(&rows).Error
	if err != nil {
		return result, mapDB(err)
	}
	data := make([]SkillSimple, 0, len(rows))
	for i := range rows {
		data = append(data, simpleOf(&rows[i]))
	}
	return response.PageResponse[SkillSimple]{Data: data, Total: total, Page: search.Page, PageSize: search.PageSize}, nil
}

func (r *Repository) Count(ctx context.Context, owner int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Skill{}).Where("account_id = ?", owner).Count(&count).Error
	return count, mapDB(err)
}

func (r *Repository) Create(ctx context.Context, entity *Skill) error {
	return mapDB(r.db.WithContext(ctx).Create(entity).Error)
}

func (r *Repository) UpdateMeta(ctx context.Context, entity *Skill) error {
	owner, err := ownerID(ctx)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Model(&Skill{}).Where("id = ? AND account_id = ?", entity.ID, owner).Updates(map[string]any{
		"name": entity.Name, "display_name": entity.DisplayName, "description": entity.Description,
		"version": entity.Version, "source": entity.Source, "namespace": entity.Namespace,
		"slug": entity.Slug, "registry": entity.Registry, "updated_at": entity.UpdatedAt,
	})
	if res.Error != nil {
		return mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound("技能不存在")
	}
	return nil
}

func (r *Repository) SetEnabled(ctx context.Context, id int64, enabled bool) (*Skill, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	res := r.db.WithContext(ctx).Model(&Skill{}).Where("id = ? AND account_id = ?", id, owner).Updates(map[string]any{
		"enabled": enabled, "updated_at": time.Now().UTC(),
	})
	if res.Error != nil {
		return nil, mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, apperr.NotFound("技能不存在")
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id int64) (*Skill, error) {
	entity, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	owner, _ := ownerID(ctx)
	if err := r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).Delete(&Skill{}).Error; err != nil {
		return nil, mapDB(err)
	}
	return entity, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Skill, error) {
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

func (r *Repository) FindMatch(ctx context.Context, raw string) (*Skill, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	key := strings.ToLower(raw)
	var rows []Skill
	err = r.db.WithContext(ctx).
		Where("account_id = ? AND (name = ? OR slug = ? OR display_name = ?)", owner, key, key, raw).
		Order("id asc").Find(&rows).Error
	if err != nil {
		return nil, mapDB(err)
	}
	if len(rows) == 0 {
		return nil, apperr.NotFound("技能不存在")
	}
	for i := range rows {
		if rows[i].Name == key || rows[i].Slug == key {
			return &rows[i], nil
		}
	}
	if len(rows) > 1 {
		return nil, apperr.Invalid("请使用技能名称")
	}
	return &rows[0], nil
}

func (r *Repository) Enabled(ctx context.Context) ([]Skill, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	var rows []Skill
	err = r.db.WithContext(ctx).Where("account_id = ? AND enabled = ?", owner, true).Order("name asc").Find(&rows).Error
	return rows, mapDB(err)
}

func simpleOf(entity *Skill) SkillSimple {
	return SkillSimple{
		ID: entity.ID, Name: entity.Name, DisplayName: entity.DisplayName, Description: entity.Description,
		Version: entity.Version, Source: entity.Source, Namespace: entity.Namespace, Slug: entity.Slug,
		Registry: entity.Registry, Enabled: entity.Enabled, UpdatedAt: entity.UpdatedAt,
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
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "unique") || strings.Contains(text, "constraint") {
		return apperr.Conflict("已经有同名技能")
	}
	return apperr.Internal(err)
}
