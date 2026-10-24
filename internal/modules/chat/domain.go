package chat

import (
	"encoding/json"
	"time"
)

var Models = map[string]string{
	"minimax-m3.1-flash": "MiniMax-M3.1-Flash",
	"minimax-m2.5":       "MiniMax-M2.5",
	"deepseek-chat":      "DeepSeek-V3",
	"builtin":            "内置助手",
}

type ConversationRequest struct {
	ID           int64  `json:"id,string"`
	Title        string `json:"title"`
	Model        string `json:"model"`
	ThinkingMode string `json:"thinking_mode"`
	WebSearch    bool   `json:"web_search"`
}

type ConversationSearch struct {
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type ConversationSimple struct {
	ID        int64     `json:"id,string"`
	Title     string    `json:"title"`
	Model     string    `json:"model"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationResponse struct {
	ID           int64     `json:"id,string"`
	Title        string    `json:"title"`
	Model        string    `json:"model"`
	ThinkingMode string    `json:"thinking_mode"`
	WebSearch    bool      `json:"web_search"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ConversationClearRequest struct {
	Confirm bool `json:"confirm"`
}

type AttachmentInput struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Content   string `json:"content"`
}

type AttachmentSimple struct {
	ID        int64  `json:"id,string"`
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
}

type MessageSearch struct {
	ConversationID int64 `json:"conversation_id,string"`
	Page           int   `json:"page"`
	PageSize       int   `json:"page_size"`
}

type MessageSimple struct {
	ID             int64              `json:"id,string"`
	ConversationID int64              `json:"conversation_id,string"`
	Role           string             `json:"role"`
	Content        string             `json:"content"`
	Thinking       string             `json:"thinking"`
	Chart          json.RawMessage    `json:"chart,omitempty"`
	Status         string             `json:"status"`
	Attachments    []AttachmentSimple `json:"attachments"`
	CreatedAt      time.Time          `json:"created_at"`
}

type ModelTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatStreamRequest struct {
	ConversationID      int64             `json:"conversation_id,string"`
	Content             string            `json:"content"`
	Model               string            `json:"model"`
	ThinkingMode        string            `json:"thinking_mode"`
	WebSearch           bool              `json:"web_search"`
	Attachments         []AttachmentInput `json:"attachments"`
	RegenerateMessageID int64             `json:"regenerate_message_id,string"`
	EditMessageID       int64             `json:"edit_message_id,string"`
	SearchNotes         string            `json:"-"`
}

type ChatPrepareResponse struct {
	ConversationID     int64       `json:"conversation_id,string"`
	UserMessageID      int64       `json:"user_message_id,string"`
	AssistantMessageID int64       `json:"assistant_message_id,string"`
	Title              string      `json:"title"`
	Model              string      `json:"model"`
	ThinkingMode       string      `json:"thinking_mode"`
	Messages           []ModelTurn `json:"messages"`
	SearchNotes        string      `json:"search_notes,omitempty"`
}

type ChatCompleteRequest struct {
	ConversationID     int64  `json:"conversation_id,string"`
	AssistantMessageID int64  `json:"assistant_message_id,string"`
	Content            string `json:"content"`
	Thinking           string `json:"thinking"`
	Chart              string `json:"chart,omitempty"`
	Status             string `json:"status"`
}

type ChatCompleteResponse struct {
	ConversationID int64  `json:"conversation_id,string"`
	MessageID      int64  `json:"message_id,string"`
	Status         string `json:"status"`
}

type ChatOptimizeRequest struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

type ChatOptimizeResponse struct {
	Content string `json:"content"`
}
