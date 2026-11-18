// Package deeplink reads lieferling:// links, which a README can use (through
// the landing page in site/, since GitHub only links to https) to put an app
// on the list and, if asked, install it:
//
//	lieferling://add?repo=owner/name
//	lieferling://add?repo=owner/name&install=1&prerelease=1
//	lieferling://install/owner/name
package deeplink

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pklaschka/lieferling/internal/store"
)

// Scheme is the URL scheme Lieferling registers.
const Scheme = "lieferling"

// Page is the landing page (site/, on GitHub Pages) that turns an https
// link from a README into an lieferling:// link.
const Page = "https://pklaschka.github.io/lieferling/"

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
	// elappma:// is the scheme from before the rename; old links keep working
	if err != nil || !(strings.EqualFold(u.Scheme, Scheme) || strings.EqualFold(u.Scheme, "elappma")) {
		return Link{}, fmt.Errorf("not an Lieferling link: %q", raw)
	}
	q := u.Query()
	repo := q.Get("repo")
	if repo == "" {
		repo = strings.Trim(u.Path, "/") // lieferling://add/owner/name
	}
	var l Link
	switch strings.ToLower(u.Host) {
	case "add":
	case "install":
		l.Install = true
	default:
		return Link{}, fmt.Errorf("unknown Lieferling link %q: expected add or install", u.Host)
	}
	if l.Repo, err = store.NormalizeRepo(repo); err != nil {
		return Link{}, err
	}
	l.Install = l.Install || truthy(q.Get("install"))
	l.Prerelease = truthy(q.Get("prerelease")) || strings.EqualFold(q.Get("channel"), "prerelease")
	return l, nil
}

// URL is the lieferling:// link.
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
