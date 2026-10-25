package chat

import (
	"context"
	"testing"

	aiplat "gptgo/internal/platform/ai"
)

type listedTools struct {
	tools []aiplat.Tool
	hit   string
}

func (l listedTools) Catalog(context.Context) ([]aiplat.Tool, error) {
	return l.tools, nil
}

func (l listedTools) Invoke(context.Context, string, string) (bool, string, error) {
	return true, l.hit, nil
}

type listedSkills struct {
	listedTools
	owned map[string]bool
}

func (l listedSkills) Owns(_ context.Context, name string) (bool, error) {
	return l.owned[name], nil
}

func TestMergedToolsKeepsSkills(t *testing.T) {
	base := make([]aiplat.Tool, 0, 30)
	for i := 0; i < 30; i++ {
		base = append(base, aiplat.Tool{Name: "http_tool", Description: "base"})
		base[i].Name = "tool_" + string(rune('a'+(i%26))) + string(rune('a'+i/26))
	}
	base = append([]aiplat.Tool{{Name: "echo", Description: "来自 HTTP"}}, base...)
	skills := listedSkills{
		listedTools: listedTools{tools: []aiplat.Tool{{Name: "echo", Description: "来自技能"}, {Name: "upper_text", Description: "技能"}}, hit: "skill"},
		owned:       map[string]bool{"echo": true, "upper_text": true},
	}
	merged := mergeTools(listedTools{tools: base, hit: "http"}, skills)
	catalog, err := merged.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) > 32 {
		t.Fatalf("目录过长: %d", len(catalog))
	}
	seen := map[string]string{}
	for _, tool := range catalog {
		seen[tool.Name] = tool.Description
	}
	if seen["upper_text"] != "技能" {
		t.Fatalf("技能没有进入目录: %#v", seen)
	}
	if seen["echo"] != "来自技能" {
		t.Fatalf("同名时应使用技能: %#v", seen["echo"])
	}
	ok, text, err := merged.Invoke(context.Background(), "echo", "{}")
	if err != nil || !ok || text != "skill" {
		t.Fatalf("同名调用应走技能: %v %v %s", err, ok, text)
	}
	ok, text, err = merged.Invoke(context.Background(), "calculator", "{}")
	if err != nil || !ok || text != "http" {
		t.Fatalf("其他工具应走原来的目录: %v %v %s", err, ok, text)
	}
}
