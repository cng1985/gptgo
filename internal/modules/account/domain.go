package account

import "time"

type AccountRequest struct {
	Name string `json:"name"`
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LogoutResponse struct {
	OK bool `json:"ok"`
}

type AccountResponse struct {
	ID            int64     `json:"id,string"`
	Name          string    `json:"name"`
	Username      string    `json:"username"`
	Provider      string    `json:"provider"`
	ProviderReady bool      `json:"provider_ready"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
