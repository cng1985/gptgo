package agent

import "time"

var Models = map[string]string{
	"minimax-m3.1-flash": "MiniMax-M3.1-Flash",
	"minimax-m2.5":       "MiniMax-M2.5",
	"deepseek-chat":      "DeepSeek-V3",
	"builtin":            "内置助手",
}

type AgentRequest struct {
	ID           int64    `json:"id,string"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Instructions string   `json:"instructions"`
	Model        string   `json:"model"`
	Opening      string   `json:"opening"`
	Suggestions  []string `json:"suggestions"`
}

type AgentSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type AgentSimple struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Model       string    `json:"model"`
	Opening     string    `json:"opening"`
	Suggestions []string  `json:"suggestions"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AgentResponse struct {
	ID           int64     `json:"id,string"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Instructions string    `json:"instructions"`
	Model        string    `json:"model"`
	Opening      string    `json:"opening"`
	Suggestions  []string  `json:"suggestions"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
