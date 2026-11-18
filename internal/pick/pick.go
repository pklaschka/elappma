// Package pick chooses the release for a channel and the asset of that
// release that fits this computer.
package pick

import (
	"path"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/pklaschka/lieferling/internal/github"
)

// Channels.
const (
	Stable     = "stable"
	Prerelease = "prerelease" // stable releases and release candidates
)

// Release returns the newest release of the channel. GitHub lists releases
// newest first; drafts are never picked.
func Release(releases []github.Release, channel string) (github.Release, bool) {
	for _, r := range releases {
		if r.Draft {
			continue
		}
		if channel != Prerelease && r.Prerelease {
			continue
		}
		return r, true
	}
	return github.Release{}, false
}

// Version turns a tag into a version: "v1.2.3" and "1.2.3" both give "1.2.3".
func Version(tag string) string {
	return strings.TrimPrefix(strings.TrimPrefix(tag, "v"), "V")
}

// Newer reports whether version a is newer than b. Versions that are not
// semantic versions count as newer whenever they differ.
func Newer(a, b string) bool {
	if a == "" || a == b {
		return false
	}
	if b == "" {
		return true
	}
	va, vb := "v"+Version(a), "v"+Version(b)
	if semver.IsValid(va) && semver.IsValid(vb) {
		return semver.Compare(va, vb) > 0
	}
	return true
}

// Asset returns the asset to install on goos/goarch, or false when none fits.
func Asset(assets []github.Asset, goos, goarch string) (github.Asset, bool) {
	best, bestScore := github.Asset{}, 0
	for _, a := range assets {
		if s := score(a.Name, goos, goarch); s > bestScore {
			best, bestScore = a, s
		}
	}
	return best, bestScore > 0
}

var osTokens = map[string][]string{
	"darwin":  {"mac", "macos", "osx", "darwin", "apple"},
	"windows": {"win", "windows", "win32", "win64"},
	"linux":   {"linux"},
}

var archTokens = map[string][]string{
	"arm64": {"arm64", "aarch64", "apple-silicon"},
	"amd64": {"x64", "x86_64", "amd64", "intel", "x86-64"},
}

// formats lists the installable file types per OS with their preference.
var formats = map[string]map[string]int{
	"darwin":  {".dmg": 30, ".zip": 20},
	"linux":   {".appimage": 30},
	"windows": {".exe": 30, ".msi": 20},
}

func score(name, goos, goarch string) int {
	lower := strings.ToLower(name)
	ext := path.Ext(lower)
	base, ok := formats[goos][ext]
	if !ok {
		return 0
	}
	tokens := tokenize(strings.TrimSuffix(lower, ext))
	has := func(list []string) bool {
		for _, t := range list {
			if tokens[t] {
				return true
			}
		}
		return false
	}

	// another OS named in the file name rules it out
	for os, list := range osTokens {
		if os != goos && has(list) {
			return 0
		}
	}
	// a bare .zip is only taken when it names this OS (it might be anything)
	if ext == ".zip" && !has(osTokens[goos]) {
		return 0
	}
	// Windows installers come as Setup.exe; skip uninstallers and helpers
	if goos == "windows" && (tokens["uninstall"] || tokens["uninstaller"]) {
		return 0
	}

	s := base
	if has(osTokens[goos]) {
		s += 2
	}
	switch {
	case has(archTokens[goarch]):
		s += 10
	case tokens["universal"]:
		s += 8
	case hasOtherArch(tokens, goarch):
		// Apple Silicon runs Intel builds through Rosetta: last resort
		if goos == "darwin" && goarch == "arm64" {
			s = 1
		} else {
			return 0
		}
	default:
		s += 5 // no architecture named: often the only build
	}
	return s
}

func hasOtherArch(tokens map[string]bool, goarch string) bool {
	for arch, list := range archTokens {
		if arch == goarch {
			continue
		}
		for _, t := range list {
			if tokens[t] {
				return true
			}
		}
	}
	return false
}

// tokenize splits a file name into its words ("Foo-1.2.0-arm64" gives foo,
// 1, 2, 0, arm64) and also keeps joined pairs such as "x86_64" and
// "apple-silicon".
func tokenize(s string) map[string]bool {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == ' ' || r == '(' || r == ')'
	})
	tokens := make(map[string]bool, len(words)*2)
	for i, w := range words {
		tokens[w] = true
		if i > 0 {
			tokens[words[i-1]+"_"+w] = true
			tokens[words[i-1]+"-"+w] = true
		}
	}
	return tokens
}
