package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"gptgo/internal/platform/domain"
	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
	"gptgo/internal/platform/search"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, request *ConversationRequest) (response.Response[ConversationResponse], error) {
	var result response.Response[ConversationResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	entity := Conversation{
		AccountID:    owner,
		Title:        request.Title,
		Model:        request.Model,
		ThinkingMode: request.ThinkingMode,
		WebSearch:    request.WebSearch,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return result, mapDB(err)
	}
	result = response.OK(*ConversationEntityToResponse(&entity))
	return result, nil
}

func (r *Repository) Update(ctx context.Context, request *ConversationRequest) (response.Response[ConversationResponse], error) {
	var result response.Response[ConversationResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	title := strings.TrimSpace(request.Title)
	res := r.db.WithContext(ctx).Model(&Conversation{}).Where("id = ? AND account_id = ?", request.ID, owner).Updates(map[string]any{
		"title":      title,
		"updated_at": time.Now().UTC(),
	})
	if res.Error != nil {
		return result, mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return result, apperr.NotFound("对话不存在")
	}
	return r.One(ctx, request)
}

func (r *Repository) One(ctx context.Context, request *ConversationRequest) (response.Response[ConversationResponse], error) {
	var result response.Response[ConversationResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var entity Conversation
	err = r.db.WithContext(ctx).Where("id = ? AND account_id = ?", request.ID, owner).First(&entity).Error
	if err != nil {
		return result, mapDB(err)
	}
	result = response.OK(*ConversationEntityToResponse(&entity))
	return result, nil
}

func (r *Repository) Search(ctx context.Context, search *ConversationSearch) (response.PageResponse[ConversationSimple], error) {
	var result response.PageResponse[ConversationSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	offset := int64(search.Page-1) * int64(search.PageSize)
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Conversation{}).Where("account_id = ?", owner)
		if keyword := strings.TrimSpace(search.Keyword); keyword != "" {
			query = query.Where("title LIKE ? ESCAPE '\\'", likePattern(keyword))
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	var rows []Conversation
	err = base().Order("updated_at desc").Order("id desc").Offset(int(offset)).Limit(search.PageSize).Find(&rows).Error
	if err != nil {
		return result, mapDB(err)
	}
	data := make([]ConversationSimple, 0, len(rows))
	for i := range rows {
		data = append(data, *ConversationEntityToSimple(&rows[i]))
	}
	return response.PageResponse[ConversationSimple]{
		Data: data, Total: total, Page: search.Page, PageSize: search.PageSize,
	}, nil
}

func (r *Repository) Delete(ctx context.Context, request *ConversationRequest) (response.Response[ConversationResponse], error) {
	var result response.Response[ConversationResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var entity Conversation
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND account_id = ?", request.ID, owner).First(&entity).Error; err != nil {
			return err
		}
		if err := deleteConversationGraph(tx, request.ID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return result, mapDB(err)
	}
	result = response.OK(*ConversationEntityToResponse(&entity))
	return result, nil
}

func (r *Repository) Clear(ctx context.Context, _ *ConversationClearRequest) (response.Response[ConversationResponse], error) {
	var result response.Response[ConversationResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.Model(&Conversation{}).Where("account_id = ?", owner).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		var messageIDs []int64
		if err := tx.Model(&Message{}).Where("conversation_id IN ?", ids).Pluck("id", &messageIDs).Error; err != nil {
			return err
		}
		if len(messageIDs) > 0 {
			if err := tx.Where("message_id IN ?", messageIDs).Delete(&Attachment{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("conversation_id IN ?", ids).Delete(&Message{}).Error; err != nil {
			return err
		}
		return tx.Where("account_id = ?", owner).Delete(&Conversation{}).Error
	})
	if err != nil {
		return result, mapDB(err)
	}
	return response.OK(ConversationResponse{}), nil
}

func (r *Repository) SearchMessages(ctx context.Context, search *MessageSearch) (response.PageResponse[MessageSimple], error) {
	var result response.PageResponse[MessageSimple]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	var exists int64
	if err := r.db.WithContext(ctx).Model(&Conversation{}).Where("id = ? AND account_id = ?", search.ConversationID, owner).Count(&exists).Error; err != nil {
		return result, mapDB(err)
	}
	if exists == 0 {
		return result, apperr.NotFound("对话不存在")
	}
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Model(&Message{}).Where("conversation_id = ?", search.ConversationID)
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return result, mapDB(err)
	}
	offset := int64(search.Page-1) * int64(search.PageSize)
	var rows []Message
	err = base().Order("id asc").Offset(int(offset)).Limit(search.PageSize).Find(&rows).Error
	if err != nil {
		return result, mapDB(err)
	}
	data, err := r.withAttachments(ctx, rows)
	if err != nil {
		return result, mapDB(err)
	}
	return response.PageResponse[MessageSimple]{
		Data: data, Total: total, Page: search.Page, PageSize: search.PageSize,
	}, nil
}

func (r *Repository) Prepare(ctx context.Context, request *ChatStreamRequest) (response.Response[ChatPrepareResponse], error) {
	var result response.Response[ChatPrepareResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			conversation *Conversation
			userID       int64
			assistantID  int64
			err          error
		)
		switch {
		case request.RegenerateMessageID > 0:
			conversation, userID, assistantID, err = prepareRegenerate(tx, request, owner)
		case request.EditMessageID > 0:
			conversation, userID, assistantID, err = prepareEdit(tx, request, owner)
		default:
			conversation, userID, assistantID, err = prepareNew(tx, request, owner)
		}
		if err != nil {
			return err
		}
		turns, err := modelTurns(tx, conversation.ID, assistantID)
		if err != nil {
			return err
		}
		result = response.OK(ChatPrepareResponse{
			ConversationID:     conversation.ID,
			UserMessageID:      userID,
			AssistantMessageID: assistantID,
			Title:              conversation.Title,
			Model:              conversation.Model,
			ThinkingMode:       conversation.ThinkingMode,
			Messages:           turns,
			SearchNotes:        request.SearchNotes,
			AgentID:            conversation.AgentID,
		})
		return nil
	})
	if err != nil {
		return response.Response[ChatPrepareResponse]{}, mapDB(err)
	}
	return result, nil
}

func (r *Repository) Complete(ctx context.Context, request *ChatCompleteRequest) (response.Response[ChatCompleteResponse], error) {
	var result response.Response[ChatCompleteResponse]
	owner, err := ownerID(ctx)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation Conversation
		if err := tx.Where("id = ? AND account_id = ?", request.ConversationID, owner).First(&conversation).Error; err != nil {
			return err
		}
		var current Message
		if err := tx.Where("id = ? AND conversation_id = ? AND role = ?", request.AssistantMessageID, request.ConversationID, "assistant").First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFound("消息不存在")
			}
			return err
		}
		res := tx.Model(&Message{}).
			Where("id = ?", current.ID).
			Updates(map[string]any{
				"content":    request.Content,
				"thinking":   request.Thinking,
				"chart":      request.Chart,
				"tools":      request.Tools,
				"sources":    mergeSourceExcerpts(current.Sources, request.Sources),
				"status":     request.Status,
				"updated_at": time.Now().UTC(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return apperr.NotFound("消息不存在")
		}
		if err := tx.Model(&Conversation{}).Where("id = ?", request.ConversationID).Update("updated_at", time.Now().UTC()).Error; err != nil {
			return err
		}
		result = response.OK(ChatCompleteResponse{
			ConversationID: request.ConversationID,
			MessageID:      request.AssistantMessageID,
			Status:         request.Status,
		})
		return nil
	})
	if err != nil {
		return response.Response[ChatCompleteResponse]{}, mapDB(err)
	}
	return result, nil
}

func prepareNew(tx *gorm.DB, request *ChatStreamRequest, owner int64) (*Conversation, int64, int64, error) {
	now := time.Now().UTC()
	var conversation Conversation
	if request.ConversationID == 0 {
		conversation = Conversation{
			AccountID:    owner,
			Title:        titleFrom(request.Content),
			Model:        request.Model,
			ThinkingMode: request.ThinkingMode,
			WebSearch:    request.WebSearch,
			AgentID:      request.AgentID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(&conversation).Error; err != nil {
			return nil, 0, 0, err
		}
	} else if err := tx.Where("id = ? AND account_id = ?", request.ConversationID, owner).First(&conversation).Error; err != nil {
		return nil, 0, 0, err
	} else if err := touchConversation(tx, &conversation, request, now); err != nil {
		return nil, 0, 0, err
	}
	user := Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        request.Content,
		Status:         "done",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := tx.Select("ConversationID", "Role", "Content", "Thinking", "Status", "CreatedAt", "UpdatedAt").Create(&user).Error; err != nil {
		return nil, 0, 0, err
	}
	if err := insertAttachments(tx, user.ID, request.Attachments, now); err != nil {
		return nil, 0, 0, err
	}
	assistant := Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "",
		Thinking:       "",
		ThinkingMode:   request.ThinkingMode,
		Status:         "streaming",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := tx.Select("ConversationID", "Role", "Content", "Thinking", "ThinkingMode", "Status", "CreatedAt", "UpdatedAt").Create(&assistant).Error; err != nil {
		return nil, 0, 0, err
	}
	return &conversation, user.ID, assistant.ID, nil
}

func prepareRegenerate(tx *gorm.DB, request *ChatStreamRequest, owner int64) (*Conversation, int64, int64, error) {
	var assistant Message
	if err := tx.Where("id = ? AND role = ?", request.RegenerateMessageID, "assistant").First(&assistant).Error; err != nil {
		return nil, 0, 0, err
	}
	if request.ConversationID > 0 && request.ConversationID != assistant.ConversationID {
		return nil, 0, 0, apperr.NotFound("消息不存在")
	}
	var user Message
	err := tx.Where("conversation_id = ? AND role = ? AND id < ?", assistant.ConversationID, "user", assistant.ID).
		Order("id desc").First(&user).Error
	if err != nil {
		return nil, 0, 0, apperr.NotFound("找不到要重新回答的问题")
	}
	var conversation Conversation
	if err := tx.Where("id = ? AND account_id = ?", assistant.ConversationID, owner).First(&conversation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, 0, apperr.NotFound("消息不存在")
		}
		return nil, 0, 0, err
	}
	now := time.Now().UTC()
	if err := touchConversation(tx, &conversation, request, now); err != nil {
		return nil, 0, 0, err
	}
	res := tx.Model(&Message{}).Where("id = ?", assistant.ID).Updates(map[string]any{
		"content": "", "thinking": "", "chart": "", "tools": "", "sources": "", "status": "streaming", "updated_at": now,
	})
	if res.Error != nil {
		return nil, 0, 0, res.Error
	}
	return &conversation, user.ID, assistant.ID, nil
}

func prepareEdit(tx *gorm.DB, request *ChatStreamRequest, owner int64) (*Conversation, int64, int64, error) {
	var user Message
	if err := tx.Where("id = ? AND role = ?", request.EditMessageID, "user").First(&user).Error; err != nil {
		return nil, 0, 0, err
	}
	if request.ConversationID > 0 && request.ConversationID != user.ConversationID {
		return nil, 0, 0, apperr.NotFound("消息不存在")
	}
	var conversation Conversation
	if err := tx.Where("id = ? AND account_id = ?", user.ConversationID, owner).First(&conversation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, 0, apperr.NotFound("消息不存在")
		}
		return nil, 0, 0, err
	}
	now := time.Now().UTC()
	if err := tx.Model(&Message{}).Where("id = ?", user.ID).Updates(map[string]any{
		"content": request.Content, "updated_at": now,
	}).Error; err != nil {
		return nil, 0, 0, err
	}
	if err := tx.Where("message_id = ?", user.ID).Delete(&Attachment{}).Error; err != nil {
		return nil, 0, 0, err
	}
	if err := insertAttachments(tx, user.ID, request.Attachments, now); err != nil {
		return nil, 0, 0, err
	}
	if err := deleteMessagesAfter(tx, conversation.ID, user.ID); err != nil {
		return nil, 0, 0, err
	}
	if err := touchConversation(tx, &conversation, request, now); err != nil {
		return nil, 0, 0, err
	}
	assistant := Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "",
		Thinking:       "",
		ThinkingMode:   request.ThinkingMode,
		Status:         "streaming",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := tx.Select("ConversationID", "Role", "Content", "Thinking", "ThinkingMode", "Status", "CreatedAt", "UpdatedAt").Create(&assistant).Error; err != nil {
		return nil, 0, 0, err
	}
	return &conversation, user.ID, assistant.ID, nil
}

func touchConversation(tx *gorm.DB, conversation *Conversation, request *ChatStreamRequest, now time.Time) error {
	conversation.Model = request.Model
	conversation.ThinkingMode = request.ThinkingMode
	conversation.WebSearch = request.WebSearch
	conversation.UpdatedAt = now
	return tx.Model(&Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{
		"model":         request.Model,
		"thinking_mode": request.ThinkingMode,
		"web_search":    request.WebSearch,
		"updated_at":    now,
	}).Error
}

func insertAttachments(tx *gorm.DB, messageID int64, inputs []AttachmentInput, now time.Time) error {
	if len(inputs) == 0 {
		return nil
	}
	rows := make([]Attachment, 0, len(inputs))
	for _, input := range inputs {
		rows = append(rows, Attachment{
			MessageID: messageID,
			Name:      input.Name,
			MediaType: input.MediaType,
			Content:   input.Content,
			CreatedAt: now,
		})
	}
	return tx.Create(&rows).Error
}

func deleteMessagesAfter(tx *gorm.DB, conversationID, userID int64) error {
	var ids []int64
	if err := tx.Model(&Message{}).Where("conversation_id = ? AND id > ?", conversationID, userID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Where("message_id IN ?", ids).Delete(&Attachment{}).Error; err != nil {
		return err
	}
	return tx.Where("id IN ?", ids).Delete(&Message{}).Error
}

func deleteConversationGraph(tx *gorm.DB, conversationID int64) error {
	var ids []int64
	if err := tx.Model(&Message{}).Where("conversation_id = ?", conversationID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := tx.Where("message_id IN ?", ids).Delete(&Attachment{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("conversation_id = ?", conversationID).Delete(&Message{}).Error; err != nil {
		return err
	}
	res := tx.Where("id = ?", conversationID).Delete(&Conversation{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound("对话不存在")
	}
	return nil
}

func modelTurns(tx *gorm.DB, conversationID, assistantID int64) ([]ModelTurn, error) {
	var rows []Message
	err := tx.Where("conversation_id = ? AND id < ?", conversationID, assistantID).Order("id desc").Limit(20).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	attachments, err := attachmentsByMessage(tx, messageIDs(rows))
	if err != nil {
		return nil, err
	}
	turns := make([]ModelTurn, 0, len(rows))
	for _, row := range rows {
		content := row.Content
		if files := attachments[row.ID]; len(files) > 0 {
			var b strings.Builder
			b.WriteString(content)
			for _, file := range files {
				b.WriteString("\n\n【附件 ")
				b.WriteString(file.Name)
				b.WriteString("】\n")
				b.WriteString(file.Content)
			}
			content = b.String()
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		turns = append(turns, ModelTurn{Role: row.Role, Content: content})
	}
	return turns, nil
}

func (r *Repository) withAttachments(ctx context.Context, rows []Message) ([]MessageSimple, error) {
	grouped, err := attachmentsByMessage(r.db.WithContext(ctx), messageIDs(rows))
	if err != nil {
		return nil, err
	}
	data := make([]MessageSimple, 0, len(rows))
	for i := range rows {
		data = append(data, *MessageEntityToSimple(&rows[i], grouped[rows[i].ID]))
	}
	return data, nil
}

func attachmentsByMessage(tx *gorm.DB, ids []int64) (map[int64][]Attachment, error) {
	grouped := map[int64][]Attachment{}
	if len(ids) == 0 {
		return grouped, nil
	}
	var rows []Attachment
	if err := tx.Where("message_id IN ?", ids).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		grouped[row.MessageID] = append(grouped[row.MessageID], row)
	}
	return grouped, nil
}

func messageIDs(rows []Message) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func likePattern(keyword string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(keyword) + "%"
}

func (r *Repository) SaveSources(ctx context.Context, conversationID, messageID int64, sources string) error {
	owner, err := ownerID(ctx)
	if err != nil {
		return err
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&Conversation{}).Where("id = ? AND account_id = ?", conversationID, owner).Count(&count).Error; err != nil {
		return mapDB(err)
	}
	if count == 0 {
		return apperr.NotFound("消息不存在")
	}
	res := r.db.WithContext(ctx).Model(&Message{}).
		Where("id = ? AND conversation_id = ? AND role = ?", messageID, conversationID, "assistant").
		Updates(map[string]any{"sources": sources, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound("消息不存在")
	}
	return nil
}

func (r *Repository) FindSource(ctx context.Context, messageID int64, rawURL string) (search.Hit, error) {
	var zero search.Hit
	message, err := r.ownedAssistant(ctx, messageID)
	if err != nil {
		return zero, err
	}
	rawURL = strings.TrimSpace(rawURL)
	for _, hit := range decodeSources(message.Sources) {
		if hit.URL == rawURL {
			return hit, nil
		}
	}
	return zero, apperr.Invalid("这条结果不在当前回答的检索列表里")
}

func (r *Repository) SaveSourceExcerpt(ctx context.Context, messageID int64, rawURL, excerpt string) error {
	message, err := r.ownedAssistant(ctx, messageID)
	if err != nil {
		return err
	}
	hits := decodeSources(message.Sources)
	found := false
	for i := range hits {
		if hits[i].URL != rawURL {
			continue
		}
		hits[i].Excerpt = excerpt
		found = true
		break
	}
	if !found {
		return apperr.Invalid("这条结果不在当前回答的检索列表里")
	}
	raw, err := json.Marshal(hits)
	if err != nil {
		return apperr.Internal(err)
	}
	res := r.db.WithContext(ctx).Model(&Message{}).Where("id = ?", message.ID).
		Updates(map[string]any{"sources": string(raw), "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return mapDB(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound("消息不存在")
	}
	return nil
}

func (r *Repository) ownedAssistant(ctx context.Context, messageID int64) (Message, error) {
	var message Message
	owner, err := ownerID(ctx)
	if err != nil {
		return message, err
	}
	if messageID <= 0 {
		return message, apperr.Invalid("请选择一条检索结果")
	}
	if err := r.db.WithContext(ctx).Where("id = ? AND role = ?", messageID, "assistant").First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return message, apperr.NotFound("消息不存在")
		}
		return message, mapDB(err)
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&Conversation{}).Where("id = ? AND account_id = ?", message.ConversationID, owner).Count(&count).Error; err != nil {
		return message, mapDB(err)
	}
	if count == 0 {
		return message, apperr.NotFound("消息不存在")
	}
	return message, nil
}

type storedSource struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Excerpt string `json:"excerpt,omitempty"`
	Engine  string `json:"engine,omitempty"`
}

func decodeSources(raw string) []search.Hit {
	var hits []search.Hit
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &hits) != nil {
		return nil
	}
	return hits
}

func mergeSourceExcerpts(stored, incoming string) string {
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return incoming
	}
	var next []storedSource
	if json.Unmarshal([]byte(incoming), &next) != nil {
		return incoming
	}
	var prev []storedSource
	if json.Unmarshal([]byte(stored), &prev) == nil {
		byURL := map[string]string{}
		for _, hit := range prev {
			if hit.Excerpt != "" {
				byURL[hit.URL] = hit.Excerpt
			}
		}
		for i := range next {
			old := byURL[next[i].URL]
			if utf8.RuneCountInString(old) > utf8.RuneCountInString(next[i].Excerpt) {
				next[i].Excerpt = old
			}
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return incoming
	}
	return string(raw)
}

func ownerID(ctx context.Context) (int64, error) {
	scope, ok := domain.ScopeFrom(ctx)
	if !ok || scope.ActorID <= 0 {
		return 0, apperr.Unauthorized("未登录")
	}
	return scope.ActorID, nil
}

func mapDB(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("对话不存在")
	}
	return apperr.Internal(err)
}
