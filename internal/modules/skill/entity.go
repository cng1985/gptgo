package skill

import "time"

type Skill struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	AccountID    int64
	Title        string
	Name         string
	Description  string
	Instructions string
	Runtime      string
	Script       string
	Parameters   string
	TimeoutSec   int
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Skill) TableName() string { return "skill" }
