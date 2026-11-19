package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandStatus(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "Lieferling.app")
	helpers := filepath.Join(bundle, "Contents", "Helpers")
	if err := os.MkdirAll(helpers, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := BundledCommand(bundle); got != "" {
		t.Fatalf("BundledCommand without a command = %q", got)
	}
	bundled := filepath.Join(helpers, "lieferling")
	if err := os.WriteFile(bundled, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := BundledCommand(bundle); got != bundled {
		t.Fatalf("BundledCommand = %q, want %q", got, bundled)
	}

	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bin, "lieferling")

	check := func(name string, bundled string, want CommandState, other string) {
		t.Helper()
		c := CommandStatus(bundled, link)
		if c.State != want || c.Other != other || c.Path != link {
			t.Errorf("%s: got %+v, want state %s, other %q", name, c, want, other)
		}
	}
	check("nothing there", bundled, CommandMissing, "")
	if got, want := CommandStatus(bundled, link).Manual, "sudo mkdir -p '"+bin+"' && sudo ln -sf '"+bundled+"' '"+link+"'"; got != want {
		t.Errorf("Manual = %q, want %q", got, want)
	}
	if got := CommandStatus("", link).Manual; got != "" {
		t.Errorf("Manual without a command = %q", got)
	}
	check("dev build", "", CommandUnavailable, "")

	if err := os.Symlink(bundled, link); err != nil {
		t.Fatal(err)
	}
	check("linked", bundled, CommandLinked, "")

	elsewhere := filepath.Join(dir, "Old.app", "lieferling")
	os.Remove(link)
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	check("dangling link elsewhere", bundled, CommandOther, elsewhere)

	os.Remove(link)
	if err := os.WriteFile(link, []byte("copy"), 0o755); err != nil {
		t.Fatal(err)
	}
	check("copy from a release", bundled, CommandOther, "")
	check("copy, dev build", "", CommandUnavailable, "")
}
