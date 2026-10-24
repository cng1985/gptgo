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

type ToolSource interface {
	Catalog(ctx context.Context) ([]aiplat.Tool, error)
	Invoke(ctx context.Context, name, arguments string) (bool, string, error)
}

const maxToolRounds = 3

type toolTrace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	OK        bool   `json:"ok"`
	Result    string `json:"result"`
	Status    string `json:"status"`
}

func ServeStream(c *gin.Context, resource *Resource, models Streamer, tools ToolSource) {
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
	var catalog []aiplat.Tool
	if tools != nil {
		listed, err := tools.Catalog(streamCtx)
		if err == nil {
			catalog = listed
		}
	}
	var traces []toolTrace
	rounds := 0
	for {
		if streamCtx.Err() != nil {
			status = "stopped"
			break
		}
		req := aiplat.Request{
			Model:        data.Model,
			Messages:     turns,
			ThinkingMode: data.ThinkingMode,
			SearchNotes:  data.SearchNotes,
			AgentName:    data.AgentName,
			Instructions: data.Instructions,
			Tools:        catalog,
		}
		if rounds >= maxToolRounds {
			req.Tools = nil
		}
		chunks, err := models.Stream(streamCtx, req)
		if err != nil && len(req.Tools) > 0 && rounds == 0 {
			catalog = nil
			req.Tools = nil
			chunks, err = models.Stream(streamCtx, req)
		}
		if err != nil {
			status = "error"
			write("error", map[string]string{"msg": "模型暂时无法回答，请稍后再试"})
			break
		}
		var roundContent strings.Builder
		var roundCalls []aiplat.ToolCall
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
				roundContent.WriteString(delta.Content)
				write("delta", map[string]string{"delta": delta.Content})
			}
			if len(delta.ToolCalls) > 0 {
				roundCalls = append(roundCalls, delta.ToolCalls...)
			}
			if streamCtx.Err() != nil {
				status = "stopped"
				break
			}
		}
		if status == "stopped" || failed {
			break
		}
		if len(roundCalls) == 0 || tools == nil || rounds >= maxToolRounds {
			break
		}
		turns = append(turns, aiplat.Message{Role: "assistant", Content: roundContent.String(), ToolCalls: roundCalls})
		rounds++
		for _, call := range roundCalls {
			if streamCtx.Err() != nil {
				status = "stopped"
				break
			}
			arguments := clipRunes(call.Arguments, 1500)
			trace := toolTrace{ID: call.ID, Name: call.Name, Arguments: arguments, Status: "running"}
			write("tool", trace)
			ok, output, invokeErr := tools.Invoke(streamCtx, call.Name, call.Arguments)
			if invokeErr != nil {
				if streamCtx.Err() != nil {
					status = "stopped"
					break
				}
				ok = false
				if strings.TrimSpace(output) == "" {
					output = "工具没有返回结果"
				}
			}
			output = clipRunes(output, 4000)
			trace.OK = ok
			trace.Result = output
			trace.Status = "done"
			traces = append(traces, trace)
			write("tool", trace)
			turns = append(turns, aiplat.Message{
				Role: "tool", ToolCallID: call.ID, Name: call.Name, Content: output,
			})
		}
		if status == "stopped" {
			break
		}
	}
	if errors.Is(streamCtx.Err(), context.Canceled) || errors.Is(streamCtx.Err(), context.DeadlineExceeded) {
		status = "stopped"
	}
	if content.Len() == 0 && len(traces) > 0 {
		var summary strings.Builder
		summary.WriteString("工具已经返回结果。\n\n")
		for _, trace := range traces {
			summary.WriteString(trace.Name)
			summary.WriteString("：")
			summary.WriteString(trace.Result)
			summary.WriteString("\n")
		}
		text := summary.String()
		content.WriteString(text)
		write("delta", map[string]string{"delta": text})
	}
	if status == "error" && content.Len() == 0 {
		content.WriteString("这次没有生成完整回答。请再试一次。")
	}
	toolsJSON := ""
	if len(traces) > 0 {
		if raw, err := json.Marshal(traces); err == nil {
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
