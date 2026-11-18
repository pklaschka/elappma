// Package catalog reads and writes repository catalogs: a list of apps in a
// file that someone can hand on, keep in a repository or publish at a URL.
// Importing one adds its apps to the shared list; exporting writes the list
// out. A catalog is not the synced configuration itself: it holds only what
// another person needs to follow the same apps.
//
// The file Lieferling writes:
//
//	{
//	  "lieferling": "catalog",
//	  "version": 1,
//	  "apps": [
//	    {"repo": "pklaschka/feldspur", "channel": "stable", "appName": "Feldspur"}
//	  ]
//	}
//
// Reading is more forgiving: an object with "apps" (so Lieferling's own
// config.json works too), a bare JSON array of repositories or objects, or
// plain text with one repository per line ("# comments" and an optional
// channel after the repository, "owner/name prerelease").
package catalog

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Kind marks a file as a Lieferling catalog.
const Kind = "catalog"

// MaxSize is the largest catalog read, from a file or a URL.
const MaxSize = 1 << 20

// Entry is one app in a catalog. Repo is as written in the file; the store
// checks and normalizes it on import.
type Entry struct {
	Repo    string `json:"repo"`
	Channel string `json:"channel,omitempty"`
	AppName string `json:"appName,omitempty"`
}

// Catalog is a list of apps.
type Catalog struct {
	Lieferling string  `json:"lieferling"` // always Kind
	Version    int     `json:"version"`
	Name       string  `json:"name,omitempty"` // shown on import, if set
	Apps       []Entry `json:"apps"`
}

// New returns a catalog of the given apps.
func New(apps []Entry) Catalog {
	if apps == nil {
		apps = []Entry{}
	}
	return Catalog{Lieferling: Kind, Version: 1, Apps: apps}
}

// Encode writes the catalog as indented JSON.
func (c Catalog) Encode() ([]byte, error) {
	c.Lieferling, c.Version = Kind, 1
	if c.Apps == nil {
		c.Apps = []Entry{}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// ErrEmpty is returned for a file that names no repository at all.
var ErrEmpty = errors.New("the catalog lists no repositories")

// Parse reads a catalog in any of the accepted forms.
func Parse(data []byte) (Catalog, error) {
	data = bytes.TrimPrefix(bytes.TrimSpace(data), []byte("\xef\xbb\xbf"))
	var c Catalog
	var err error
	switch {
	case len(data) == 0:
		return c, ErrEmpty
	case data[0] == '{':
		c, err = parseObject(data)
	case data[0] == '[':
		var apps []Entry
		apps, err = parseApps(data)
		c = New(apps)
	case looksLikeHTML(data):
		return c, errors.New("this is a web page, not a catalog (for a file on GitHub, use its Raw link)")
	default:
		c = New(parseText(data))
	}
	if err != nil {
		return Catalog{}, err
	}
	c.Lieferling, c.Version = Kind, 1
	if len(c.Apps) == 0 {
		return c, ErrEmpty
	}
	return c, nil
}

func parseObject(data []byte) (Catalog, error) {
	var raw struct {
		Name string          `json:"name"`
		Apps json.RawMessage `json:"apps"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Catalog{}, fmt.Errorf("not valid JSON: %w", err)
	}
	if len(raw.Apps) == 0 {
		return Catalog{}, errors.New(`not a catalog: there is no "apps" list`)
	}
	apps, err := parseApps(raw.Apps)
	c := New(apps)
	c.Name = strings.TrimSpace(raw.Name)
	return c, err
}

// parseApps reads a JSON array whose items are repository strings or entries.
func parseApps(data []byte) ([]Entry, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf(`"apps" is not a list: %w`, err)
	}
	apps := make([]Entry, 0, len(items))
	for i, item := range items {
		var e Entry
		var s string
		if err := json.Unmarshal(item, &s); err == nil {
			e.Repo = s
		} else if err := json.Unmarshal(item, &e); err != nil {
			return nil, fmt.Errorf("app %d is neither a repository nor an object with \"repo\"", i+1)
		}
		e.Repo, e.Channel, e.AppName = strings.TrimSpace(e.Repo), strings.TrimSpace(e.Channel), strings.TrimSpace(e.AppName)
		if e.Repo != "" {
			apps = append(apps, e)
		}
	}
	return apps, nil
}

func parseText(data []byte) []Entry {
	var apps []Entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(strings.TrimLeft(strings.TrimSpace(line), "-*+ "))
		if len(fields) == 0 {
			continue
		}
		e := Entry{Repo: fields[0]}
		if len(fields) > 1 {
			e.Channel = fields[1]
		}
		apps = append(apps, e)
	}
	return apps
}

func looksLikeHTML(data []byte) bool {
	head := strings.ToLower(string(data[:min(len(data), 512)]))
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html")
}

// Fetch downloads a catalog over HTTPS. A link to a file's page on GitHub is
// turned into its raw link first.
func Fetch(ctx context.Context, client *http.Client, raw string) (Catalog, error) {
	u, err := FetchURL(raw)
	if err != nil {
		return Catalog{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Catalog{}, err
	}
	req.Header.Set("Accept", "application/json, text/plain;q=0.9, */*;q=0.1")
	resp, err := client.Do(req)
	if err != nil {
		return Catalog{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Catalog{}, fmt.Errorf("the server answered %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxSize+1))
	if err != nil {
		return Catalog{}, err
	}
	if len(data) > MaxSize {
		return Catalog{}, errors.New("the catalog is larger than 1 MB")
	}
	return Parse(data)
}

// FetchURL checks that raw is an https URL and turns a GitHub file page
// (github.com/owner/name/blob/ref/path) into its raw link.
func FetchURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%q is not a web address", raw)
	}
	if u.Scheme != "https" {
		return "", errors.New("catalogs are only loaded over https")
	}
	if strings.EqualFold(u.Host, "github.com") || strings.EqualFold(u.Host, "www.github.com") {
		parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 4)
		if len(parts) == 4 && (parts[2] == "blob" || parts[2] == "raw") {
			u.Host = "raw.githubusercontent.com"
			u.Path = "/" + parts[0] + "/" + parts[1] + "/" + parts[3]
			u.RawQuery = ""
		}
	}
	return u.String(), nil
}
