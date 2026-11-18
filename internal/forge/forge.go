// Package forge reads releases and downloads release assets from the places
// a repository can live: GitHub, and any Forgejo or Gitea server (such as
// codeberg.org), which share one API. Public repositories work without a
// token; private ones need one (see GitHubToken and TokenFor).
package forge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Release is the part of a release this app uses.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// Asset is a file attached to a release.
type Asset struct {
	Name string `json:"name"`
	// URL serves the file. On GitHub it is the API URL, which serves the file
	// itself with Accept: application/octet-stream (this also works for
	// private repositories); on Forgejo and Gitea it is the download link.
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// ErrNotFound is returned when a repository has no releases or cannot be seen
// (GitHub, Forgejo and Gitea answer 404 for private repositories without
// access, too).
var ErrNotFound = errors.New("repository or release not found (private repositories need a token)")

// ErrNoForge is returned for a host that does not serve the Forgejo/Gitea API.
var ErrNoForge = errors.New("does not answer like a Forgejo or Gitea server")

// Repo names a repository: Host is "" for GitHub, else the Forgejo or Gitea
// server ("codeberg.org", "git.example.com:3000").
type Repo struct {
	Host, Owner, Name string
}

var (
	namePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	hostPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]{1,5})?$`)
)

// ErrBadRepo is returned by ParseRepo for anything it cannot read.
var ErrBadRepo = errors.New("is not a repository (owner/name, host/owner/name or its URL)")

// ParseRepo accepts owner/name for GitHub, host/owner/name for a Forgejo or
// Gitea server, and the web, clone and SSH URLs of either (with or without
// .git, /releases and the like).
func ParseRepo(s string) (Repo, error) {
	orig := strings.TrimSpace(s)
	s = orig
	for _, p := range []string{"https://", "http://", "ssh://"} {
		if len(s) >= len(p) && strings.EqualFold(s[:len(p)], p) {
			s = s[len(p):]
		}
	}
	if rest, ok := strings.CutPrefix(s, "git@"); ok {
		// git@host:owner/name, or git@host:2222/owner/name from ssh://
		host, path, found := strings.Cut(rest, ":")
		if found && !strings.Contains(host, "/") {
			if port, after, ok := strings.Cut(path, "/"); ok && isDigits(port) {
				path = after // an SSH port says nothing about the web address
			}
			s = host + "/" + path
		} else {
			s = rest
		}
	}
	s, _, _ = strings.Cut(s, "?")
	s, _, _ = strings.Cut(s, "#")
	var parts []string
	for _, p := range strings.Split(s, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	var r Repo
	if len(parts) > 0 && looksLikeHost(parts[0]) {
		r.Host = strings.TrimPrefix(strings.ToLower(parts[0]), "www.")
		parts = parts[1:]
		if r.Host == "github.com" {
			r.Host = ""
		}
	}
	if len(parts) < 2 {
		return Repo{}, fmt.Errorf("%q %w", orig, ErrBadRepo)
	}
	r.Owner, r.Name = parts[0], strings.TrimSuffix(parts[1], ".git") // drop /releases and the like
	if !namePattern.MatchString(r.Owner) || !namePattern.MatchString(r.Name) ||
		r.Host != "" && !hostPattern.MatchString(r.Host) {
		return Repo{}, fmt.Errorf("%q %w", orig, ErrBadRepo)
	}
	return r, nil
}

// A first part with a dot, a port or "localhost" is a host: GitHub owners
// cannot contain dots.
func looksLikeHost(s string) bool {
	return strings.ContainsAny(s, ".:") || strings.EqualFold(s, "localhost")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// String is how the repository is written in the list: owner/name on GitHub,
// host/owner/name elsewhere.
func (r Repo) String() string {
	if r.Host == "" {
		return r.Owner + "/" + r.Name
	}
	return r.Host + "/" + r.Owner + "/" + r.Name
}

// Path is owner/name.
func (r Repo) Path() string { return r.Owner + "/" + r.Name }

// WebHost is the host of the repository's web pages.
func (r Repo) WebHost() string {
	if r.Host == "" {
		return "github.com"
	}
	return r.Host
}

// Forge is the name to show for where the repository lives: "GitHub",
// "Codeberg", or the host of any other server.
func (r Repo) Forge() string {
	switch r.Host {
	case "":
		return "GitHub"
	case "codeberg.org":
		return "Codeberg"
	}
	return r.Host
}

// Client reads releases from wherever a repository lives.
type Client struct {
	GitHub *GitHub
	Gitea  *Gitea
}

// New returns a client that finds tokens the usual ways.
func New() *Client {
	return &Client{GitHub: &GitHub{Token: GitHubToken()}, Gitea: &Gitea{}}
}

// Releases lists the newest releases of repo (as ParseRepo reads it), newest
// first.
func (c *Client) Releases(ctx context.Context, repo string) ([]Release, error) {
	r, err := ParseRepo(repo)
	if err != nil {
		return nil, err
	}
	if r.Host == "" {
		return c.GitHub.Releases(ctx, r.Path())
	}
	return c.Gitea.Releases(ctx, r)
}

// Download writes an asset of repo's releases to w. progress, if set, is
// called as bytes arrive.
func (c *Client) Download(ctx context.Context, repo string, a Asset, w io.Writer, progress func(done, total int64)) error {
	r, err := ParseRepo(repo)
	if err != nil {
		return err
	}
	if r.Host == "" {
		return c.GitHub.Download(ctx, a, w, progress)
	}
	return c.Gitea.Download(ctx, r, a, w, progress)
}

// TreeFile is a file in a repository.
type TreeFile struct {
	Path string `json:"path"`
	Type string `json:"type"` // blob for files
	Size int64  `json:"size"`
}

// Tree lists the files on the default branch of repo.
func (c *Client) Tree(ctx context.Context, repo string) ([]TreeFile, error) {
	r, err := ParseRepo(repo)
	if err != nil {
		return nil, err
	}
	if r.Host == "" {
		return c.GitHub.Tree(ctx, r.Path())
	}
	return c.Gitea.Tree(ctx, r)
}

// File returns a file from the default branch of repo, at most limit bytes.
func (c *Client) File(ctx context.Context, repo, file string, limit int64) ([]byte, error) {
	r, err := ParseRepo(repo)
	if err != nil {
		return nil, err
	}
	if r.Host == "" {
		return c.GitHub.File(ctx, r.Path(), file, limit)
	}
	return c.Gitea.File(ctx, r, file, limit)
}

// readTree reads a git tree as GitHub, Forgejo and Gitea send it and keeps
// the files.
func readTree(body io.Reader, repo string) ([]TreeFile, error) {
	var t struct {
		Tree []TreeFile `json:"tree"`
	}
	if err := json.NewDecoder(body).Decode(&t); err != nil {
		return nil, fmt.Errorf("%s: reading the file list: %w", repo, err)
	}
	files := t.Tree[:0]
	for _, f := range t.Tree {
		if f.Type == "blob" {
			files = append(files, f)
		}
	}
	return files, nil
}

func readLimited(body io.Reader, repo, file string, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%s: reading %s: %w", repo, file, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s: %s is too large", repo, file)
	}
	return data, nil
}

func escapePath(file string) string {
	parts := strings.Split(file, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// copyBody writes a download to w, reporting progress.
func copyBody(resp *http.Response, a Asset, w io.Writer, progress func(done, total int64)) error {
	total := resp.ContentLength
	if total <= 0 {
		total = a.Size
	}
	var r io.Reader = resp.Body
	if progress != nil {
		r = &progressReader{r: resp.Body, total: total, fn: progress}
	}
	if _, err := io.Copy(w, r); err != nil {
		return fmt.Errorf("downloading %s: %w", a.Name, err)
	}
	return nil
}

type progressReader struct {
	r     io.Reader
	done  int64
	total int64
	fn    func(done, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	p.fn(p.done, p.total)
	return n, err
}

// tokenCache remembers tokens per host, since asking the Keychain is slow.
type tokenCache struct {
	mu     sync.Mutex
	tokens map[string]string
}

func (t *tokenCache) get(host string, find func(string) string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if v, ok := t.tokens[host]; ok {
		return v
	}
	if t.tokens == nil {
		t.tokens = map[string]string{}
	}
	v := find(host)
	t.tokens[host] = v
	return v
}
