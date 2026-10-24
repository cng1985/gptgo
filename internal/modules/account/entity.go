package account

import "time"

type Account struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	Name         string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Account) TableName() string { return "account" }

type LoginSession struct {
	Token     string `gorm:"primaryKey"`
	AccountID int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (LoginSession) TableName() string { return "login_session" }
