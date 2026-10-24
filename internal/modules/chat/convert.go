package chat

import "strings"

func ConversationEntityToSimple(entity *Conversation) *ConversationSimple {
	return &ConversationSimple{
		ID:        entity.ID,
		Title:     entity.Title,
		Model:     entity.Model,
		UpdatedAt: entity.UpdatedAt,
	}
}

func ConversationEntityToResponse(entity *Conversation) *ConversationResponse {
	return &ConversationResponse{
		ID:           entity.ID,
		Title:        entity.Title,
		Model:        entity.Model,
		ThinkingMode: entity.ThinkingMode,
		WebSearch:    entity.WebSearch,
		CreatedAt:    entity.CreatedAt,
		UpdatedAt:    entity.UpdatedAt,
	}
}

func MessageEntityToSimple(entity *Message, attachments []Attachment) *MessageSimple {
	items := make([]AttachmentSimple, 0, len(attachments))
	for _, attachment := range attachments {
		items = append(items, AttachmentSimple{
			ID:        attachment.ID,
			Name:      attachment.Name,
			MediaType: attachment.MediaType,
		})
	}
	return &MessageSimple{
		ID:             entity.ID,
		ConversationID: entity.ConversationID,
		Role:           entity.Role,
		Content:        entity.Content,
		Thinking:       entity.Thinking,
		Status:         entity.Status,
		Attachments:    items,
		CreatedAt:      entity.CreatedAt,
	}
}

func titleFrom(content string) string {
	line := strings.TrimSpace(content)
	if line == "" {
		return "新对话"
	}
	if idx := strings.IndexAny(line, "\r\n"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	line = strings.Join(strings.Fields(line), " ")
	runes := []rune(line)
	if len(runes) > 36 {
		return string(runes[:36])
	}
	if line == "" {
		return "新对话"
	}
	return line
}
