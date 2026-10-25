package chat

import (
	"context"

	aiplat "gptgo/internal/platform/ai"
)

const (
	maxCatalogTools = 32
	maxSkillTools   = 12
)

type skillSource interface {
	Catalog(ctx context.Context) ([]aiplat.Tool, error)
	Invoke(ctx context.Context, name, arguments string) (bool, string, error)
	Owns(ctx context.Context, name string) (bool, error)
}

type mergedTools struct {
	base   ToolSource
	skills skillSource
}

func mergeTools(base ToolSource, skills skillSource) ToolSource {
	return mergedTools{base: base, skills: skills}
}

func (m mergedTools) Catalog(ctx context.Context) ([]aiplat.Tool, error) {
	var base []aiplat.Tool
	if m.base != nil {
		listed, err := m.base.Catalog(ctx)
		if err != nil {
			return nil, err
		}
		base = listed
	}
	var extra []aiplat.Tool
	if m.skills != nil {
		listed, err := m.skills.Catalog(ctx)
		if err != nil {
			return nil, err
		}
		extra = listed
	}
	if len(extra) > maxSkillTools {
		extra = extra[:maxSkillTools]
	}
	room := maxCatalogTools - len(extra)
	if room < 0 {
		room = 0
	}
	if len(base) > room {
		base = base[:room]
	}
	seen := make(map[string]int, len(base)+len(extra))
	out := make([]aiplat.Tool, 0, len(base)+len(extra))
	for _, tool := range base {
		seen[tool.Name] = len(out)
		out = append(out, tool)
	}
	for _, tool := range extra {
		if at, ok := seen[tool.Name]; ok {
			out[at] = tool
			continue
		}
		out = append(out, tool)
	}
	return out, nil
}

func (m mergedTools) Invoke(ctx context.Context, name, arguments string) (bool, string, error) {
	if m.skills != nil {
		owned, err := m.skills.Owns(ctx, name)
		if err != nil {
			return false, "", err
		}
		if owned {
			return m.skills.Invoke(ctx, name, arguments)
		}
	}
	if m.base == nil {
		return false, "没有这个工具", nil
	}
	return m.base.Invoke(ctx, name, arguments)
}
