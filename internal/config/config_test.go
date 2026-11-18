package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateMergesICloudConflictCopies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := Update(path, func(c *Config) error {
		c.Apps = append(c.Apps, App{Repo: "a/one", Channel: "stable"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// another Mac added an app at the same time; iCloud kept both files
	conflict := filepath.Join(dir, "config 2.json")
	if err := os.WriteFile(conflict, []byte(`{"version":1,"apps":[{"repo":"b/two","channel":"prerelease"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Apps) != 2 || cfg.Find("B/Two") == nil {
		t.Fatalf("conflict copy not merged: %+v", cfg.Apps)
	}

	if err := Update(path, func(c *Config) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(conflict); !os.IsNotExist(err) {
		t.Error("conflict copy still there after an update")
	}
	cfg, _ = Load(path)
	if len(cfg.Apps) != 2 || cfg.Apps[0].Repo != "a/one" {
		t.Errorf("after update: %+v", cfg.Apps)
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || len(cfg.Apps) != 0 || cfg.Interval() != 6 {
		t.Fatalf("got %+v, %v", cfg, err)
	}
}
