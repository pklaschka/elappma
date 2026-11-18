// Package github reads releases and downloads release assets through the
// GitHub REST API. Public repositories work without a token; private ones
// need one (see Token).
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Release is the part of a GitHub release this app uses.
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
	// URL is the API URL; it serves the file itself with
	// Accept: application/octet-stream, which also works for private repos.
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// ErrNotFound is returned when a repository has no releases or cannot be seen
// (GitHub answers 404 for private repositories without access, too).
var ErrNotFound = errors.New("repository or release not found (private repositories need a token)")

// Client talks to the GitHub API.
type Client struct {
	BaseURL string // defaults to https://api.github.com
	Token   string
	HTTP    *http.Client
}

// New returns a client that uses the token from Token().
func New() *Client {
	return &Client{Token: Token()}
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.github.com"
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) request(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("GitHub answered %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

// Releases lists the newest releases of owner/repo, newest first.
func (c *Client) Releases(ctx context.Context, repo string) ([]Release, error) {
	resp, err := c.request(ctx, c.base()+"/repos/"+repo+"/releases?per_page=30", "application/vnd.github+json")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", repo, err)
	}
	defer resp.Body.Close()
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("%s: reading releases: %w", repo, err)
	}
	return releases, nil
}

// Download writes the asset to w. progress, if set, is called as bytes arrive.
func (c *Client) Download(ctx context.Context, a Asset, w io.Writer, progress func(done, total int64)) error {
	resp, err := c.request(ctx, a.URL, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("downloading %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
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

// Token finds a GitHub token: GH_TOKEN or GITHUB_TOKEN, then the GitHub CLI's
// login (gh auth token), then, on macOS, the Keychain item "Lieferling GitHub"
// (security add-generic-password -s "Lieferling GitHub" -a github -w <token>)
// or the one from when Lieferling was called Elappma.
// It returns "" when there is none; public repositories still work then.
func Token() string {
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	if t := commandOutput(ghPath(), "auth", "token"); t != "" {
		return t
	}
	if runtime.GOOS == "darwin" {
		for _, item := range []string{"Lieferling GitHub", "Elappma GitHub"} {
			if t := commandOutput("security", "find-generic-password", "-s", item, "-w"); t != "" {
				return t
			}
		}
	}
	return ""
}

// ghPath finds the GitHub CLI also when the app is started from Finder, where
// PATH lacks Homebrew's directories.
func ghPath() string {
	if p, err := exec.LookPath("gh"); err == nil {
		return p
	}
	for _, p := range []string{"/opt/homebrew/bin/gh", "/usr/local/bin/gh"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func commandOutput(name string, args ...string) string {
	if name == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
