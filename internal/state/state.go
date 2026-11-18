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

	"github.com/pklaschka/elappma/internal/config"
)

// Installed is one app installed by Elappma.
type Installed struct {
	Version     string    `json:"version"`
	Tag         string    `json:"tag"`
	Asset       string    `json:"asset"`
	Path        string    `json:"path"`
	InstalledAt time.Time `json:"installedAt"`
}

// State maps a repository (lowercase owner/name) to its installation.
type State struct {
	Apps map[string]Installed `json:"apps"`
}

// DefaultPath is ELAPPMA_STATE if set, else Elappma/state.json in the user's
// configuration directory (on macOS: ~/Library/Application Support).
func DefaultPath() string {
	if p := os.Getenv("ELAPPMA_STATE"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	return filepath.Join(dir, "Elappma", "state.json")
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

// Delete forgets an installation.
func (s *State) Delete(repo string) { delete(s.Apps, key(repo)) }

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
