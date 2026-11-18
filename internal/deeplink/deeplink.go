// Package deeplink reads elappma:// links, which a README can use (through
// the landing page in site/, since GitHub only links to https) to put an app
// on the list and, if asked, install it:
//
//	elappma://add?repo=owner/name
//	elappma://add?repo=owner/name&install=1&prerelease=1
//	elappma://install/owner/name
package deeplink

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pklaschka/elappma/internal/store"
)

// Scheme is the URL scheme Elappma registers.
const Scheme = "elappma"

// Page is the landing page (site/, on GitHub Pages) that turns an https
// link from a README into an elappma:// link.
const Page = "https://pklaschka.github.io/elappma/"

// Link is what a deep link asks for. The window still asks before acting:
// a link comes from a web page.
type Link struct {
	Repo       string `json:"repo"`
	Install    bool   `json:"install"`
	Prerelease bool   `json:"prerelease"`
}

// Parse reads a deep link.
func Parse(raw string) (Link, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(u.Scheme, Scheme) {
		return Link{}, fmt.Errorf("not an Elappma link: %q", raw)
	}
	q := u.Query()
	repo := q.Get("repo")
	if repo == "" {
		repo = strings.Trim(u.Path, "/") // elappma://add/owner/name
	}
	var l Link
	switch strings.ToLower(u.Host) {
	case "add":
	case "install":
		l.Install = true
	default:
		return Link{}, fmt.Errorf("unknown Elappma link %q: expected add or install", u.Host)
	}
	if l.Repo, err = store.NormalizeRepo(repo); err != nil {
		return Link{}, err
	}
	l.Install = l.Install || truthy(q.Get("install"))
	l.Prerelease = truthy(q.Get("prerelease")) || strings.EqualFold(q.Get("channel"), "prerelease")
	return l, nil
}

// URL is the elappma:// link.
func (l Link) URL() string { return Scheme + "://add?" + l.query() }

// PageURL is the https link to the landing page, for a README.
func (l Link) PageURL() string { return Page + "?" + l.query() }

func (l Link) query() string {
	q := url.Values{"repo": {l.Repo}}
	if l.Install {
		q.Set("install", "1")
	}
	if l.Prerelease {
		q.Set("prerelease", "1")
	}
	// keep owner/name readable in the link
	return strings.ReplaceAll(q.Encode(), "%2F", "/")
}

func truthy(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
