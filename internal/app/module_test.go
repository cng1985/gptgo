package app

import (
	"path/filepath"
	"runtime"
	"testing"

	"gptgo/internal/platform/config"
)

func TestNewBuildsGraph(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位模块根目录")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
	cfg, err := config.Load(config.Options{
		BaseDir: root,
		DataDir: t.TempDir(),
		Profile: "dev",
	}, config.RuntimeInfo{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	application := New(cfg)
	if err := application.Err(); err != nil {
		t.Fatal(err)
	}
}
