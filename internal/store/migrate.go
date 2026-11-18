package store

import (
	"os"
	"strings"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/state"
)

// The repository and app as they were called before the rename.
const (
	oldRepo, newRepo = "pklaschka/elappma", "pklaschka/lieferling"
	oldApp, newApp   = "Elappma", "Lieferling"
)

// MigrateFromElappma copies the list and the local state from where they were
// kept while Lieferling was called Elappma. It runs once: only when the new
// file does not exist yet and the old one does. The entry for the app itself
// moves to the renamed repository. The old files stay, for a Mac that still
// runs Elappma.
func MigrateFromElappma(configPath, legacyConfig, statePath, legacyState string) error {
	if missing(configPath) && !missing(legacyConfig) && configPath != legacyConfig {
		old, err := config.Load(legacyConfig)
		if err != nil {
			return err
		}
		for i := range old.Apps {
			if strings.EqualFold(old.Apps[i].Repo, oldRepo) {
				old.Apps[i].Repo = newRepo
			}
			if old.Apps[i].AppName == oldApp {
				old.Apps[i].AppName = newApp
			}
		}
		if err := config.Update(configPath, func(c *config.Config) error {
			*c = old
			return nil
		}); err != nil {
			return err
		}
	}
	if missing(statePath) && !missing(legacyState) && statePath != legacyState {
		old, err := state.Load(legacyState)
		if err != nil {
			return err
		}
		if in, ok := old.Get(oldRepo); ok {
			old.Delete(oldRepo)
			old.Set(newRepo, in)
		}
		if err := state.Save(statePath, old); err != nil {
			return err
		}
	}
	return nil
}

func missing(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}
