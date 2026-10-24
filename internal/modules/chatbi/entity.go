package chatbi

import "time"

type Datasource struct {
	ID             int64 `gorm:"primaryKey;autoIncrement"`
	AccountID      int64
	Name           string
	Driver         string
	Host           string
	Port           int
	DatabaseName   string
	Username       string
	PasswordCipher string
	SSLMode        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Datasource) TableName() string { return "datasource" }

type Wallet struct {
	AccountID int64 `gorm:"primaryKey"`
	Balance   int64
	UpdatedAt time.Time
}

func (Wallet) TableName() string { return "wallet" }

type BillingEntry struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	AccountID    int64
	Kind         string
	Amount       int64
	BalanceAfter int64
	RefType      string
	RefID        int64
	Note         string
	CreatedAt    time.Time
}

func (BillingEntry) TableName() string { return "billing_entry" }

type UsageRecord struct {
	ID               int64 `gorm:"primaryKey;autoIncrement"`
	AccountID        int64
	DatasourceID     int64
	DatasourceName   string
	Question         string
	SQLText          string
	Status           string
	RowCount         int64
	DurationMs       int64
	PromptTokens     int64
	CompletionTokens int64
	Cost             int64
	ErrorMessage     string
	Day              string
	CreatedAt        time.Time
}

func (UsageRecord) TableName() string { return "usage_record" }
