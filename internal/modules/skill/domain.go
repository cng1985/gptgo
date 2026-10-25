package skill

import "time"

type SkillSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type SkillRequest struct {
	ID      int64 `json:"id,string"`
	Enabled bool  `json:"enabled"`
}

type SkillSimple struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	Source      string    `json:"source"`
	Namespace   string    `json:"namespace"`
	Slug        string    `json:"slug"`
	Registry    string    `json:"registry"`
	Enabled     bool      `json:"enabled"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SkillResponse struct {
	SkillSimple
	Files     []string  `json:"files"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type HubSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type HubInstall struct {
	Namespace string `json:"namespace"`
	Slug      string `json:"slug"`
	Version   string `json:"version"`
}

type HubSkill struct {
	Namespace   string `json:"namespace"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type Entry struct {
	Name        string
	DisplayName string
	Description string
}
