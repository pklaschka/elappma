package store

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/github"
	"github.com/pklaschka/lieferling/internal/install"
)

type fakeInstaller struct {
	installed map[string]string // asset name -> file contents
	foreign   map[string]string // app name -> version installed without Lieferling
	opened    []string          // paths passed to Open
}

func (f *fakeInstaller) Install(_ context.Context, file, asset string) (install.Result, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return install.Result{}, err
	}
	f.installed[asset] = string(data)
	return install.Result{Path: "/Applications/Demo.app", AppName: "Demo"}, nil
}

func (f *fakeInstaller) Uninstall(context.Context, string) error { return nil }

func (f *fakeInstaller) Open(_ context.Context, path string) error {
	f.opened = append(f.opened, path)
	return nil
}

func (f *fakeInstaller) InstalledVersion(name string) (string, string) {
	if v, ok := f.foreign[name]; ok {
		return v, "/Applications/" + name + ".app"
	}
	return "", ""
}

func newTestStore(t *testing.T, releases map[string][]github.Release) (*Store, *fakeInstaller) {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/download/") {
			if r.Header.Get("Accept") != "application/octet-stream" {
				http.Error(w, "wrong accept", 400)
				return
			}
			w.Write([]byte("contents of " + strings.TrimPrefix(r.URL.Path, "/download/")))
			return
		}
		repo := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/repos/"), "/releases")
		rs, ok := releases[repo]
		if !ok {
			http.NotFound(w, r)
			return
		}
		for i := range rs {
			for j := range rs[i].Assets {
				rs[i].Assets[j].URL = srv.URL + "/download/" + rs[i].Assets[j].Name
			}
		}
		json.NewEncoder(w).Encode(rs)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	fi := &fakeInstaller{installed: map[string]string{}, foreign: map[string]string{}}
	return &Store{
		ConfigPath: filepath.Join(dir, "icloud", "config.json"),
		StatePath:  filepath.Join(dir, "local", "state.json"),
		GitHub:     &github.Client{BaseURL: srv.URL},
		Installer:  fi,
		GOOS:       "darwin",
		GOARCH:     "arm64",
	}, fi
}

func TestAddInstallUpdate(t *testing.T) {
	releases := map[string][]github.Release{
		"me/demo": {
			{TagName: "v1.1.0-rc.1", Prerelease: true, Assets: []github.Asset{{Name: "Demo-1.1.0-rc.1-arm64.dmg"}}},
			{TagName: "v1.0.0", Assets: []github.Asset{{Name: "Demo-1.0.0-arm64.dmg"}, {Name: "Demo-1.0.0-x64.dmg"}}},
		},
	}
	s, fi := newTestStore(t, releases)
	ctx := context.Background()

	if _, err := s.Add(ctx, "https://github.com/me/demo.git", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, "me/missing", ""); err == nil {
		t.Error("added a repository that does not exist")
	}

	list, err := s.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	if a := list[0]; a.Latest != "1.0.0" || a.Installed != "" || a.UpdateAvailable || a.Asset != "Demo-1.0.0-arm64.dmg" {
		t.Errorf("before install: %+v", a)
	}

	st, err := s.Install(ctx, "me/demo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Installed != "1.0.0" || fi.installed["Demo-1.0.0-arm64.dmg"] != "contents of Demo-1.0.0-arm64.dmg" {
		t.Errorf("install: %+v %v", st, fi.installed)
	}
	cfg, _ := config.Load(s.ConfigPath)
	if cfg.Find("me/demo").AppName != "Demo" {
		t.Error("app name not recorded in the shared configuration")
	}

	// switching to release candidates offers the update
	if err := s.SetChannel("me/demo", "prerelease"); err != nil {
		t.Fatal(err)
	}
	updates, err := s.Updates(ctx)
	if err != nil || len(updates) != 1 || updates[0].Latest != "1.1.0-rc.1" {
		t.Fatalf("updates: %+v %v", updates, err)
	}

	// another computer: shared config, empty local state, copy installed by hand
	other := &Store{
		ConfigPath: s.ConfigPath, StatePath: filepath.Join(t.TempDir(), "state.json"),
		GitHub: s.GitHub, Installer: s.Installer, GOOS: s.GOOS, GOARCH: s.GOARCH,
	}
	fi.foreign["Demo"] = "0.9.0"
	list, _ = other.List(ctx)
	if a := list[0]; a.Installed != "0.9.0" || a.ManagedByLieferling || !a.UpdateAvailable {
		t.Errorf("other computer: %+v", a)
	}

	// opening: the copy Lieferling installed, and the one found by name
	if err := s.Open(ctx, "me/demo"); err != nil {
		t.Fatal(err)
	}
	if err := other.Open(ctx, "me/demo"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"/Applications/Demo.app", "/Applications/Demo.app"}; strings.Join(fi.opened, ",") != strings.Join(want, ",") {
		t.Errorf("opened %v, want %v", fi.opened, want)
	}
	delete(fi.foreign, "Demo")
	if err := other.Open(ctx, "me/demo"); err != install.ErrNotInstalled {
		t.Errorf("opening an app that is not installed: %v", err)
	}

	if err := s.Remove(ctx, "me/demo", true); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(ctx); len(list) != 0 {
		t.Errorf("after remove: %+v", list)
	}
}

func TestNormalizeRepo(t *testing.T) {
	for in, want := range map[string]string{
		"owner/name":                                "owner/name",
		"https://github.com/owner/name":             "owner/name",
		"https://github.com/owner/name.git":         "owner/name",
		"github.com/owner/name/releases/tag/v1.0.0": "owner/name",
		"git@github.com:owner/name.git":             "owner/name",
	} {
		if got, err := NormalizeRepo(in); err != nil || got != want {
			t.Errorf("NormalizeRepo(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "owner", "https://example.com"} {
		if _, err := NormalizeRepo(bad); err == nil {
			t.Errorf("NormalizeRepo(%q) accepted", bad)
		}
	}
}
