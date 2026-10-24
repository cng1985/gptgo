package mcp

import "time"

type Server struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	AccountID    int64
	Name         string
	Transport    string
	URL          string
	Command      string
	Args         string
	HeaderCipher string
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Server) TableName() string { return "mcp_server" }
