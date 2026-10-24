package chat

import "time"

type Conversation struct {
	ID           int64 `gorm:"primaryKey;autoIncrement"`
	AccountID    int64
	Title        string
	Model        string
	ThinkingMode string
	WebSearch    bool
	AgentID      int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Conversation) TableName() string { return "conversation" }

type Message struct {
	ID             int64 `gorm:"primaryKey;autoIncrement"`
	ConversationID int64
	Role           string
	Content        string
	Thinking       string
	ThinkingMode   string
	Chart          string
	Tools          string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (Message) TableName() string { return "message" }

type Attachment struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	MessageID int64
	Name      string
	MediaType string
	Content   string
	CreatedAt time.Time
}

func (Attachment) TableName() string { return "attachment" }
