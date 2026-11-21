package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pklaschka/lieferling/internal/verify"
)

func TestHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	var s State
	r := verify.Report{Version: "1.0", Verdict: verify.Held, Summary: "text", Checks: []verify.Check{{ID: verify.CheckSynced, Result: verify.Fail, Code: "mismatch", Text: "text"}}}
	s.Hold("Owner/App", r)
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.HeldFor("owner/app")
	if !ok || got.Version != "1.0" || got.Verdict != verify.Held || len(got.Checks) != 1 {
		t.Fatalf("HeldFor = %+v, %v", got, ok)
	}
	if got.Summary != "" || got.Checks[0].Text != "" {
		t.Errorf("text was stored: %+v", got)
	}
	s.Set("owner/app", Installed{Version: "0.9"})
	s.Delete("owner/app")
	if _, ok := s.HeldFor("owner/app"); ok {
		t.Error("Delete kept the held report")
	}
}

func TestOldState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"apps":{"a/b":{"version":"1.0","path":"/Applications/B.app"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if in, ok := s.Get("a/b"); !ok || in.Customs != nil {
		t.Errorf("old state: %+v %v", in, ok)
	}
	if _, ok := s.HeldFor("a/b"); ok {
		t.Error("held in old state")
	}
}
