package toolkit

import "time"

type ToolPref struct {
	AccountID int64  `gorm:"primaryKey"`
	Name      string `gorm:"primaryKey"`
	Enabled   bool
}

func (ToolPref) TableName() string { return "tool_pref" }

type HTTPTool struct {
	ID          int64 `gorm:"primaryKey;autoIncrement"`
	AccountID   int64
	Name        string
	Description string
	Parameters  string
	Method      string
	URL         string
	Headers     string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (HTTPTool) TableName() string { return "http_tool" }

type MCPServer struct {
	ID            int64 `gorm:"primaryKey;autoIncrement"`
	AccountID     int64
	Name          string
	Transport     string
	Command       string
	Args          string
	EnvCipher     string
	URL           string
	HeadersCipher string
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (MCPServer) TableName() string { return "mcp_server" }
