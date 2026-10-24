package mcp

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"gptgo/internal/modules/agent"
	"gptgo/internal/modules/chat"
	aiplat "gptgo/internal/platform/ai"
)

var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type Runner struct {
	agents        *agent.Repository
	conversations *chat.Repository
	servers       *Repository
}

func NewRunner(agents *agent.Repository, conversations *chat.Repository, servers *Repository) *Runner {
	return &Runner{agents: agents, conversations: conversations, servers: servers}
}

func (r *Runner) ForAgent(ctx context.Context, agentID int64) ([]aiplat.Tool, error) {
	if r == nil || agentID <= 0 {
		return nil, nil
	}
	one, err := r.agents.One(ctx, &agent.AgentRequest{ID: agentID})
	if err != nil {
		return nil, err
	}
	tools := make([]aiplat.Tool, 0, len(one.Data.Tools))
	for _, name := range one.Data.Tools {
		if tool, ok := builtinByName(name); ok {
			tools = append(tools, tool)
		}
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		seen[tool.Name] = true
	}
	for _, id := range one.Data.McpIDs {
		entity, headers, err := r.servers.Open(ctx, int64(id))
		if err != nil || !entity.Enabled {
			continue
		}
		listed, err := probeTools(ctx, entity, headers)
		if err != nil {
			continue
		}
		for _, remote := range listed {
			name := advertise(entity.ID, remote.Name)
			if name == "" || seen[name] || !toolNamePattern.MatchString(name) {
				continue
			}
			seen[name] = true
			schema := remote.InputSchema
			if len(schema) == 0 || !json.Valid(schema) {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			tools = append(tools, aiplat.Tool{
				Name:        name,
				Description: clipText(remote.Description, 300),
				Parameters:  schema,
				Kind:        "mcp",
				ServerID:    entity.ID,
				RemoteName:  remote.Name,
			})
		}
	}
	return tools, nil
}

func (r *Runner) Call(ctx context.Context, tool aiplat.Tool, arguments string) (string, error) {
	switch tool.Kind {
	case "builtin":
		return r.callBuiltin(ctx, tool.Name, arguments)
	case "mcp":
		return r.callMCP(ctx, tool, arguments)
	default:
		return "未知工具", nil
	}
}

func (r *Runner) callBuiltin(ctx context.Context, name, arguments string) (string, error) {
	switch name {
	case "clock":
		return ClockText(arguments, clockNow()), nil
	case "ask_catalog":
		question := argumentString(arguments, "question")
		if strings.TrimSpace(question) == "" {
			return "问题不能为空", nil
		}
		outcome, matched, err := r.conversations.Ask(ctx, question, "off")
		if err != nil {
			return "问数暂时无法完成", err
		}
		if !matched {
			return "没有识别成问数。可以问某张表的条数、字段，或让系统把全部数据表画成图表。", nil
		}
		payload := map[string]any{"answer": outcome.Answer}
		if outcome.Chart != nil {
			payload["chart_type"] = outcome.Chart.Type
			payload["title"] = outcome.Chart.Title
			payload["columns"] = outcome.Chart.Columns
			payload["rows"] = outcome.Chart.Rows
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return outcome.Answer, nil
		}
		return string(raw), nil
	default:
		return "未知工具", nil
	}
}

func (r *Runner) callMCP(ctx context.Context, tool aiplat.Tool, arguments string) (string, error) {
	entity, headers, err := r.servers.Open(ctx, tool.ServerID)
	if err != nil {
		return "MCP 服务不存在", err
	}
	if !entity.Enabled {
		return "MCP 服务未启用", errDisabled()
	}
	session, closeFn, err := openSession(ctx, entity, headers)
	if err != nil {
		return "无法连接 MCP 服务", err
	}
	defer closeFn()
	if err := session.Initialize(ctx); err != nil {
		return "MCP 服务初始化失败", err
	}
	text, ok, err := session.CallTool(ctx, tool.RemoteName, arguments)
	if err != nil {
		return "MCP 工具调用失败", err
	}
	if !ok {
		if text == "" {
			text = "MCP 工具返回错误"
		}
		return text, errDisabled()
	}
	if strings.TrimSpace(text) == "" {
		return "工具没有返回内容", nil
	}
	return text, nil
}

func advertise(serverID int64, remote string) string {
	var b strings.Builder
	b.WriteString("m")
	b.WriteString(itoa(serverID))
	b.WriteByte('_')
	for _, r := range remote {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	name := b.String()
	if len(name) > 64 {
		name = name[:64]
	}
	if !toolNamePattern.MatchString(name) {
		return ""
	}
	return name
}

func argumentString(arguments, key string) string {
	var parsed map[string]any
	if json.Unmarshal([]byte(arguments), &parsed) != nil {
		return ""
	}
	value, _ := parsed[key].(string)
	return value
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

type disabledError struct{}

func (disabledError) Error() string { return "工具调用失败" }

func errDisabled() error { return disabledError{} }
