package main

import (
	"fmt"
	"os"

	"gptgo/internal/app"
)

var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

func main() {
	if err := app.Run(app.BuildInfo{Version: version, Commit: commit, BuildTime: buildTime}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
