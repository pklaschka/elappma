package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Gitea talks to Forgejo and Gitea servers, which share the Gitea API
// (/api/v1). Every server is reached over https.
type Gitea struct {
	HTTP *http.Client
	// Token returns the token for a host; nil uses TokenFor.
	Token func(host string) string

	tokens tokenCache
}

// giteaRelease is a release as Forgejo and Gitea send it: like GitHub's, but
// assets carry their download link in browser_download_url.
type giteaRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []struct {
		Name        string `json:"name"`
		Size        int64  `json:"size"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (c *Gitea) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Gitea) token(host string) string {
	if c.Token != nil {
		return c.Token(host)
	}
	return c.tokens.get(host, TokenFor)
}

func (c *Gitea) get(ctx context.Context, r Repo, rawURL, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	// the token only goes to the repository's own server, never to a host a
	// download link points to
	if u, err := url.Parse(rawURL); err == nil && strings.EqualFold(u.Host, r.Host) {
		if t := c.token(r.Host); t != "" {
			req.Header.Set("Authorization", "token "+t)
		}
	}
	return c.http().Do(req)
}

// Releases lists the newest releases of the repository, newest first.
func (c *Gitea) Releases(ctx context.Context, r Repo) ([]Release, error) {
	api := "https://" + r.Host + "/api/v1/repos/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name) + "/releases?limit=30"
	resp, err := c.get(ctx, r, api, "application/json")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		if !c.isForge(ctx, r) {
			return nil, fmt.Errorf("%s %w", r.Host, ErrNoForge)
		}
		return nil, fmt.Errorf("%s: %w", r, ErrNotFound)
	}
	if resp.StatusCode >= 300 {
		return nil, answered(r.Host, resp)
	}
	var raw []giteaRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		// a web page instead of JSON: not a Forgejo or Gitea server
		return nil, fmt.Errorf("%s %w", r.Host, ErrNoForge)
	}
	out := make([]Release, len(raw))
	for i, g := range raw {
		out[i] = Release{
			TagName: g.TagName, Name: g.Name, Draft: g.Draft, Prerelease: g.Prerelease,
			PublishedAt: g.PublishedAt, HTMLURL: g.HTMLURL,
		}
		for _, a := range g.Assets {
			out[i].Assets = append(out[i].Assets, Asset{Name: a.Name, URL: a.DownloadURL, Size: a.Size})
		}
	}
	return out, nil
}

// isForge asks the server for its version, which every Forgejo and Gitea
// server answers, also without a token.
func (c *Gitea) isForge(ctx context.Context, r Repo) bool {
	resp, err := c.get(ctx, r, "https://"+r.Host+"/api/v1/version", "application/json")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var v struct {
		Version string `json:"version"`
	}
	return resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&v) == nil && v.Version != ""
}

// Download writes the asset to w. progress, if set, is called as bytes arrive.
func (c *Gitea) Download(ctx context.Context, r Repo, a Asset, w io.Writer, progress func(done, total int64)) error {
	if u, err := url.Parse(a.URL); err != nil || u.Scheme != "https" {
		return fmt.Errorf("downloading %s: %q is not an https link", a.Name, a.URL)
	}
	resp, err := c.get(ctx, r, a.URL, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("downloading %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("downloading %s: %w", a.Name, ErrNotFound)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("downloading %s: %w", a.Name, answered(r.Host, resp))
	}
	return copyBody(resp, a, w, progress)
}

func answered(host string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("%s answered %s: %s", host, resp.Status, strings.TrimSpace(string(body)))
}
