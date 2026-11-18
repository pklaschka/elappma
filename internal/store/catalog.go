package store

import (
	"strings"

	"github.com/pklaschka/lieferling/internal/catalog"
	"github.com/pklaschka/lieferling/internal/config"
)

// Export returns the list as a catalog.
func (s *Store) Export() (catalog.Catalog, error) {
	cfg, err := config.Load(s.ConfigPath)
	if err != nil {
		return catalog.Catalog{}, err
	}
	apps := make([]catalog.Entry, 0, len(cfg.Apps))
	for _, a := range cfg.Apps {
		ch, err := validChannel(a.Channel)
		if err != nil {
			ch = a.Channel
		}
		apps = append(apps, catalog.Entry{Repo: a.Repo, Channel: ch, AppName: a.AppName})
	}
	return catalog.New(apps), nil
}

// ImportItem is one app of a catalog, checked against the list.
type ImportItem struct {
	Repo    string `json:"repo"`    // normalized, or as written when invalid
	Channel string `json:"channel"` // stable or prerelease
	AppName string `json:"appName"`
	// OnList is true when the list has the app already; importing leaves it
	// as it is (its channel too).
	OnList bool `json:"onList"`
	// Error says why the entry cannot be imported.
	Error string `json:"error"`
}

// ImportPlan is what importing a catalog would do.
type ImportPlan struct {
	Name   string       `json:"name"`   // the catalog's own name, often ""
	Source string       `json:"source"` // file name or URL, set by the caller
	Items  []ImportItem `json:"items"`
}

// New returns the items that importing would add.
func (p ImportPlan) New() []ImportItem {
	var out []ImportItem
	for _, it := range p.Items {
		if !it.OnList && it.Error == "" {
			out = append(out, it)
		}
	}
	return out
}

// PlanImport checks every entry of a catalog and compares it with the list,
// without changing anything. Entries that name the same repository twice
// are folded into one.
func (s *Store) PlanImport(c catalog.Catalog) (ImportPlan, error) {
	cfg, err := config.Load(s.ConfigPath)
	if err != nil {
		return ImportPlan{}, err
	}
	plan := ImportPlan{Name: c.Name, Items: []ImportItem{}}
	seen := map[string]bool{}
	for _, e := range c.Apps {
		it := ImportItem{Repo: e.Repo, AppName: cleanAppName(e.AppName)}
		repo, err := NormalizeRepo(e.Repo)
		if err != nil {
			it.Error = err.Error()
			plan.Items = append(plan.Items, it)
			continue
		}
		it.Repo = repo
		if seen[strings.ToLower(repo)] {
			continue
		}
		seen[strings.ToLower(repo)] = true
		if it.Channel, err = validChannel(strings.ToLower(e.Channel)); err != nil {
			it.Channel, it.Error = e.Channel, err.Error()
		}
		if a := cfg.Find(repo); a != nil {
			it.OnList, it.Repo = true, a.Repo
		}
		plan.Items = append(plan.Items, it)
	}
	return plan, nil
}

// Import adds the given apps to the list and returns how many it added. Apps
// already on the list are left as they are; invalid entries are skipped. It
// does not ask GitHub: an app without a release shows its problem in the
// list, like one whose releases went away.
func (s *Store) Import(items []ImportItem) (int, error) {
	added := 0
	err := config.Update(s.ConfigPath, func(c *config.Config) error {
		added = 0
		for _, it := range items {
			if it.Error != "" {
				continue
			}
			repo, err := NormalizeRepo(it.Repo)
			if err != nil {
				continue
			}
			ch, err := validChannel(strings.ToLower(it.Channel))
			if err != nil {
				continue
			}
			if c.Find(repo) != nil {
				continue
			}
			c.Apps = append(c.Apps, config.App{Repo: repo, Channel: ch, AppName: cleanAppName(it.AppName)})
			added++
		}
		return nil
	})
	return added, err
}

// cleanAppName keeps an app name from a catalog only when it is a plain
// bundle name: it becomes part of a path (/Applications/<name>.app), and a
// catalog may come from anyone.
func cleanAppName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\:\x00") || len(name) > 255 {
		return ""
	}
	return name
}
