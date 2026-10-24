package mcp

import (
	"encoding/json"
	"strings"
	"time"

	aiplat "gptgo/internal/platform/ai"
)

func BuiltinTools() []aiplat.Tool {
	return []aiplat.Tool{clockTool(), askTool()}
}

func BuiltinInfo() []BuiltinTool {
	return []BuiltinTool{
		{Name: "clock", Label: "当前时间", Description: "读取指定时区的当前日期和时间"},
		{Name: "ask_catalog", Label: "本库问数", Description: "查询当前应用库的数据表、字段和条数"},
	}
}

func builtinByName(name string) (aiplat.Tool, bool) {
	switch name {
	case "clock":
		return clockTool(), true
	case "ask_catalog":
		return askTool(), true
	default:
		return aiplat.Tool{}, false
	}
}

func clockTool() aiplat.Tool {
	return aiplat.Tool{
		Name:        "clock",
		Description: "读取当前日期和时间。timezone 使用 IANA 时区名，不填则用 Asia/Shanghai。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"timezone":{"type":"string","description":"IANA 时区，默认 Asia/Shanghai"}}}`),
		Kind:        "builtin",
	}
}

func askTool() aiplat.Tool {
	return aiplat.Tool{
		Name:        "ask_catalog",
		Description: "查询当前应用库有哪些表、字段和各表条数。question 是用户的问数问题。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"question":{"type":"string"}},"required":["question"]}`),
		Kind:        "builtin",
	}
}

func ClockText(arguments string, now time.Time) string {
	tz := "Asia/Shanghai"
	var parsed map[string]any
	if json.Unmarshal([]byte(arguments), &parsed) == nil {
		if value, ok := parsed["timezone"].(string); ok && strings.TrimSpace(value) != "" {
			tz = strings.TrimSpace(value)
		}
	}
	loc, err := time.LoadLocation(tz)
	label := tz
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
		label = "CST"
	}
	return now.In(loc).Format("2006-01-02 15:04:05 Monday") + " " + label
}
