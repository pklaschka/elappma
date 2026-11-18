package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/forge"
	"github.com/pklaschka/lieferling/internal/icon"
	"github.com/pklaschka/lieferling/internal/state"
)

// How long an icon found in a repository is kept before looking again, and
// how long to wait after finding none. Both spare the GitHub rate limit,
// which allows 60 requests an hour without a token, and the other servers.
const (
	repoIconFresh = 7 * 24 * time.Hour
	noIconFresh   = 24 * time.Hour
)

// cached is what the icon cache keeps per repository.
type cached struct {
	icon.Icon
	Source  string    `json:"source"` // "bundle" or "repo"
	Stamp   string    `json:"stamp"`  // bundle path and change time, or repo file
	Checked time.Time `json:"checked"`
}

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Lieferling", "icons")
}

// Icon returns the app's icon: from its bundle when it is installed here,
// else from its repository. It returns icon.ErrNone when there is none.
func (s *Store) Icon(ctx context.Context, repo string) (icon.Icon, error) {
	repo, err := NormalizeRepo(repo)
	if err != nil {
		return icon.Icon{}, err
	}
	st, err := state.Load(s.StatePath)
	if err != nil {
		return icon.Icon{}, err
	}
	old, _ := s.loadIcon(repo)

	if path := s.installedPath(repo, st); path != "" {
		if fi, err := os.Stat(filepath.Join(path, "Contents", "Info.plist")); err == nil {
			stamp := path + "@" + fi.ModTime().UTC().Format(time.RFC3339Nano)
			if old.Source == "bundle" && old.Stamp == stamp && len(old.Data) > 0 {
				return old.Icon, nil
			}
			if ic, err := icon.FromBundle(ctx, path); err == nil {
				s.saveIcon(repo, cached{Icon: ic, Source: "bundle", Stamp: stamp, Checked: time.Now()})
				return ic, nil
			}
		}
	}

	if old.Source == "repo" {
		fresh := repoIconFresh
		if len(old.Data) == 0 {
			fresh = noIconFresh
		}
		if time.Since(old.Checked) < fresh {
			return old.found()
		}
	}
	ic, file, err := s.repoIcon(ctx, repo)
	if err != nil && !errors.Is(err, icon.ErrNone) {
		if len(old.Data) > 0 {
			return old.Icon, nil // offline: the last one will do
		}
		return icon.Icon{}, err
	}
	c := cached{Icon: ic, Source: "repo", Stamp: file, Checked: time.Now()}
	s.saveIcon(repo, c)
	return c.found()
}

func (c cached) found() (icon.Icon, error) {
	if len(c.Data) == 0 {
		return icon.Icon{}, icon.ErrNone
	}
	return c.Icon, nil
}

// repoIcon looks through the repository's files for the app icon.
func (s *Store) repoIcon(ctx context.Context, repo string) (icon.Icon, string, error) {
	tree, err := s.Forge.Tree(ctx, repo)
	if err != nil {
		if errors.Is(err, forge.ErrNotFound) {
			return icon.Icon{}, "", icon.ErrNone // an empty repository has no tree
		}
		return icon.Icon{}, "", err
	}
	files := make([]icon.File, len(tree))
	for i, f := range tree {
		files[i] = icon.File{Path: f.Path, Size: f.Size}
	}
	file, ok := icon.Pick(files)
	if !ok {
		return icon.Icon{}, "", icon.ErrNone
	}
	data, err := s.Forge.File(ctx, repo, file, icon.MaxFile)
	if err != nil {
		return icon.Icon{}, file, err
	}
	ic, err := icon.Normalize(data, file)
	return ic, file, err
}

func (s *Store) iconFile(repo string) string {
	if s.CacheDir == "" {
		return ""
	}
	return filepath.Join(s.CacheDir, strings.ReplaceAll(strings.ToLower(repo), "/", "__")+".json")
}

func (s *Store) loadIcon(repo string) (cached, error) {
	var c cached
	f := s.iconFile(repo)
	if f == "" {
		return c, os.ErrNotExist
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(data, &c)
}

// saveIcon keeps the icon for next time; failing to is no reason to fail.
func (s *Store) saveIcon(repo string, c cached) {
	f := s.iconFile(repo)
	if f == "" {
		return
	}
	if data, err := json.Marshal(c); err == nil {
		_ = config.WriteFileAtomic(f, data)
	}
}
