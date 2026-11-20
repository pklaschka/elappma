package forge

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrNotFound is returned when a repository has no releases or cannot be seen
// (GitHub, Forgejo and Gitea answer 404 for private repositories without
// access, too).
var ErrNotFound = errors.New("repository or release not found (private repositories need a token)")

// ErrNoForge is returned for a host that does not serve the Forgejo/Gitea API.
var ErrNoForge = errors.New("does not answer like a Forgejo or Gitea server")

// ErrBadRepo is returned by ParseRepo for anything it cannot read.
var ErrBadRepo = errors.New("is not a repository (owner/name, host/owner/name or its URL)")

// statusError is where an HTTP answer becomes an error: nil for success,
// ErrNotFound for 404, and for any other failure an error that names who
// answered ("GitHub" or the host) and quotes the start of the body. It leaves
// closing the body to the caller.
func statusError(who string, resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode >= 300:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s answered %s: %s", who, resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
