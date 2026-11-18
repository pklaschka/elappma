package store

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/pklaschka/lieferling/internal/github"
	"github.com/pklaschka/lieferling/internal/icon"
)

func TestIconFromRepository(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 512, 512)))
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/repos/me/demo/git/trees/HEAD":
			w.Write([]byte(`{"tree":[{"path":"build","type":"tree"},{"path":"build/icon.png","type":"blob","size":` +
				strconv.Itoa(buf.Len()) + `},{"path":"README.md","type":"blob","size":10}]}`))
		case "/repos/me/demo/contents/build/icon.png":
			if r.Header.Get("Accept") != "application/vnd.github.raw+json" {
				http.Error(w, "wrong accept", 400)
				return
			}
			w.Write(buf.Bytes())
		case "/repos/me/plain/git/trees/HEAD":
			w.Write([]byte(`{"tree":[{"path":"main.go","type":"blob","size":10}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	s := &Store{
		ConfigPath: filepath.Join(dir, "config.json"),
		StatePath:  filepath.Join(dir, "state.json"),
		CacheDir:   filepath.Join(dir, "icons"),
		GitHub:     &github.Client{BaseURL: srv.URL},
		Installer:  &fakeInstaller{foreign: map[string]string{}},
	}
	ctx := context.Background()

	ic, err := s.Icon(ctx, "me/demo")
	if err != nil || ic.MIME != "image/png" || len(ic.Data) == 0 {
		t.Fatalf("Icon: %v %v", ic.MIME, err)
	}
	before := calls.Load()
	if _, err := s.Icon(ctx, "me/demo"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != before {
		t.Error("the cached icon was fetched again")
	}

	if _, err := s.Icon(ctx, "me/plain"); !errors.Is(err, icon.ErrNone) {
		t.Errorf("repo without icon: %v", err)
	}
	before = calls.Load()
	if _, err := s.Icon(ctx, "me/plain"); !errors.Is(err, icon.ErrNone) || calls.Load() != before {
		t.Errorf("no icon was not remembered: %v", err)
	}
	if _, err := s.Icon(ctx, "me/gone"); !errors.Is(err, icon.ErrNone) {
		t.Errorf("missing repo: %v", err)
	}
}
