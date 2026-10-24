package chatbi

import "time"

const (
	WelcomeCredits int64 = 1000
	AskCost        int64 = 2
	MaxRows              = 200
)

var Models = map[string]struct{}{
	"minimax-m3.1-flash": {},
	"minimax-m2.5":       {},
	"deepseek-chat":      {},
	"builtin":            {},
}

type DatasourceRequest struct {
	ID       int64  `json:"id,string"`
	Name     string `json:"name"`
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"`
}

type DatasourceSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type DatasourceSimple struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	Driver      string    `json:"driver"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Database    string    `json:"database"`
	Username    string    `json:"username"`
	SSLMode     string    `json:"ssl_mode"`
	PasswordSet bool      `json:"password_set"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DatasourceResponse struct {
	ID          int64     `json:"id,string"`
	Name        string    `json:"name"`
	Driver      string    `json:"driver"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Database    string    `json:"database"`
	Username    string    `json:"username"`
	SSLMode     string    `json:"ssl_mode"`
	PasswordSet bool      `json:"password_set"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DatasourceTestResponse struct {
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	TableCount int    `json:"table_count"`
}

type ColumnInfo struct {
	Name     string `json:"name"`
	DataType string `json:"data_type"`
}

type TableInfo struct {
	Schema  string       `json:"schema"`
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns"`
}

type SchemaResponse struct {
	Tables []TableInfo `json:"tables"`
}

type AskRequest struct {
	DatasourceID int64  `json:"datasource_id,string"`
	Question     string `json:"question"`
	Model        string `json:"model"`
}

type AskResponse struct {
	UsageID          int64    `json:"usage_id,string"`
	SQL              string   `json:"sql"`
	Columns          []string `json:"columns"`
	Rows             [][]any  `json:"rows"`
	Chart            string   `json:"chart"`
	Summary          string   `json:"summary"`
	RowCount         int      `json:"row_count"`
	Truncated        bool     `json:"truncated"`
	DurationMs       int64    `json:"duration_ms"`
	Cost             int64    `json:"cost"`
	Balance          int64    `json:"balance"`
	PromptTokens     int64    `json:"prompt_tokens"`
	CompletionTokens int64    `json:"completion_tokens"`
}

type WalletResponse struct {
	Balance        int64 `json:"balance"`
	WelcomeCredits int64 `json:"welcome_credits"`
	AskCost        int64 `json:"ask_cost"`
}

type TopupRequest struct {
	Amount int64  `json:"amount"`
	Note   string `json:"note"`
}

type BillingSimple struct {
	ID           int64     `json:"id,string"`
	Kind         string    `json:"kind"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	RefType      string    `json:"ref_type"`
	RefID        int64     `json:"ref_id,string"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
}

type TopupResponse struct {
	Balance int64         `json:"balance"`
	Entry   BillingSimple `json:"entry"`
}

type BillingSearch struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type UsageSearch struct {
	Keyword      string `json:"keyword"`
	DatasourceID int64  `json:"datasource_id,string"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
}

type UsageSimple struct {
	ID               int64     `json:"id,string"`
	DatasourceID     int64     `json:"datasource_id,string"`
	DatasourceName   string    `json:"datasource_name"`
	Question         string    `json:"question"`
	SQL              string    `json:"sql"`
	Status           string    `json:"status"`
	RowCount         int64     `json:"row_count"`
	DurationMs       int64     `json:"duration_ms"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	Cost             int64     `json:"cost"`
	ErrorMessage     string    `json:"error_message"`
	CreatedAt        time.Time `json:"created_at"`
}

type UsageSummaryRequest struct {
	Days int `json:"days"`
}

type UsageDay struct {
	Date         string `json:"date"`
	QueryCount   int64  `json:"query_count"`
	SuccessCount int64  `json:"success_count"`
	Cost         int64  `json:"cost"`
	TokenCount   int64  `json:"token_count"`
}

type UsageSource struct {
	DatasourceID   int64  `json:"datasource_id,string"`
	DatasourceName string `json:"datasource_name"`
	QueryCount     int64  `json:"query_count"`
	SuccessCount   int64  `json:"success_count"`
	Cost           int64  `json:"cost"`
}

type UsageSummary struct {
	Balance      int64         `json:"balance"`
	QueryCount   int64         `json:"query_count"`
	SuccessCount int64         `json:"success_count"`
	FailCount    int64         `json:"fail_count"`
	TokenCount   int64         `json:"token_count"`
	Cost         int64         `json:"cost"`
	RowCount     int64         `json:"row_count"`
	Days         []UsageDay    `json:"days"`
	Datasources  []UsageSource `json:"datasources"`
}
