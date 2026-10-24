package mcp

import (
	"encoding/json"
	"time"
)

type ServerRequest struct {
	ID        int64           `json:"id,string"`
	Name      string          `json:"name"`
	Transport string          `json:"transport"`
	URL       string          `json:"url"`
	Command   string          `json:"command"`
	Args      []string        `json:"args"`
	Headers   json.RawMessage `json:"headers"`
	Enabled   *bool           `json:"enabled"`
}

type ServerSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type ServerSimple struct {
	ID         int64     `json:"id,string"`
	Name       string    `json:"name"`
	Transport  string    `json:"transport"`
	Enabled    bool      `json:"enabled"`
	HeaderKeys []string  `json:"header_keys"`
	HeadersSet bool      `json:"headers_set"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ServerResponse struct {
	ID         int64     `json:"id,string"`
	Name       string    `json:"name"`
	Transport  string    `json:"transport"`
	URL        string    `json:"url"`
	Command    string    `json:"command"`
	Args       []string  `json:"args"`
	HeaderKeys []string  `json:"header_keys"`
	HeadersSet bool      `json:"headers_set"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type RemoteToolView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type TestResponse struct {
	Tools []RemoteToolView `json:"tools"`
}

type BuiltinTool struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
}
