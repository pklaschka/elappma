package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GitHub talks to the GitHub API.
type GitHub struct {
	BaseURL string // defaults to https://api.github.com
	Token   string
	HTTP    *http.Client
}

func (c *GitHub) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.github.com"
}

func (c *GitHub) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *GitHub) request(ctx context.Context, url, accept string) (*http.Response, error) {
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
	if err := statusError("GitHub", resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

// Releases lists the newest releases of owner/repo, newest first.
func (c *GitHub) Releases(ctx context.Context, repo string) ([]Release, error) {
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

// Tree lists the files on the default branch of owner/repo (GitHub cuts very
// large trees short, which is fine for finding an icon).
func (c *GitHub) Tree(ctx context.Context, repo string) ([]TreeFile, error) {
	resp, err := c.request(ctx, c.base()+"/repos/"+repo+"/git/trees/HEAD?recursive=1", "application/vnd.github+json")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", repo, err)
	}
	defer resp.Body.Close()
	return readTree(resp.Body, repo)
}

// File returns a file from the default branch of owner/repo, at most limit
// bytes; it works for private repositories with a token, too.
func (c *GitHub) File(ctx context.Context, repo, file string, limit int64) ([]byte, error) {
	resp, err := c.request(ctx, c.base()+"/repos/"+repo+"/contents/"+escapePath(file), "application/vnd.github.raw+json")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", repo, err)
	}
	defer resp.Body.Close()
	return readLimited(resp.Body, repo, file, limit)
}

// Download writes the asset to w. progress, if set, is called as bytes arrive.
func (c *GitHub) Download(ctx context.Context, a Asset, w io.Writer, progress func(done, total int64)) error {
	resp, err := c.request(ctx, a.URL, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("downloading %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	return copyBody(resp, a, w, progress)
}
