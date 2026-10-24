package account

import "time"

type Account struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Account) TableName() string { return "account" }
