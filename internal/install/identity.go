package install

import (
	"context"
	"regexp"
	"strings"
)

// Identity is what an app bundle says about itself.
type Identity struct {
	BundleID string // CFBundleIdentifier
	TeamID   string // Developer ID team, only when codesign confirms it; else ""
}

// Unpacked is an app taken out of its download, before anything is replaced.
type Unpacked struct {
	Path     string // the .app in the mounted DMG or the unzipped folder
	Name     string // bundle name without .app
	Identity Identity
}

// Inspect looks at an unpacked app. An error stops the install before the
// running app is quit or anything is replaced.
type Inspect func(ctx context.Context, u Unpacked) error

var teamPattern = regexp.MustCompile(`^[A-Z0-9]{10}$`)

// parseTeamID finds "TeamIdentifier=ABCDE12345" in codesign's output. "not
// set" and anything that is not ten characters of A-Z and 0-9 give "". The
// check also makes the team safe to put into a code requirement.
func parseTeamID(codesignOutput string) string {
	for _, line := range strings.Split(codesignOutput, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "TeamIdentifier="); ok {
			v = strings.TrimSpace(v)
			if teamPattern.MatchString(v) {
				return v
			}
			return ""
		}
	}
	return ""
}
