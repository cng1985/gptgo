package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	aiplat "gptgo/internal/platform/ai"
	"gptgo/internal/platform/domain"
	"gptgo/internal/platform/response"
)

type Streamer interface {
	Stream(ctx context.Context, req aiplat.Request) (<-chan aiplat.Delta, error)
}

func ServeStream(c *gin.Context, resource *Resource, models Streamer) {
	var request ChatStreamRequest
	if err := response.BindJSON(c, &request); err != nil {
		response.HandleError(c, err)
		return
	}
	prepared, err := resource.Prepare(c.Request.Context(), &request)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	var mu sync.Mutex
	write := func(event string, payload any) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, raw)
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	stopHeartbeat := make(chan struct{})
	defer close(stopHeartbeat)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ticker.C:
				mu.Lock()
				fmt.Fprint(c.Writer, ": ping\n\n")
				if flusher, ok := c.Writer.(http.Flusher); ok {
					flusher.Flush()
				}
				mu.Unlock()
			}
		}
	}()

	data := prepared.Data
	write("meta", map[string]string{
		"conversation_id":      formatID(data.ConversationID),
		"user_message_id":      formatID(data.UserMessageID),
		"assistant_message_id": formatID(data.AssistantMessageID),
		"title":                data.Title,
	})

	streamCtx := c.Request.Context()
	var content strings.Builder
	var thinking strings.Builder
	status := "done"
	chartJSON := ""
	outcome, asked, askErr := resource.Ask(streamCtx, lastQuestion(data.Messages), data.ThinkingMode)
	switch {
	case errors.Is(askErr, context.Canceled) || errors.Is(askErr, context.DeadlineExceeded):
		status = "stopped"
	case asked && askErr != nil:
		status = "error"
		write("error", map[string]string{"msg": "问数暂时无法完成，请稍后再试"})
	case asked:
		if outcome.Thinking != "" {
			thinking.WriteString(outcome.Thinking)
			writeText(streamCtx, write, "thinking", outcome.Thinking)
		}
		if outcome.Chart != nil {
			write("chart", outcome.Chart)
			if raw, err := json.Marshal(outcome.Chart); err == nil {
				chartJSON = string(raw)
			}
		}
		content.WriteString(outcome.Answer)
		writeText(streamCtx, write, "delta", outcome.Answer)
		if streamCtx.Err() != nil {
			status = "stopped"
		}
	default:
		turns := make([]aiplat.Message, 0, len(data.Messages))
		for _, turn := range data.Messages {
			turns = append(turns, aiplat.Message{Role: turn.Role, Content: turn.Content})
		}
		chunks, err := models.Stream(streamCtx, aiplat.Request{
			Model:        data.Model,
			Messages:     turns,
			ThinkingMode: data.ThinkingMode,
			SearchNotes:  data.SearchNotes,
		})
		if err != nil {
			status = "error"
			write("error", map[string]string{"msg": "模型暂时无法回答，请稍后再试"})
		} else {
			for delta := range chunks {
				if delta.Err != nil {
					status = "error"
					write("error", map[string]string{"msg": "模型暂时无法回答，请稍后再试"})
					break
				}
				if delta.Thinking != "" {
					thinking.WriteString(delta.Thinking)
					write("thinking", map[string]string{"delta": delta.Thinking})
				}
				if delta.Content != "" {
					content.WriteString(delta.Content)
					write("delta", map[string]string{"delta": delta.Content})
				}
				if streamCtx.Err() != nil {
					status = "stopped"
					break
				}
			}
			if errors.Is(streamCtx.Err(), context.Canceled) || errors.Is(streamCtx.Err(), context.DeadlineExceeded) {
				status = "stopped"
			}
		}
	}
	if status == "error" && content.Len() == 0 {
		content.WriteString("这次没有生成完整回答。请再试一次。")
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
	defer cancel()
	if _, ok := domain.ScopeFrom(saveCtx); !ok {
		if scope, ok := domain.ScopeFrom(c.Request.Context()); ok {
			saveCtx = domain.WithScope(saveCtx, scope)
		}
	}
	_, _ = resource.Complete(saveCtx, &ChatCompleteRequest{
		ConversationID:     data.ConversationID,
		AssistantMessageID: data.AssistantMessageID,
		Content:            content.String(),
		Thinking:           thinking.String(),
		Chart:              chartJSON,
		Status:             status,
	})
	if status == "done" {
		write("done", map[string]string{
			"conversation_id": formatID(data.ConversationID),
			"message_id":      formatID(data.AssistantMessageID),
			"status":          status,
		})
	} else if status == "stopped" {
		write("done", map[string]string{
			"conversation_id": formatID(data.ConversationID),
			"message_id":      formatID(data.AssistantMessageID),
			"status":          status,
		})
	}
}

func formatID(id int64) string {
	return fmt.Sprintf("%d", id)
}

func lastQuestion(messages []ModelTurn) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		text := messages[i].Content
		if idx := strings.Index(text, "\n\n【附件 "); idx >= 0 {
			text = text[:idx]
		}
		return strings.TrimSpace(text)
	}
	return ""
}

func writeText(ctx context.Context, write func(string, any), event, text string) {
	runes := []rune(text)
	for i := 0; i < len(runes); i += 32 {
		if ctx.Err() != nil {
			return
		}
		end := i + 32
		if end > len(runes) {
			end = len(runes)
		}
		write(event, map[string]string{"delta": string(runes[i:end])})
	}
}
