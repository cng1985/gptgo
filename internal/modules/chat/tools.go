package chat

import (
	"context"
	"encoding/json"

	"gptgo/internal/modules/skill"
	"gptgo/internal/modules/toolkit"
	aiplat "gptgo/internal/platform/ai"
)

type toolSource struct {
	tools  *toolkit.Resource
	skills *skill.Resource
}

func NewToolSource(tools *toolkit.Resource, skills *skill.Resource) ToolSource {
	return &toolSource{tools: tools, skills: skills}
}

func (s *toolSource) Catalog(ctx context.Context) ([]aiplat.Tool, error) {
	var listed []aiplat.Tool
	if s != nil && s.tools != nil {
		var err error
		listed, err = s.tools.Catalog(ctx)
		if err != nil {
			return nil, err
		}
	}
	if s == nil || s.skills == nil {
		return listed, nil
	}
	entries, err := s.skills.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	description, _ := skill.ToolText(entries)
	if description == "" {
		return listed, nil
	}
	if len(listed) >= 32 {
		listed = listed[:31]
	}
	return append(listed, aiplat.Tool{
		Name:        skill.ToolName,
		Description: description,
		Parameters:  json.RawMessage(skill.ToolParameters),
	}), nil
}

func (s *toolSource) Invoke(ctx context.Context, name, arguments string) (bool, string, error) {
	if name == skill.ToolName {
		if s == nil || s.skills == nil {
			return false, "没有这个技能", nil
		}
		return s.skills.Load(ctx, arguments)
	}
	if s == nil || s.tools == nil {
		return false, "没有这个工具", nil
	}
	return s.tools.Invoke(ctx, name, arguments)
}

func (s *toolSource) Notes(ctx context.Context) (string, error) {
	if s == nil || s.skills == nil {
		return "", nil
	}
	entries, err := s.skills.Enabled(ctx)
	if err != nil {
		return "", err
	}
	_, notes := skill.ToolText(entries)
	return notes, nil
}
