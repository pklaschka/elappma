package store

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/forge"
	"github.com/pklaschka/lieferling/internal/install"
	"github.com/pklaschka/lieferling/internal/pick"
	"github.com/pklaschka/lieferling/internal/state"
)

// Progress reports a download: done and total bytes (total may be 0).
type Progress func(repo string, done, total int64)

// Install installs the newest release of the app's channel, replacing any
// older copy. It also serves for updates. The app is added to the list when
// it is not on it yet.
func (s *Store) Install(ctx context.Context, repo string, progress Progress) (Status, error) {
	repo, err := NormalizeRepo(repo)
	if err != nil {
		return Status{}, err
	}
	cfg, err := config.Load(s.ConfigPath)
	if err != nil {
		return Status{}, err
	}
	app := config.App{Repo: repo, Channel: pick.Stable}
	if a := cfg.Find(repo); a != nil {
		app = *a
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rel, asset, err := s.newest(ctx, app)
	if err != nil {
		return Status{}, err
	}
	return s.installRelease(ctx, app, rel, asset, progress)
}

// newest picks the newest release of the app's channel and its asset for
// this computer.
func (s *Store) newest(ctx context.Context, app config.App) (forge.Release, forge.Asset, error) {
	releases, err := s.Forge.Releases(ctx, app.Repo)
	if err != nil {
		return forge.Release{}, forge.Asset{}, err
	}
	rel, ok := pick.Release(releases, app.Channel)
	if !ok {
		return forge.Release{}, forge.Asset{}, fmt.Errorf("%s has no release in the %s channel", app.Repo, app.Channel)
	}
	asset, ok := pick.Asset(rel.Assets, s.GOOS, s.GOARCH)
	if !ok {
		return forge.Release{}, forge.Asset{}, fmt.Errorf("release %s of %s has nothing to install on %s/%s (one still being built has no files yet)", rel.TagName, app.Repo, s.GOOS, s.GOARCH)
	}
	return rel, asset, nil
}

// installRelease downloads asset of rel, installs it and records what was
// installed. The caller holds s.mu.
func (s *Store) installRelease(ctx context.Context, app config.App, rel forge.Release, asset forge.Asset, progress Progress) (Status, error) {
	file, remove, err := s.download(ctx, app, asset, progress)
	if err != nil {
		return Status{}, err
	}
	defer remove()

	res, err := s.Installer.Install(ctx, file, asset.Name)
	if err != nil {
		return Status{}, err
	}

	st, err := s.record(app, rel, asset, res)
	if err != nil {
		return Status{}, err
	}
	app.AppName = res.AppName
	return s.status(ctx, app, st), nil
}

// download writes the asset to a temporary file; remove deletes it again.
func (s *Store) download(ctx context.Context, app config.App, asset forge.Asset, progress Progress) (file string, remove func(), err error) {
	f, err := os.CreateTemp("", "lieferling-download-*-"+asset.Name)
	if err != nil {
		return "", nil, err
	}
	remove = func() { os.Remove(f.Name()) }
	var cb func(done, total int64)
	if progress != nil {
		cb = func(done, total int64) { progress(app.Repo, done, total) }
	}
	err = s.Forge.Download(ctx, app.Repo, asset, f, cb)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		remove()
		return "", nil, err
	}
	return f.Name(), remove, nil
}

// record writes what was installed to state.json and the app's name to the
// shared list, adding the app to the list when it is not on it yet.
func (s *Store) record(app config.App, rel forge.Release, asset forge.Asset, res install.Result) (state.State, error) {
	st, err := state.Load(s.StatePath)
	if err != nil {
		return state.State{}, err
	}
	st.Set(app.Repo, state.Installed{
		Version: pick.Version(rel.TagName), Tag: rel.TagName, Asset: asset.Name,
		Path: res.Path, InstalledAt: time.Now(),
	})
	if err := state.Save(s.StatePath, st); err != nil {
		return state.State{}, err
	}
	err = config.Update(s.ConfigPath, func(c *config.Config) error {
		a := c.Find(app.Repo)
		if a == nil {
			c.Apps = append(c.Apps, app)
			a = &c.Apps[len(c.Apps)-1]
		}
		a.AppName = res.AppName
		return nil
	})
	if err != nil {
		return state.State{}, err
	}
	return st, nil
}
