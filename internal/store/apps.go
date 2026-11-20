package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/install"
	"github.com/pklaschka/lieferling/internal/state"
)

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

// Open starts an installed app on this computer, whether Lieferling
// installed it or found a copy installed some other way.
func (s *Store) Open(ctx context.Context, repo string) error {
	repo, err := NormalizeRepo(repo)
	if err != nil {
		return err
	}
	st, err := state.Load(s.StatePath)
	if err != nil {
		return err
	}
	path := s.installedPath(repo, st)
	if path == "" {
		return install.ErrNotInstalled
	}
	return s.Installer.Open(ctx, path)
}

// installedPath is where repo's app is installed on this computer, by
// Lieferling or otherwise, or "".
func (s *Store) installedPath(repo string, st state.State) string {
	if in, ok := st.Get(repo); ok {
		return in.Path
	}
	if cfg, err := config.Load(s.ConfigPath); err == nil {
		if a := cfg.Find(repo); a != nil {
			_, path := s.Installer.InstalledVersion(a.AppName)
			return path
		}
	}
	return ""
}
