package skill

import "time"

type Skill struct {
	ID          int64 `gorm:"primaryKey;autoIncrement"`
	AccountID   int64
	Name        string
	DisplayName string
	Description string
	Version     string
	Source      string
	Namespace   string
	Slug        string
	Registry    string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Skill) TableName() string { return "skill" }
