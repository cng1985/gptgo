package chatbi

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

func (r *Repository) CreateSource(ctx context.Context, entity *Datasource) error {
	owner, err := ownerID(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	entity.AccountID = owner
	entity.CreatedAt = now
	entity.UpdatedAt = now
	err = r.db.WithContext(ctx).
		Select("AccountID", "Name", "Driver", "Host", "Port", "DatabaseName", "Username", "PasswordCipher", "SSLMode", "CreatedAt", "UpdatedAt").
		Create(entity).Error
	return mapDB(err)
}

func (r *Repository) SaveSource(ctx context.Context, entity *Datasource) error {
	owner, err := ownerID(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&Datasource{}).Where("id = ? AND account_id = ?", entity.ID, owner).Updates(map[string]any{
		"name":            entity.Name,
		"driver":          entity.Driver,
		"host":            entity.Host,
		"port":            entity.Port,
		"database_name":   entity.DatabaseName,
		"username":        entity.Username,
		"password_cipher": entity.PasswordCipher,
		"ssl_mode":        entity.SSLMode,
		"updated_at":      now,
	})
	if res.Error != nil {
		return mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound("数据源不存在")
	}
	entity.UpdatedAt = now
	return nil
}

func (r *Repository) DeleteSource(ctx context.Context, id int64) error {
	owner, err := ownerID(ctx)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).Delete(&Datasource{})
	if res.Error != nil {
		return mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound("数据源不存在")
	}
	return nil
}

func (r *Repository) FindSource(ctx context.Context, id int64) (Datasource, error) {
	var entity Datasource
	owner, err := ownerID(ctx)
	if err != nil {
		return entity, err
	}
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", id, owner).First(&entity).Error
	if err != nil {
		return entity, mapDB(err)
	}
	return entity, nil
}

func (r *Repository) SearchSource(ctx context.Context, search *DatasourceSearch) (response.PageResponse[DatasourceSimple], error) {
	var result response.PageResponse[DatasourceSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Datasource{}).Where("account_id = ?", owner)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			query = query.Where("name LIKE ? ESCAPE '\\'", likePattern(keyword))
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []Datasource
	offset := (search.Page - 1) * search.PageSize
	err = base().Order("updated_at DESC, id DESC").Offset(offset).Limit(search.PageSize).Find(&rows).Error
	if err != nil {
		return result, mapDB(err)
	}
	items := make([]DatasourceSimple, 0, len(rows))
	for i := range rows {
		items = append(items, *DatasourceEntityToSimple(&rows[i]))
	}
	return response.PageResponse[DatasourceSimple]{Data: items, Total: total, Page: search.Page, PageSize: search.PageSize}, nil
}

func (r *Repository) EnsureWallet(ctx context.Context) (int64, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return 0, err
	}
	var balance int64
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var wallet Wallet
		err := tx.Where("account_id = ?", owner).First(&wallet).Error
		if err == nil {
			balance = wallet.Balance
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := time.Now().UTC()
		wallet = Wallet{AccountID: owner, Balance: WelcomeCredits, UpdatedAt: now}
		if err := tx.Select("AccountID", "Balance", "UpdatedAt").Create(&wallet).Error; err != nil {
			if !isUnique(err) {
				return err
			}
			if err := tx.Where("account_id = ?", owner).First(&wallet).Error; err != nil {
				return err
			}
			balance = wallet.Balance
			return nil
		}
		entry := BillingEntry{
			AccountID:    owner,
			Kind:         "grant",
			Amount:       WelcomeCredits,
			BalanceAfter: WelcomeCredits,
			RefType:      "welcome",
			Note:         "新用户赠送额度",
			CreatedAt:    now,
		}
		if err := tx.Select("AccountID", "Kind", "Amount", "BalanceAfter", "RefType", "RefID", "Note", "CreatedAt").Create(&entry).Error; err != nil {
			return err
		}
		balance = WelcomeCredits
		return nil
	})
	if err != nil {
		return 0, mapDB(err)
	}
	return balance, nil
}

func (r *Repository) Topup(ctx context.Context, amount int64, note string) (TopupResponse, error) {
	var result TopupResponse
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	if _, err := r.EnsureWallet(ctx); err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var wallet Wallet
		if err := tx.Where("account_id = ?", owner).First(&wallet).Error; err != nil {
			return err
		}
		if wallet.Balance > 1_000_000_000-amount {
			return apperr.Invalid("余额超出上限")
		}
		now := time.Now().UTC()
		res := tx.Model(&Wallet{}).Where("account_id = ?", owner).Updates(map[string]any{
			"balance":    gorm.Expr("balance + ?", amount),
			"updated_at": now,
		})
		if res.Error != nil {
			return res.Error
		}
		if err := tx.Where("account_id = ?", owner).First(&wallet).Error; err != nil {
			return err
		}
		entry := BillingEntry{
			AccountID:    owner,
			Kind:         "topup",
			Amount:       amount,
			BalanceAfter: wallet.Balance,
			RefType:      "topup",
			Note:         note,
			CreatedAt:    now,
		}
		if err := tx.Select("AccountID", "Kind", "Amount", "BalanceAfter", "RefType", "RefID", "Note", "CreatedAt").Create(&entry).Error; err != nil {
			return err
		}
		result.Balance = wallet.Balance
		result.Entry = *BillingEntityToSimple(&entry)
		return nil
	})
	if err != nil {
		return result, mapDB(err)
	}
	return result, nil
}

func (r *Repository) SearchBilling(ctx context.Context, search *BillingSearch) (response.PageResponse[BillingSimple], error) {
	var result response.PageResponse[BillingSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&BillingEntry{}).Where("account_id = ?", owner)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []BillingEntry
	offset := (search.Page - 1) * search.PageSize
	if err := base().Order("id DESC").Offset(offset).Limit(search.PageSize).Find(&rows).Error; err != nil {
		return result, mapDB(err)
	}
	items := make([]BillingSimple, 0, len(rows))
	for i := range rows {
		items = append(items, *BillingEntityToSimple(&rows[i]))
	}
	return response.PageResponse[BillingSimple]{Data: items, Total: total, Page: search.Page, PageSize: search.PageSize}, nil
}

func (r *Repository) SaveUsage(ctx context.Context, record *UsageRecord) (int64, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return 0, err
	}
	record.AccountID = owner
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.Day = record.CreatedAt.UTC().Format("2006-01-02")
	var balance int64
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("AccountID", "DatasourceID", "DatasourceName", "Question", "SQLText", "Status", "RowCount", "DurationMs", "PromptTokens", "CompletionTokens", "Cost", "ErrorMessage", "Day", "CreatedAt").Create(record).Error; err != nil {
			return err
		}
		if record.Cost <= 0 {
			var wallet Wallet
			if err := tx.Where("account_id = ?", owner).First(&wallet).Error; err != nil {
				return err
			}
			balance = wallet.Balance
			return nil
		}
		now := time.Now().UTC()
		res := tx.Model(&Wallet{}).Where("account_id = ? AND balance >= ?", owner, record.Cost).Updates(map[string]any{
			"balance":    gorm.Expr("balance - ?", record.Cost),
			"updated_at": now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return apperr.Forbidden("余额不足，请到账单页充值")
		}
		var wallet Wallet
		if err := tx.Where("account_id = ?", owner).First(&wallet).Error; err != nil {
			return err
		}
		balance = wallet.Balance
		entry := BillingEntry{
			AccountID:    owner,
			Kind:         "consume",
			Amount:       record.Cost,
			BalanceAfter: balance,
			RefType:      "usage",
			RefID:        record.ID,
			Note:         "ChatBI 问数",
			CreatedAt:    now,
		}
		return tx.Select("AccountID", "Kind", "Amount", "BalanceAfter", "RefType", "RefID", "Note", "CreatedAt").Create(&entry).Error
	})
	if err != nil {
		return 0, mapDB(err)
	}
	return balance, nil
}

func (r *Repository) SearchUsage(ctx context.Context, search *UsageSearch) (response.PageResponse[UsageSimple], error) {
	var result response.PageResponse[UsageSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&UsageRecord{}).Where("account_id = ?", owner)
		if search.DatasourceID > 0 {
			query = query.Where("datasource_id = ?", search.DatasourceID)
		}
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			query = query.Where("question LIKE ? ESCAPE '\\'", likePattern(keyword))
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []UsageRecord
	offset := (search.Page - 1) * search.PageSize
	if err := base().Order("id DESC").Offset(offset).Limit(search.PageSize).Find(&rows).Error; err != nil {
		return result, mapDB(err)
	}
	items := make([]UsageSimple, 0, len(rows))
	for i := range rows {
		items = append(items, *UsageEntityToSimple(&rows[i]))
	}
	return response.PageResponse[UsageSimple]{Data: items, Total: total, Page: search.Page, PageSize: search.PageSize}, nil
}

func (r *Repository) Summarize(ctx context.Context, days int) (UsageSummary, error) {
	var summary UsageSummary
	owner, err := ownerID(ctx)
	if err != nil {
		return summary, err
	}
	balance, err := r.EnsureWallet(ctx)
	if err != nil {
		return summary, err
	}
	summary.Balance = balance
	today := time.Now().UTC()
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(days - 1))
	since := start.Format("2006-01-02")
	row := struct {
		QueryCount   int64
		SuccessCount int64
		FailCount    int64
		TokenCount   int64
		Cost         int64
		RowCount     int64
	}{}
	err = r.db.WithContext(ctx).Raw(`
SELECT COUNT(1) AS query_count,
       COALESCE(SUM(CASE WHEN status = 'ok' THEN 1 ELSE 0 END), 0) AS success_count,
       COALESCE(SUM(CASE WHEN status <> 'ok' THEN 1 ELSE 0 END), 0) AS fail_count,
       COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS token_count,
       COALESCE(SUM(cost), 0) AS cost,
       COALESCE(SUM(row_count), 0) AS row_count
FROM usage_record
WHERE account_id = ? AND day >= ?`, owner, since).Scan(&row).Error
	if err != nil {
		return summary, mapDB(err)
	}
	summary.QueryCount = row.QueryCount
	summary.SuccessCount = row.SuccessCount
	summary.FailCount = row.FailCount
	summary.TokenCount = row.TokenCount
	summary.Cost = row.Cost
	summary.RowCount = row.RowCount
	var dayRows []struct {
		Day          string
		QueryCount   int64
		SuccessCount int64
		Cost         int64
		TokenCount   int64
	}
	err = r.db.WithContext(ctx).Raw(`
SELECT day,
       COUNT(1) AS query_count,
       COALESCE(SUM(CASE WHEN status = 'ok' THEN 1 ELSE 0 END), 0) AS success_count,
       COALESCE(SUM(cost), 0) AS cost,
       COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS token_count
FROM usage_record
WHERE account_id = ? AND day >= ?
GROUP BY day
ORDER BY day`, owner, since).Scan(&dayRows).Error
	if err != nil {
		return summary, mapDB(err)
	}
	byDay := map[string]UsageDay{}
	for _, item := range dayRows {
		byDay[item.Day] = UsageDay{Date: item.Day, QueryCount: item.QueryCount, SuccessCount: item.SuccessCount, Cost: item.Cost, TokenCount: item.TokenCount}
	}
	summary.Days = make([]UsageDay, 0, days)
	for i := 0; i < days; i++ {
		key := start.AddDate(0, 0, i).Format("2006-01-02")
		if item, ok := byDay[key]; ok {
			summary.Days = append(summary.Days, item)
			continue
		}
		summary.Days = append(summary.Days, UsageDay{Date: key})
	}
	var sources []UsageSource
	err = r.db.WithContext(ctx).Raw(`
SELECT datasource_id, datasource_name,
       COUNT(1) AS query_count,
       COALESCE(SUM(CASE WHEN status = 'ok' THEN 1 ELSE 0 END), 0) AS success_count,
       COALESCE(SUM(cost), 0) AS cost
FROM usage_record
WHERE account_id = ? AND day >= ?
GROUP BY datasource_id, datasource_name
ORDER BY query_count DESC, datasource_id DESC`, owner, since).Scan(&sources).Error
	if err != nil {
		return summary, mapDB(err)
	}
	if sources == nil {
		sources = []UsageSource{}
	}
	summary.Datasources = sources
	return summary, nil
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
		return apperr.NotFound("数据源不存在")
	}
	if isUnique(err) {
		return apperr.Conflict("数据源名称已存在")
	}
	return apperr.Internal(err)
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique")
}
