package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

func (r *Repository) One(ctx context.Context, _ *AccountRequest) (response.Response[AccountResponse], error) {
	var result response.Response[AccountResponse]
	accountID, err := actorID(ctx)
	if err != nil {
		return result, err
	}
	entity, err := r.Find(ctx, accountID)
	if err != nil {
		return result, err
	}
	result = response.OK(*AccountEntityToResponse(&entity, "", false))
	return result, nil
}

func (r *Repository) Update(ctx context.Context, request *AccountRequest) (response.Response[AccountResponse], error) {
	var result response.Response[AccountResponse]
	accountID, err := actorID(ctx)
	if err != nil {
		return result, err
	}
	entity, err := r.Find(ctx, accountID)
	if err != nil {
		return result, err
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

func (r *Repository) Find(ctx context.Context, id int64) (Account, error) {
	var entity Account
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&entity).Error
	if err != nil {
		return entity, mapDB(err)
	}
	return entity, nil
}

func (r *Repository) FindByUsername(ctx context.Context, username string) (Account, error) {
	var entity Account
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&entity).Error
	if err != nil {
		return entity, mapDB(err)
	}
	return entity, nil
}

func (r *Repository) CreateWithSession(ctx context.Context, entity *Account, ttl time.Duration) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	entity.CreatedAt = now
	entity.UpdatedAt = now
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", now).Delete(&LoginSession{}).Error; err != nil {
			return err
		}
		if err := tx.Create(entity).Error; err != nil {
			return err
		}
		return tx.Create(&LoginSession{
			Token:     token,
			AccountID: entity.ID,
			ExpiresAt: now.Add(ttl),
			CreatedAt: now,
		}).Error
	})
	if err != nil {
		return "", mapDB(err)
	}
	return token, nil
}

func (r *Repository) IssueSession(ctx context.Context, accountID int64, ttl time.Duration) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", now).Delete(&LoginSession{}).Error; err != nil {
			return err
		}
		return tx.Create(&LoginSession{
			Token:     token,
			AccountID: accountID,
			ExpiresAt: now.Add(ttl),
			CreatedAt: now,
		}).Error
	})
	if err != nil {
		return "", mapDB(err)
	}
	return token, nil
}

func (r *Repository) ActiveAccountID(ctx context.Context, token string) (int64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, nil
	}
	var session LoginSession
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		_ = r.db.WithContext(ctx).Where("token = ?", token).Delete(&LoginSession{}).Error
		return 0, nil
	}
	return session.AccountID, nil
}

func (r *Repository) DeleteSession(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return mapDB(r.db.WithContext(ctx).Where("token = ?", token).Delete(&LoginSession{}).Error)
}

func actorID(ctx context.Context) (int64, error) {
	scope, ok := domain.ScopeFrom(ctx)
	if !ok {
		return 0, apperr.Unauthorized("未登录")
	}
	return scope.ActorID, nil
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", apperr.Internal(err)
	}
	return hex.EncodeToString(buf), nil
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
	if isUnique(err) {
		return apperr.Conflict("用户名已被使用")
	}
	return apperr.Internal(err)
}

func isUnique(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique")
}
