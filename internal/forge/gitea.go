package forge

import (
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
	api := c.repoAPI(r) + "/releases?limit=30"
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
	if err := statusError(r.Host, resp); err != nil {
		return nil, err
	}
	var raw []giteaRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		// a web page instead of JSON: not a Forgejo or Gitea server
		return nil, fmt.Errorf("%s %w", r.Host, ErrNoForge)
	}
	out := make([]Release, len(raw))
	for i, g := range raw {
		out[i] = g.release()
	}
	return out, nil
}

// release is the release in the shape GitHub sends it.
func (g giteaRelease) release() Release {
	rel := Release{
		TagName: g.TagName, Name: g.Name, Draft: g.Draft, Prerelease: g.Prerelease,
		PublishedAt: g.PublishedAt, HTMLURL: g.HTMLURL,
	}
	for _, a := range g.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.DownloadURL, Size: a.Size})
	}
	return rel
}

func (c *Gitea) repoAPI(r Repo) string {
	return "https://" + r.Host + "/api/v1/repos/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name)
}

// getOK is get for API calls: 404 becomes ErrNotFound, other failures an error.
func (c *Gitea) getOK(ctx context.Context, r Repo, rawURL string) (*http.Response, error) {
	resp, err := c.get(ctx, r, rawURL, "application/json")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r, err)
	}
	if err := statusError(r.Host, resp); err != nil {
		resp.Body.Close()
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%s: %w", r, err)
		}
		return nil, err
	}
	return resp, nil
}

// Tree lists the files on the default branch of the repository. Forgejo and
// Gitea name the default branch in the repository and hand out the tree in
// pages; the first page is plenty for finding an icon.
func (c *Gitea) Tree(ctx context.Context, r Repo) ([]TreeFile, error) {
	resp, err := c.getOK(ctx, r, c.repoAPI(r))
	if err != nil {
		return nil, err
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
		Empty         bool   `json:"empty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&info)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("%s: reading the repository: %w", r, err)
	}
	if info.Empty || info.DefaultBranch == "" {
		return nil, fmt.Errorf("%s: %w", r, ErrNotFound)
	}
	resp, err = c.getOK(ctx, r, c.repoAPI(r)+"/git/trees/"+url.PathEscape(info.DefaultBranch)+"?recursive=true&per_page=10000")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return readTree(resp.Body, r.String())
}

// File returns a file from the default branch, at most limit bytes.
func (c *Gitea) File(ctx context.Context, r Repo, file string, limit int64) ([]byte, error) {
	resp, err := c.getOK(ctx, r, c.repoAPI(r)+"/raw/"+escapePath(file))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return readLimited(resp.Body, r.String(), file, limit)
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
	if err := statusError(r.Host, resp); err != nil {
		return fmt.Errorf("downloading %s: %w", a.Name, err)
	}
	return copyBody(resp, a, w, progress)
}
