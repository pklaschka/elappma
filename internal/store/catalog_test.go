package store

import (
	"context"
	"testing"

	"github.com/pklaschka/lieferling/internal/catalog"
	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/forge"
)

func TestExportImport(t *testing.T) {
	s, _ := newTestStore(t, map[string][]forge.Release{})
	err := config.Update(s.ConfigPath, func(c *config.Config) error {
		c.Apps = []config.App{
			{Repo: "me/demo", Channel: "prerelease", AppName: "Demo"},
			{Repo: "me/old", Channel: ""},
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Apps) != 2 || out.Apps[0] != (catalog.Entry{Repo: "me/demo", Channel: "prerelease", AppName: "Demo"}) || out.Apps[1].Channel != "stable" {
		t.Fatalf("export: %+v", out.Apps)
	}

	in := catalog.New([]catalog.Entry{
		{Repo: "ME/Demo", Channel: "stable"},                        // on the list: left alone
		{Repo: "https://github.com/team/tool", Channel: "Beta"},     // new, prerelease
		{Repo: "team/tool"},                                         // duplicate
		{Repo: "nonsense"},                                          // invalid
		{Repo: "team/other", Channel: "nightly"},                    // unknown channel
		{Repo: "team/sneaky", AppName: "../../../System/Something"}, // path in the name
	})
	plan, err := s.PlanImport(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 5 {
		t.Fatalf("plan: %+v", plan.Items)
	}
	if it := plan.Items[0]; !it.OnList || it.Repo != "me/demo" {
		t.Errorf("existing: %+v", it)
	}
	if it := plan.Items[1]; it.OnList || it.Repo != "team/tool" || it.Channel != "prerelease" || it.Error != "" {
		t.Errorf("new: %+v", it)
	}
	if plan.Items[2].Error == "" || plan.Items[3].Error == "" {
		t.Errorf("invalid entries pass: %+v", plan.Items[2:4])
	}
	if it := plan.Items[4]; it.AppName != "" || it.Error != "" {
		t.Errorf("app name with a path kept: %+v", it)
	}
	fresh := plan.New()
	if len(fresh) != 2 {
		t.Fatalf("new items: %+v", fresh)
	}

	added, err := s.Import(plan.Items) // all of them: existing and invalid are skipped
	if err != nil || added != 2 {
		t.Fatalf("import: %d %v", added, err)
	}
	cfg, _ := config.Load(s.ConfigPath)
	if len(cfg.Apps) != 4 || cfg.Find("me/demo").Channel != "prerelease" || cfg.Find("team/tool").Channel != "prerelease" {
		t.Errorf("after import: %+v", cfg.Apps)
	}
	if added, _ := s.Import(fresh); added != 0 {
		t.Errorf("second import added %d", added)
	}

	// importing into nothing works too, and does not ask GitHub
	if _, err := s.List(context.Background()); err != nil {
		t.Fatal(err)
	}
}
