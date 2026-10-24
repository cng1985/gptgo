package toolkit

import "time"

type ToolSearch struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type BuiltinTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type BuiltinUpdate struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type HTTPToolRequest struct {
	ID          int64  `json:"id,string"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  string `json:"parameters"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	Headers     string `json:"headers"`
	Enabled     bool   `json:"enabled"`
}

type HTTPToolSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type HTTPToolSimple struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Method      string    `json:"method"`
	URL         string    `json:"url"`
	Enabled     bool      `json:"enabled"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type HTTPToolResponse struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Parameters  string    `json:"parameters"`
	Method      string    `json:"method"`
	URL         string    `json:"url"`
	HasHeaders  bool      `json:"has_headers"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MCPRequest struct {
	ID        int64    `json:"id,string"`
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Env       string   `json:"env"`
	URL       string   `json:"url"`
	Headers   string   `json:"headers"`
	Enabled   bool     `json:"enabled"`
}

type MCPSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type MCPSimple struct {
	ID        int64     `json:"id,string"`
	Name      string    `json:"name"`
	Transport string    `json:"transport"`
	URL       string    `json:"url"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MCPResponse struct {
	ID         int64     `json:"id,string"`
	Name       string    `json:"name"`
	Transport  string    `json:"transport"`
	Command    string    `json:"command"`
	Args       []string  `json:"args"`
	URL        string    `json:"url"`
	HasEnv     bool      `json:"has_env"`
	HasHeaders bool      `json:"has_headers"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type MCPToolSimple struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
