//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// reopenAfterExit starts a small shell that waits for this process to end and
// then opens bundle, the freshly installed copy of Elappma.
func reopenAfterExit(bundle string) error {
	q := "'" + strings.ReplaceAll(bundle, "'", `'\''`) + "'"
	cmd := exec.Command("/bin/sh", "-c",
		fmt.Sprintf("while kill -0 %d 2>/dev/null; do sleep 0.2; done; open %s", os.Getpid(), q))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
