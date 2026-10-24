package account

import "time"

type AccountRequest struct {
	Name string `json:"name"`
}

type AccountResponse struct {
	ID            int64     `json:"id,string"`
	Name          string    `json:"name"`
	Provider      string    `json:"provider"`
	ProviderReady bool      `json:"provider_ready"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
