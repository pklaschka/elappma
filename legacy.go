package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// retireElappma moves the copy of this app from before the rename
// (Elappma.app) to the Bin once Lieferling runs: an Elappma that updated
// itself installs Lieferling next to it and keeps running. It is quit first
// and only moved when its bundle identifier says it is really the old app.
func retireElappma(dir string) {
	if runtime.GOOS != "darwin" {
		return
	}
	old := filepath.Join(dir, "Elappma.app")
	out, err := exec.Command("plutil", "-extract", "CFBundleIdentifier", "raw",
		filepath.Join(old, "Contents", "Info.plist")).Output()
	if err != nil || strings.TrimSpace(string(out)) != "de.pklaschka.elappma" {
		return
	}
	_ = exec.Command("osascript", "-e", `tell application id "de.pklaschka.elappma" to quit`).Run()
	for i := 0; i < 20 && exec.Command("pgrep", "-xq", "Elappma").Run() == nil; i++ {
		time.Sleep(500 * time.Millisecond)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	bin := filepath.Join(home, ".Trash", "Elappma.app")
	for i := 2; exists(bin); i++ {
		bin = filepath.Join(home, ".Trash", fmt.Sprintf("Elappma %d.app", i))
	}
	_ = os.Rename(old, bin)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
