package store

import (
	"path/filepath"
	"testing"

	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/state"
)

func TestMigrateFromElappma(t *testing.T) {
	dir := t.TempDir()
	oldCfg, newCfg := filepath.Join(dir, "Elappma", "config.json"), filepath.Join(dir, "Lieferling", "config.json")
	oldSt, newSt := filepath.Join(dir, "old", "state.json"), filepath.Join(dir, "new", "state.json")

	if err := config.Update(oldCfg, func(c *config.Config) error {
		c.Apps = []config.App{
			{Repo: "pklaschka/elappma", Channel: "stable", AppName: "Elappma"},
			{Repo: "pklaschka/feldspur", Channel: "prerelease", AppName: "Feldspur"},
		}
		c.CheckIntervalHours = 3
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	st := state.State{}
	st.Set("pklaschka/elappma", state.Installed{Version: "0.2.1", Path: "/Applications/Elappma.app"})
	st.Set("pklaschka/feldspur", state.Installed{Version: "1.4.0"})
	if err := state.Save(oldSt, st); err != nil {
		t.Fatal(err)
	}

	if err := MigrateFromElappma(newCfg, oldCfg, newSt, oldSt); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(newCfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Apps) != 2 || cfg.CheckIntervalHours != 3 {
		t.Fatalf("config not copied: %+v", cfg)
	}
	if a := cfg.Find("pklaschka/lieferling"); a == nil || a.AppName != "Lieferling" {
		t.Errorf("own entry not renamed: %+v", cfg.Apps)
	}
	if a := cfg.Find("pklaschka/feldspur"); a == nil || a.Channel != "prerelease" {
		t.Errorf("other entry changed: %+v", cfg.Apps)
	}
	got, err := state.Load(newSt)
	if err != nil {
		t.Fatal(err)
	}
	if in, ok := got.Get("pklaschka/lieferling"); !ok || in.Version != "0.2.1" {
		t.Errorf("own state not moved: %+v", got)
	}
	if _, ok := got.Get("pklaschka/elappma"); ok {
		t.Error("old key left behind")
	}

	// a second run leaves the new files alone
	if err := config.Update(newCfg, func(c *config.Config) error { c.Apps = c.Apps[:1]; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := MigrateFromElappma(newCfg, oldCfg, newSt, oldSt); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := config.Load(newCfg); len(cfg.Apps) != 1 {
		t.Errorf("migration ran twice: %+v", cfg.Apps)
	}
}
