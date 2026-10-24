package app

import (
	"flag"
	"fmt"
	"os"

	"gptgo/internal/platform/config"
)

type BuildInfo struct {
	Version   string
	Commit    string
	BuildTime string
}

func Run(info BuildInfo) error {
	fs := flag.NewFlagSet("chatbi", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	baseDir := fs.String("base-dir", "", "应用根目录")
	configDir := fs.String("config-dir", "", "配置目录")
	dataDir := fs.String("data-dir", "", "数据目录")
	profile := fs.String("profile", "", "配置环境：dev 或 pro")
	showVersion := fs.Bool("version", false, "打印版本")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *showVersion {
		fmt.Printf("chatbi %s commit=%s built=%s\n", info.Version, info.Commit, info.BuildTime)
		return nil
	}
	cfg, err := config.Load(config.Options{
		BaseDir:   *baseDir,
		ConfigDir: *configDir,
		DataDir:   *dataDir,
		Profile:   *profile,
	}, config.RuntimeInfo{Version: info.Version, Commit: info.Commit, BuildTime: info.BuildTime})
	if err != nil {
		return err
	}
	application := New(cfg)
	application.Run()
	return application.Err()
}
