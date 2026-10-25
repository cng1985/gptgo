package skill

import "time"

type SkillRequest struct {
	ID           int64  `json:"id,string"`
	Title        string `json:"title"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
	Runtime      string `json:"runtime"`
	Script       string `json:"script"`
	Parameters   string `json:"parameters"`
	TimeoutSec   int    `json:"timeout_sec"`
	Enabled      bool   `json:"enabled"`
}

type SkillSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type SkillRunRequest struct {
	ID         int64  `json:"id,string"`
	Name       string `json:"name"`
	Runtime    string `json:"runtime"`
	Script     string `json:"script"`
	Input      string `json:"input"`
	TimeoutSec int    `json:"timeout_sec"`
}

type SkillSimple struct {
	ID          int64     `json:"id,string"`
	Title       string    `json:"title"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Runtime     string    `json:"runtime"`
	TimeoutSec  int       `json:"timeout_sec"`
	Enabled     bool      `json:"enabled"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SkillResponse struct {
	ID           int64     `json:"id,string"`
	Title        string    `json:"title"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Instructions string    `json:"instructions"`
	Runtime      string    `json:"runtime"`
	Script       string    `json:"script"`
	Parameters   string    `json:"parameters"`
	TimeoutSec   int       `json:"timeout_sec"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SkillRunResult struct {
	OK         bool   `json:"ok"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMS int64  `json:"duration_ms"`
}
