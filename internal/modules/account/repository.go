package account

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) One(ctx context.Context, _ *AccountRequest) (response.Response[AccountResponse], error) {
	var result response.Response[AccountResponse]
	var entity Account
	err := r.db.WithContext(ctx).Order("id asc").First(&entity).Error
	if err != nil {
		return result, mapDB(err)
	}
	result = response.OK(*AccountEntityToResponse(&entity, "", false))
	return result, nil
}

func (r *Repository) Update(ctx context.Context, request *AccountRequest) (response.Response[AccountResponse], error) {
	var result response.Response[AccountResponse]
	var entity Account
	err := r.db.WithContext(ctx).Order("id asc").First(&entity).Error
	if err != nil {
		return result, mapDB(err)
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", entity.ID).Updates(map[string]any{
		"name":       request.Name,
		"updated_at": now,
	})
	if res.Error != nil {
		return result, mapDB(res.Error)
	}
	entity.Name = request.Name
	entity.UpdatedAt = now
	result = response.OK(*AccountEntityToResponse(&entity, "", false))
	return result, nil
}

func mapDB(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("账号不存在")
	}
	return apperr.Internal(err)
}
