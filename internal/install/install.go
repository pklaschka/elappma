// Package install puts a downloaded app in place. Only macOS is supported so
// far; other systems get ErrUnsupported until they have an Installer.
package install

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// ErrUnsupported is returned on systems without an installer yet.
var ErrUnsupported = errors.New("installing is not supported on this system yet")

// Installer installs and removes apps.
type Installer interface {
	// Install installs the downloaded file (its original name is assetName,
	// which tells the format) and returns where the app now is and its name.
	Install(ctx context.Context, file, assetName string) (Result, error)
	// Uninstall removes an app installed at path.
	Uninstall(ctx context.Context, path string) error
	// InstalledVersion reads the version of an app called name that is
	// installed without Elappma, or "" when there is none.
	InstalledVersion(name string) (version, path string)
}

// Result describes an installed app.
type Result struct {
	Path    string // such as /Applications/Feldspur.app
	AppName string // such as Feldspur
}

// Options shared by the installers.
type Options struct {
	// Dir is where apps go; on macOS /Applications unless ELAPPMA_APPLICATIONS
	// is set.
	Dir string
	// Interactive allows asking for an administrator password with sudo in
	// the terminal; otherwise the system's password dialog is used.
	Interactive bool
}

// DefaultOptions uses ELAPPMA_APPLICATIONS and treats a terminal on stdin as
// interactive.
func DefaultOptions() Options {
	o := Options{Dir: os.Getenv("ELAPPMA_APPLICATIONS")}
	if st, err := os.Stdin.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
		o.Interactive = true
	}
	return o
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return errors.New(name + ": " + msg)
		}
		return err
	}
	return nil
}

// shellQuote quotes s for /bin/sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
