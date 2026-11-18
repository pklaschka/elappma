package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "Elappma.app")
	if err := os.Mkdir(app, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.app")
	if err := os.Symlink(app, link); err != nil {
		t.Fatal(err)
	}
	if !SamePath(app, link) || !SamePath(app+"/", app) {
		t.Error("same bundle not recognised")
	}
	if SamePath(app, filepath.Join(dir, "Other.app")) || SamePath(app, "") || SamePath("", "") {
		t.Error("different or empty paths taken as the same")
	}
}
