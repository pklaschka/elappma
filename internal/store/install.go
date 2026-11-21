package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/forge"
	"github.com/pklaschka/lieferling/internal/install"
	"github.com/pklaschka/lieferling/internal/pick"
	"github.com/pklaschka/lieferling/internal/state"
	"github.com/pklaschka/lieferling/internal/verify"
)

// Progress reports a download: done and total bytes (total may be 0).
type Progress func(repo string, done, total int64)

// Install installs the newest release of the app's channel, replacing any
// older copy. It also serves for updates. The app is added to the list when
// it is not on it yet.
//
// Every package goes through the customs check first (package verify). A
// package that is held or whose sender changed is not installed: the
// returned Status carries the report in Customs, and the error wraps
// verify.ErrHeld or verify.ErrAsk.
func (s *Store) Install(ctx context.Context, repo string, progress Progress) (Status, error) {
	return s.install(ctx, repo, progress, nil)
}

// AcceptSender installs the package whose sender change is waiting for repo.
// The acceptance covers exactly the digest and the sender in the waiting
// report: if the download or the app inside differs, the result is a new
// report and nothing is installed.
func (s *Store) AcceptSender(ctx context.Context, repo string, progress Progress) (Status, error) {
	repo, err := NormalizeRepo(repo)
	if err != nil {
		return Status{}, err
	}
	st, err := state.Load(s.StatePath)
	if err != nil {
		return Status{}, err
	}
	held, ok := st.HeldFor(repo)
	if !ok || held.Verdict != verify.Ask {
		return Status{}, fmt.Errorf("%s has no sender change waiting for confirmation", repo)
	}
	return s.install(ctx, repo, progress, &held)
}

func (s *Store) install(ctx context.Context, repo string, progress Progress, accepted *verify.Report) (Status, error) {
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
	return s.installRelease(ctx, app, rel, asset, progress, accepted)
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

// installRelease downloads asset of rel, checks it, installs it and records
// what was installed. The caller holds s.mu.
func (s *Store) installRelease(ctx context.Context, app config.App, rel forge.Release, asset forge.Asset, progress Progress, accepted *verify.Report) (Status, error) {
	file, digest, remove, err := s.download(ctx, app, asset, progress)
	if err != nil {
		return Status{}, err
	}
	defer remove()

	// read the list again: another computer may have written digests meanwhile
	cfg, err := config.Load(s.ConfigPath)
	if err != nil {
		return Status{}, err
	}
	if a := cfg.Find(app.Repo); a != nil {
		app = *a
	}
	version := pick.Version(rel.TagName)
	rep := verify.Report{
		Version: version, Asset: asset.Name, Digest: digest,
		Policy: verify.Resolve(cfg.Verify, app.Verify), CheckedAt: time.Now(),
	}
	name := app.AppName
	if name == "" {
		name = app.Repo
	}

	if rep.Policy == verify.Off {
		rep.Verdict = verify.Exempt
		res, err := s.Installer.Install(ctx, file, asset.Name, nil)
		if err != nil {
			return Status{}, err
		}
		return s.finish(ctx, app, rel, asset, res, rep, verify.Sender{})
	}

	if accepted != nil && accepted.Digest != digest {
		accepted = nil // another package than the one the user saw
	}
	rep.Checks = s.checkDownload(ctx, app, rel, asset, digest)
	if rep.Failed() {
		rep.Verdict, rep.Checks = verify.Decide(rep.Policy, rep.Checks)
		return s.held(ctx, app, name, rep)
	}

	st, err := state.Load(s.StatePath)
	if err != nil {
		return Status{}, err
	}
	var now verify.Sender
	res, err := s.Installer.Install(ctx, file, asset.Name, func(ctx context.Context, u install.Unpacked) error {
		now = verify.Sender{BundleID: u.Identity.BundleID, TeamID: u.Identity.TeamID}
		if now.BundleID != "" {
			rep.Sender = &now
		}
		known := s.knownSender(ctx, app, st)
		ok := accepted != nil && accepted.After != nil && accepted.After.Equal(now)
		rep.Checks = append(rep.Checks, verify.SenderCheck(known, now, ok))
		if !known.IsZero() && !known.Equal(now) {
			rep.Before, rep.After = &known, &now
		}
		rep.Verdict, rep.Checks = verify.Decide(rep.Policy, rep.Checks)
		switch rep.Verdict {
		case verify.Ask:
			return verify.ErrAsk
		case verify.Held:
			return verify.ErrHeld
		}
		return nil
	})
	if errors.Is(err, verify.ErrAsk) || errors.Is(err, verify.ErrHeld) {
		return s.held(ctx, app, name, rep)
	}
	if err != nil {
		return Status{}, err
	}
	return s.finish(ctx, app, rel, asset, res, rep, now)
}

// held records a package that was not installed and returns the status that
// shows it, with an error that says why.
func (s *Store) held(ctx context.Context, app config.App, name string, rep verify.Report) (Status, error) {
	if err := s.hold(app, rep); err != nil {
		return Status{}, err
	}
	st, err := state.Load(s.StatePath)
	if err != nil {
		return Status{}, err
	}
	if cfg, err := config.Load(s.ConfigPath); err == nil {
		if a := cfg.Find(app.Repo); a != nil {
			app = *a
		}
	}
	return s.status(ctx, app, st), heldError(name, rep)
}

// finish records an installed package and returns its status.
func (s *Store) finish(ctx context.Context, app config.App, rel forge.Release, asset forge.Asset, res install.Result, rep verify.Report, sender verify.Sender) (Status, error) {
	st, err := s.record(app, rel, asset, res, rep, sender)
	if err != nil {
		return Status{}, err
	}
	app.AppName = res.AppName
	return s.status(ctx, app, st), nil
}

// download writes the asset to a temporary file and returns its SHA-256,
// computed while the bytes arrive; remove deletes the file again.
func (s *Store) download(ctx context.Context, app config.App, asset forge.Asset, progress Progress) (file, digest string, remove func(), err error) {
	f, err := os.CreateTemp("", "lieferling-download-*-"+asset.Name)
	if err != nil {
		return "", "", nil, err
	}
	remove = func() { os.Remove(f.Name()) }
	var cb func(done, total int64)
	if progress != nil {
		cb = func(done, total int64) { progress(app.Repo, done, total) }
	}
	h := sha256.New()
	err = s.Forge.Download(ctx, app.Repo, asset, io.MultiWriter(f, h), cb)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		remove()
		return "", "", nil, err
	}
	return f.Name(), verify.FormatDigest(h.Sum(nil)), remove, nil
}

// record writes what was installed and its customs report to state.json, and
// the app's name, sender and digest to the shared list, adding the app to the
// list when it is not on it yet. A package exempt from the check writes
// neither sender nor digest.
func (s *Store) record(app config.App, rel forge.Release, asset forge.Asset, res install.Result, rep verify.Report, sender verify.Sender) (state.State, error) {
	st, err := state.Load(s.StatePath)
	if err != nil {
		return state.State{}, err
	}
	st.Set(app.Repo, state.Installed{
		Version: pick.Version(rel.TagName), Tag: rel.TagName, Asset: asset.Name,
		Path: res.Path, InstalledAt: time.Now(), Customs: ptr(rep.Stored()),
	})
	st.Release(app.Repo)
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
		if rep.Verdict == verify.Exempt {
			return nil
		}
		if sender.BundleID != "" {
			a.BundleID, a.TeamID = sender.BundleID, sender.TeamID
		}
		if rep.Digest != "" {
			if a.Digests == nil {
				a.Digests = map[string]string{}
			}
			k := verify.Key(rep.Version, rep.Asset)
			a.Digests[k] = verify.Merge(a.Digests[k], rep.Digest)
			pruneDigests(a.Digests, keepVersions, k)
		}
		return nil
	})
	if err != nil {
		return state.State{}, err
	}
	return st, nil
}

func ptr[T any](v T) *T { return &v }
