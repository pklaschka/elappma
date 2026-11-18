// Package store is the core that both the command line and the app use: it
// joins the shared list of apps, what is installed here, and GitHub.
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/github"
	"github.com/pklaschka/lieferling/internal/install"
	"github.com/pklaschka/lieferling/internal/pick"
	"github.com/pklaschka/lieferling/internal/state"
)

// Store does the work. Its zero value is not usable; use New.
type Store struct {
	ConfigPath string
	StatePath  string
	GitHub     *github.Client
	Installer  install.Installer
	GOOS       string
	GOARCH     string

	mu sync.Mutex // serializes installs and state writes
}

// New returns a store with the default paths, GitHub token and installer.
func New() *Store {
	cfg, st := config.DefaultPath(), state.DefaultPath()
	// best effort: without it the list starts empty, which Add can fix
	_ = MigrateFromElappma(cfg, config.LegacyPath(), st, state.LegacyPath())
	return &Store{
		ConfigPath: cfg,
		StatePath:  st,
		GitHub:     github.New(),
		Installer:  install.New(install.DefaultOptions()),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
	}
}

// Status is what the list shows for one app.
type Status struct {
	Repo      string `json:"repo"`
	Name      string `json:"name"`
	Channel   string `json:"channel"`
	Installed string `json:"installed"` // version here, "" when not installed
	Path      string `json:"path"`
	// ManagedByLieferling is false for a copy installed some other way (found
	// through the app name another computer recorded).
	ManagedByLieferling bool   `json:"managedByLieferling"`
	Latest              string `json:"latest"` // newest version in the channel
	LatestTag           string `json:"latestTag"`
	PublishedAt         string `json:"publishedAt"`
	ReleaseURL          string `json:"releaseUrl"`
	Asset               string `json:"asset"` // file that would be installed
	UpdateAvailable     bool   `json:"updateAvailable"`
	Error               string `json:"error"`
}

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// NormalizeRepo accepts owner/name and GitHub URLs (with or without .git).
func NormalizeRepo(s string) (string, error) {
	s = strings.TrimSpace(s)
	for _, p := range []string{"https://", "http://", "git@"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimPrefix(s, "www.")
	s = strings.TrimPrefix(s, "github.com/")
	s = strings.TrimPrefix(s, "github.com:")
	s = strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git")
	parts := strings.Split(s, "/")
	if len(parts) >= 2 {
		s = parts[0] + "/" + parts[1] // drop /releases and the like
	}
	if !repoPattern.MatchString(s) {
		return "", fmt.Errorf("%q is not a GitHub repository (owner/name or its URL)", s)
	}
	return s, nil
}

func validChannel(ch string) (string, error) {
	switch ch {
	case "", pick.Stable:
		return pick.Stable, nil
	case pick.Prerelease, "rc", "beta":
		return pick.Prerelease, nil
	}
	return "", fmt.Errorf("unknown channel %q: use stable or prerelease", ch)
}

// Add puts a repository on the list after checking that it has a release
// with something to install here.
func (s *Store) Add(ctx context.Context, repo, channel string) (Status, error) {
	repo, err := NormalizeRepo(repo)
	if err != nil {
		return Status{}, err
	}
	channel, err = validChannel(channel)
	if err != nil {
		return Status{}, err
	}
	st := s.status(ctx, config.App{Repo: repo, Channel: channel}, state.State{})
	if st.Error != "" {
		return st, errors.New(st.Error)
	}
	err = config.Update(s.ConfigPath, func(c *config.Config) error {
		if a := c.Find(repo); a != nil {
			a.Channel = channel
			return nil
		}
		c.Apps = append(c.Apps, config.App{Repo: repo, Channel: channel})
		return nil
	})
	return st, err
}

// SetChannel switches an app between stable and prerelease.
func (s *Store) SetChannel(repo, channel string) error {
	channel, err := validChannel(channel)
	if err != nil {
		return err
	}
	return config.Update(s.ConfigPath, func(c *config.Config) error {
		a := c.Find(repo)
		if a == nil {
			return fmt.Errorf("%s is not on the list", repo)
		}
		a.Channel = channel
		return nil
	})
}

// Remove takes an app off the list, and with uninstall also off this
// computer. Other computers keep their copy.
func (s *Store) Remove(ctx context.Context, repo string, uninstall bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := state.Load(s.StatePath)
	if err != nil {
		return err
	}
	if uninstall {
		if in, ok := st.Get(repo); ok {
			if err := s.Installer.Uninstall(ctx, in.Path); err != nil {
				return err
			}
		}
	}
	st.Delete(repo)
	if err := state.Save(s.StatePath, st); err != nil {
		return err
	}
	return config.Update(s.ConfigPath, func(c *config.Config) error {
		for i := range c.Apps {
			if strings.EqualFold(c.Apps[i].Repo, repo) {
				c.Apps = append(c.Apps[:i], c.Apps[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("%s is not on the list", repo)
	})
}

// Config returns the shared configuration.
func (s *Store) Config() (config.Config, error) { return config.Load(s.ConfigPath) }

// List returns the status of every app, asking GitHub for all at once.
func (s *Store) List(ctx context.Context) ([]Status, error) {
	cfg, err := config.Load(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	st, err := state.Load(s.StatePath)
	if err != nil {
		return nil, err
	}
	out := make([]Status, len(cfg.Apps))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, a := range cfg.Apps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = s.status(ctx, a, st)
		}()
	}
	wg.Wait()
	return out, nil
}

func (s *Store) status(ctx context.Context, a config.App, st state.State) Status {
	out := Status{Repo: a.Repo, Name: a.AppName, Channel: a.Channel}
	if out.Channel == "" {
		out.Channel = pick.Stable
	}
	if out.Name == "" {
		out.Name = a.Repo[strings.Index(a.Repo, "/")+1:]
	}
	if in, ok := st.Get(a.Repo); ok {
		out.Installed, out.Path, out.ManagedByLieferling = in.Version, in.Path, true
	} else if v, p := s.Installer.InstalledVersion(a.AppName); v != "" {
		out.Installed, out.Path = v, p
	}

	releases, err := s.GitHub.Releases(ctx, a.Repo)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	rel, ok := pick.Release(releases, out.Channel)
	if !ok {
		out.Error = "no release in the " + out.Channel + " channel"
		return out
	}
	out.Latest, out.LatestTag, out.ReleaseURL = pick.Version(rel.TagName), rel.TagName, rel.HTMLURL
	if !rel.PublishedAt.IsZero() {
		out.PublishedAt = rel.PublishedAt.Format(time.RFC3339)
	}
	asset, ok := pick.Asset(rel.Assets, s.GOOS, s.GOARCH)
	if !ok {
		out.Error = fmt.Sprintf("release %s has nothing to install on %s/%s", rel.TagName, s.GOOS, s.GOARCH)
		return out
	}
	out.Asset = asset.Name
	out.UpdateAvailable = out.Installed != "" && pick.Newer(out.Latest, out.Installed)
	return out
}

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

	releases, err := s.GitHub.Releases(ctx, app.Repo)
	if err != nil {
		return Status{}, err
	}
	rel, ok := pick.Release(releases, app.Channel)
	if !ok {
		return Status{}, fmt.Errorf("%s has no release in the %s channel", repo, app.Channel)
	}
	asset, ok := pick.Asset(rel.Assets, s.GOOS, s.GOARCH)
	if !ok {
		return Status{}, fmt.Errorf("release %s of %s has nothing to install on %s/%s (one still being built has no files yet)", rel.TagName, repo, s.GOOS, s.GOARCH)
	}

	f, err := os.CreateTemp("", "lieferling-download-*-"+asset.Name)
	if err != nil {
		return Status{}, err
	}
	defer os.Remove(f.Name())
	var cb func(done, total int64)
	if progress != nil {
		cb = func(done, total int64) { progress(app.Repo, done, total) }
	}
	err = s.GitHub.Download(ctx, asset, f, cb)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return Status{}, err
	}

	res, err := s.Installer.Install(ctx, f.Name(), asset.Name)
	if err != nil {
		return Status{}, err
	}

	st, err := state.Load(s.StatePath)
	if err != nil {
		return Status{}, err
	}
	st.Set(app.Repo, state.Installed{
		Version: pick.Version(rel.TagName), Tag: rel.TagName, Asset: asset.Name,
		Path: res.Path, InstalledAt: time.Now(),
	})
	if err := state.Save(s.StatePath, st); err != nil {
		return Status{}, err
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
		return Status{}, err
	}
	app.AppName = res.AppName
	return s.status(ctx, app, st), nil
}

// Updates returns the apps with a newer release than the installed one.
func (s *Store) Updates(ctx context.Context) ([]Status, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []Status
	for _, a := range all {
		if a.UpdateAvailable {
			out = append(out, a)
		}
	}
	return out, nil
}
