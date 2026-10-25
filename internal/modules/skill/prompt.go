package skill

import "strings"

const ToolName = "load_skill"

const ToolParameters = `{"type":"object","properties":{"name":{"type":"string","description":"技能名称，例如 pdf-parser"},"path":{"type":"string","description":"包内相对路径。留空读取 SKILL.md"}},"required":["name"],"additionalProperties":false}`

func ToolText(entries []Entry) (description, notes string) {
	list := joinEntries(entries, 360)
	if list == "" {
		return "", ""
	}
	description = "读取一个已启用技能的说明，并按说明完成任务。可用技能：" + list + "。path 留空时返回 SKILL.md。"
	notes = "需要技能时调用 load_skill，并遵守它返回的说明，不要编造技能里没有的步骤。可用技能：" + list + "。"
	return description, notes
}

func joinEntries(entries []Entry, budget int) string {
	var b strings.Builder
	for i, item := range entries {
		if i >= 20 {
			b.WriteString("等")
			break
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		piece := name
		if display := strings.TrimSpace(item.DisplayName); display != "" && display != name {
			piece += "，" + clipRunes(display, 20)
		}
		if desc := clipRunes(strings.TrimSpace(item.Description), 36); desc != "" {
			piece += "（" + desc + "）"
		}
		if b.Len() > 0 {
			piece = "；" + piece
		}
		if len([]rune(b.String()+piece)) > budget {
			if b.Len() > 0 {
				b.WriteString("等")
			}
			break
		}
		b.WriteString(piece)
	}
	return b.String()
}

func clipRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
