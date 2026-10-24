package toolkit

import (
	"strings"
	"testing"
)

func TestCalculator(t *testing.T) {
	ok, text := runCalculator(`{"expression":"(12+3)*4"}`)
	if !ok || text != "60" {
		t.Fatalf("60: %v %s", ok, text)
	}
	ok, text = runCalculator(`{"expression":"1/2"}`)
	if !ok || text != "0.5" {
		t.Fatalf("0.5: %v %s", ok, text)
	}
	ok, text = runCalculator(`{"expression":"1/0"}`)
	if ok || !strings.Contains(text, "0") {
		t.Fatalf("除零应失败: %v %s", ok, text)
	}
	ok, text = runCalculator(`{"expression":"2+alert(1)"}`)
	if ok {
		t.Fatalf("非法算式不应通过: %s", text)
	}
}

func TestCurrentTime(t *testing.T) {
	ok, text := runCurrentTime(`{"timezone":"Asia/Shanghai"}`)
	if !ok || !strings.Contains(text, "Asia/Shanghai") || !strings.Contains(text, "星期") {
		t.Fatalf("时间: %v %s", ok, text)
	}
	ok, text = runCurrentTime(`{"timezone":"Not/AZone"}`)
	if ok {
		t.Fatalf("无效时区应失败: %s", text)
	}
}

func TestPublicMCPName(t *testing.T) {
	name := publicMCPName(12, 0, "get weather")
	if name != "m12n0_get_weather" {
		t.Fatal(name)
	}
	id, ok := mcpServerID(name)
	if !ok || id != 12 {
		t.Fatalf("id=%d ok=%v", id, ok)
	}
	if !reservedToolName("calculator") || !reservedToolName(name) {
		t.Fatal("内置名和 MCP 名应保留")
	}
}
