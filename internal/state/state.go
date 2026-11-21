// Package state records what is installed on this computer. It is never
// synchronized: each computer has its own.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/verify"
)

// Installed is one app installed by Lieferling.
type Installed struct {
	Version     string    `json:"version"`
	Tag         string    `json:"tag"`
	Asset       string    `json:"asset"`
	Path        string    `json:"path"`
	InstalledAt time.Time `json:"installedAt"`
	// Customs is the report of the check this copy passed; nil for copies
	// installed before the check existed.
	Customs *verify.Report `json:"customs,omitempty"`
}

// State maps a repository (lowercase owner/name) to its installation.
type State struct {
	Apps map[string]Installed `json:"apps"`
	// Held maps a repository to the report of a package that was not
	// installed (verdict held or ask). It is replaced by the next attempt and
	// removed by a successful install.
	Held map[string]verify.Report `json:"held,omitempty"`
}

// DefaultPath is LIEFERLING_STATE if set, else Lieferling/state.json in the user's
// configuration directory (on macOS: ~/Library/Application Support).
func DefaultPath() string {
	if p := os.Getenv("LIEFERLING_STATE"); p != "" {
		return p
	}
	if p := os.Getenv("ELAPPMA_STATE"); p != "" { // the old name
		return p
	}
	return pathIn("Lieferling")
}

// LegacyPath is where the state was kept while Lieferling was called Elappma.
func LegacyPath() string { return pathIn("Elappma") }

func pathIn(folder string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	return filepath.Join(dir, folder, "state.json")
}

func key(repo string) string { return strings.ToLower(repo) }

// Get returns the installation of repo.
func (s *State) Get(repo string) (Installed, bool) {
	in, ok := s.Apps[key(repo)]
	return in, ok
}

// Set records an installation.
func (s *State) Set(repo string, in Installed) {
	if s.Apps == nil {
		s.Apps = map[string]Installed{}
	}
	s.Apps[key(repo)] = in
}

// Delete forgets an installation and any package held for it.
func (s *State) Delete(repo string) {
	delete(s.Apps, key(repo))
	s.Release(repo)
}

// Hold records the report of a package that was not installed. The text is
// not stored; it is filled again on the way out.
func (s *State) Hold(repo string, r verify.Report) {
	if s.Held == nil {
		s.Held = map[string]verify.Report{}
	}
	s.Held[key(repo)] = r.Stored()
}

// HeldFor returns the report of the package held for repo.
func (s *State) HeldFor(repo string) (verify.Report, bool) {
	r, ok := s.Held[key(repo)]
	return r, ok
}

// Release forgets the package held for repo.
func (s *State) Release(repo string) { delete(s.Held, key(repo)) }

// Load reads the state; a missing file is an empty state.
func Load(path string) (State, error) {
	var s State
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{Apps: map[string]Installed{}}, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	if s.Apps == nil {
		s.Apps = map[string]Installed{}
	}
	return s, nil
}

// Save writes the state.
func Save(path string, s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return config.WriteFileAtomic(path, append(data, '\n'))
}
