//go:build windows

package skill

import (
	"os/exec"
	"time"
)

func prepareCommand(cmd *exec.Cmd) {
	cmd.WaitDelay = 200 * time.Millisecond
}
