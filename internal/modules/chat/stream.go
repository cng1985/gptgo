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
	turns := make([]aiplat.Message, 0, len(data.Messages))
	for _, turn := range data.Messages {
		turns = append(turns, aiplat.Message{Role: turn.Role, Content: turn.Content})
	}
	var tools []aiplat.Tool
	if data.AgentID > 0 && resource.tools != nil {
		listed, listErr := resource.tools.ForAgent(streamCtx, data.AgentID)
		if listErr == nil {
			tools = listed
		}
	}
	traces := make([]ToolTrace, 0)
	for round := 0; round < 3; round++ {
		var roundText strings.Builder
		var calls []aiplat.ToolCall
		chunks, err := models.Stream(streamCtx, aiplat.Request{
			Model:        data.Model,
			Messages:     turns,
			ThinkingMode: data.ThinkingMode,
			SearchNotes:  data.SearchNotes,
			AgentName:    data.AgentName,
			Instructions: data.Instructions,
			Tools:        tools,
		})
		if err != nil {
			status = "error"
			write("error", map[string]string{"msg": "模型暂时无法回答，请稍后再试"})
			break
		}
		failed := false
		for delta := range chunks {
			if delta.Err != nil {
				status = "error"
				write("error", map[string]string{"msg": "模型暂时无法回答，请稍后再试"})
				failed = true
				break
			}
			if delta.Thinking != "" {
				thinking.WriteString(delta.Thinking)
				write("thinking", map[string]string{"delta": delta.Thinking})
			}
			if delta.Content != "" {
				content.WriteString(delta.Content)
				roundText.WriteString(delta.Content)
				write("delta", map[string]string{"delta": delta.Content})
			}
			if len(delta.ToolCalls) > 0 {
				calls = append(calls, delta.ToolCalls...)
			}
			if streamCtx.Err() != nil {
				status = "stopped"
				failed = true
				break
			}
		}
		if errors.Is(streamCtx.Err(), context.Canceled) || errors.Is(streamCtx.Err(), context.DeadlineExceeded) {
			status = "stopped"
			failed = true
		}
		if failed || len(calls) == 0 || resource.tools == nil {
			break
		}
		if len(calls) > 3 {
			calls = calls[:3]
		}
		turns = append(turns, aiplat.Message{Role: "assistant", Content: roundText.String(), ToolCalls: calls})
		for _, call := range calls {
			result, ok := runTool(streamCtx, resource, tools, call)
			trace := ToolTrace{
				Name:      call.Name,
				Arguments: clipRunes(call.Arguments, 2000),
				Result:    clipRunes(result, 4000),
				OK:        ok,
			}
			traces = append(traces, trace)
			write("tool", trace)
			turns = append(turns, aiplat.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    clipRunes(result, 8000),
			})
		}
	}
	if status == "error" && content.Len() == 0 {
		content.WriteString("这次没有生成完整回答。请再试一次。")
	}
	toolsJSON := ""
	if len(traces) > 0 {
		if raw, err := json.Marshal(traces); err == nil && len(raw) <= 20000 {
			toolsJSON = string(raw)
		}
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
		Tools:              toolsJSON,
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

func runTool(ctx context.Context, resource *Resource, tools []aiplat.Tool, call aiplat.ToolCall) (string, bool) {
	var matched aiplat.Tool
	found := false
	for _, tool := range tools {
		if tool.Name == call.Name {
			matched = tool
			found = true
			break
		}
	}
	if !found {
		return "未知工具", false
	}
	result, err := resource.tools.Call(ctx, matched, call.Arguments)
	if err != nil {
		if strings.TrimSpace(result) == "" {
			return "工具调用失败", false
		}
		return result, false
	}
	return result, true
}
