// Package install puts a downloaded app in place. Only macOS is supported so
// far; other systems get ErrUnsupported until they have an Installer.
package install

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	// installed without Lieferling, or "" when there is none.
	InstalledVersion(name string) (version, path string)
}

// Result describes an installed app.
type Result struct {
	Path    string // such as /Applications/Feldspur.app
	AppName string // such as Feldspur
	Self    bool   // the running app replaced itself and should restart
}

// Options shared by the installers.
type Options struct {
	// Dir is where apps go; on macOS /Applications unless LIEFERLING_APPLICATIONS
	// is set.
	Dir string
	// Interactive allows asking for an administrator password with sudo in
	// the terminal; otherwise the system's password dialog is used.
	Interactive bool
	// Self is the app bundle this process runs from (see SelfBundle), or
	// "". Installing over it replaces the bundle without quitting first;
	// the caller then restarts.
	Self string
}

// ErrSelf is returned when asked to uninstall the running app itself.
var ErrSelf = errors.New("Lieferling cannot remove itself while it runs: quit it and move it to the Bin")

// SelfBundle returns the .app bundle the running executable belongs to, or
// "" when it does not run from one (the command line, go run).
func SelfBundle() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	i := strings.Index(exe, ".app/Contents/MacOS/")
	if i < 0 {
		return ""
	}
	return exe[:i+len(".app")]
}

// SamePath reports whether a and b name the same file, after symlinks.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	clean := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		return filepath.Clean(p)
	}
	return clean(a) == clean(b)
}

// DefaultOptions uses LIEFERLING_APPLICATIONS and treats a terminal on stdin as
// interactive.
func DefaultOptions() Options {
	o := Options{Dir: os.Getenv("LIEFERLING_APPLICATIONS")}
	if o.Dir == "" {
		o.Dir = os.Getenv("ELAPPMA_APPLICATIONS") // the old name
	}
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
