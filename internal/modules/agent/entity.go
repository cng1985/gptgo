package agent

import "time"

type Agent struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	AccountID    int64
	Name         string
	Description  string
	Instructions string
	Model        string
	Opening      string
	Suggestions  string
	Tools        string
	McpIDs       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Agent) TableName() string { return "agent" }
