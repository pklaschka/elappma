package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CommandPath is where the app links the command line: /usr/local/bin is on
// the PATH of every macOS shell.
const CommandPath = "/usr/local/bin/lieferling"

// ErrNoCommand is returned when the running app carries no command line
// (a development build).
var ErrNoCommand = errors.New("this copy of Lieferling carries no command line")

// ErrCanceled is returned when the user closed the password dialog.
var ErrCanceled = errors.New("canceled")

// CommandState says what is at CommandPath.
type CommandState string

const (
	CommandMissing     CommandState = "missing"     // nothing there yet
	CommandLinked      CommandState = "linked"      // a link to this app's command line
	CommandOther       CommandState = "other"       // a copy from a release download, or a link elsewhere
	CommandUnavailable CommandState = "unavailable" // this app carries no command line
)

// Command describes the command line and its place on the PATH.
type Command struct {
	State CommandState `json:"state"`
	// Path is where the link goes, such as /usr/local/bin/lieferling.
	Path string `json:"path"`
	// Bundled is the command line inside the app bundle, or "".
	Bundled string `json:"bundled"`
	// Other is where a foreign link at Path points, or "" for a plain file.
	Other string `json:"other"`
	// Manual is the same setup as a command for the Terminal, for when the
	// password dialog does not work out; "" without a bundled command line.
	Manual string `json:"manual"`
}

// BundledCommand returns the command line in the app bundle (see
// SelfBundle), Contents/Helpers/lieferling, or "" when it has none.
func BundledCommand(bundle string) string {
	if bundle == "" {
		return ""
	}
	p := filepath.Join(bundle, "Contents", "Helpers", "lieferling")
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return ""
	}
	return p
}

// CommandStatus looks at path and compares it with the bundled command line.
func CommandStatus(bundled, path string) Command {
	c := Command{Path: path, Bundled: bundled, Manual: ManualCommand(bundled, path)}
	st, err := os.Lstat(path)
	switch {
	case err != nil:
		c.State = CommandMissing
	case SamePath(path, bundled):
		c.State = CommandLinked
	default:
		c.State = CommandOther
		if st.Mode()&os.ModeSymlink != 0 {
			c.Other, _ = os.Readlink(path)
		}
	}
	// without a command line of its own the app can only remove its link
	if bundled == "" && c.State != CommandLinked {
		c.State = CommandUnavailable
	}
	return c
}

// ManualCommand returns the shell command that does what LinkCommand does,
// with sudo, or "" when there is nothing to link.
func ManualCommand(bundled, path string) string {
	if bundled == "" {
		return ""
	}
	return fmt.Sprintf("sudo mkdir -p %s && sudo ln -sf %s %s",
		shellQuote(filepath.Dir(path)), shellQuote(bundled), shellQuote(path))
}
