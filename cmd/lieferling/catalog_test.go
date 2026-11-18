package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pklaschka/lieferling/internal/store"
)

func TestExportImportCommands(t *testing.T) {
	dir := t.TempDir()
	src := &store.Store{ConfigPath: filepath.Join(dir, "a", "config.json"), StatePath: filepath.Join(dir, "a", "state.json")}
	dst := &store.Store{ConfigPath: filepath.Join(dir, "b", "config.json"), StatePath: filepath.Join(dir, "b", "state.json")}
	ctx := context.Background()
	if err := os.MkdirAll(filepath.Dir(src.ConfigPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src.ConfigPath, []byte(`{"version":1,"apps":[{"repo":"me/one","channel":"prerelease"},{"repo":"me/two","channel":"stable"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := run(ctx, src, []string{"export"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"repo": "me/one"`) {
		t.Fatalf("export to stdout: %s", out.String())
	}
	file := filepath.Join(dir, "team.json")
	out.Reset()
	if err := run(ctx, src, []string{"export", file}, &out); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := run(ctx, dst, []string{"import", file, "--dry-run"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Would add 2") {
		t.Errorf("dry run: %s", out.String())
	}
	if cfg, _ := dst.Config(); len(cfg.Apps) != 0 {
		t.Fatalf("dry run changed the list: %+v", cfg.Apps)
	}

	stdin = strings.NewReader("me/two\nme/three\n")
	t.Cleanup(func() { stdin = os.Stdin })
	out.Reset()
	if err := run(ctx, dst, []string{"import", "-"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(ctx, dst, []string{"import", file}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Added 1, 1 already on the list") {
		t.Errorf("import: %s", out.String())
	}
	cfg, _ := dst.Config()
	if len(cfg.Apps) != 3 || cfg.Find("me/one").Channel != "prerelease" {
		t.Errorf("after import: %+v", cfg.Apps)
	}

	if err := run(ctx, dst, []string{"import", filepath.Join(dir, "missing.json")}, &out); err == nil {
		t.Error("imported a file that does not exist")
	}
}
