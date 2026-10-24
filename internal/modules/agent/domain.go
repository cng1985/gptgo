package agent

import (
	"encoding/json"
	"strconv"
	"time"
)

type IDList []int64

func (l IDList) MarshalJSON() ([]byte, error) {
	if l == nil {
		l = IDList{}
	}
	texts := make([]string, len(l))
	for i, id := range l {
		texts[i] = strconv.FormatInt(id, 10)
	}
	return json.Marshal(texts)
}

func (l *IDList) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*l = IDList{}
		return nil
	}
	var texts []string
	if err := json.Unmarshal(raw, &texts); err == nil {
		out := make(IDList, 0, len(texts))
		for _, text := range texts {
			id, err := strconv.ParseInt(text, 10, 64)
			if err != nil || id <= 0 {
				return strconv.ErrSyntax
			}
			out = append(out, id)
		}
		*l = out
		return nil
	}
	var nums []int64
	if err := json.Unmarshal(raw, &nums); err != nil {
		return err
	}
	for _, id := range nums {
		if id <= 0 {
			return strconv.ErrSyntax
		}
	}
	*l = IDList(nums)
	return nil
}

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
	Tools        []string `json:"tools"`
	McpIDs       IDList   `json:"mcp_ids"`
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
	Tools        []string  `json:"tools"`
	McpIDs       IDList    `json:"mcp_ids"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
