package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pklaschka/lieferling/internal/deeplink"
	"github.com/pklaschka/lieferling/internal/forge"
	"github.com/pklaschka/lieferling/internal/store"
)

// Deep links (lieferling://…) and links the window opens in the browser.

// TakeLink returns the deep link that opened Lieferling, once, or nil.
func (s *AppService) TakeLink() *deeplink.Link {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.link
	s.link = nil
	return l
}

// LinkFor returns the deep link that adds repo, for a README.
func (s *AppService) LinkFor(repo string, install bool) (string, error) {
	repo, err := store.NormalizeRepo(repo)
	if err != nil {
		return "", translate(err)
	}
	return deeplink.Link{Repo: repo, Install: install}.PageURL(), nil
}

// openLink keeps a deep link for the window and tells it.
func (s *AppService) openLink(raw string) {
	l, err := deeplink.Parse(raw)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.link = &l
	s.mu.Unlock()
	s.app.Event.Emit("link:opened")
}

// OpenURL opens a release page in the browser: only https links to GitHub
// or to the server of an app on the list.
func (s *AppService) OpenURL(raw string) error {
	if !s.knownHost(raw) {
		return fmt.Errorf("Nur Links zu GitHub und zu den Servern der Apps auf der Liste werden geöffnet.")
	}
	return s.app.Browser.OpenURL(raw)
}

func (s *AppService) knownHost(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	if strings.EqualFold(u.Host, "github.com") {
		return true
	}
	cfg, err := s.store.Config()
	if err != nil {
		return false
	}
	for _, a := range cfg.Apps {
		if r, err := forge.ParseRepo(a.Repo); err == nil && strings.EqualFold(r.WebHost(), u.Host) {
			return true
		}
	}
	return false
}
