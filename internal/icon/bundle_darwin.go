package icon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FromBundle reads the icon of the .app at path: the .icns its Info.plist
// names, converted by sips when it only has the old formats, else the
// Finder's own rendering through Quick Look (for apps that keep their icon
// in an asset catalogue only).
func FromBundle(ctx context.Context, app string) (Icon, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res := filepath.Join(app, "Contents", "Resources")
	name, _ := exec.CommandContext(ctx, "plutil", "-extract", "CFBundleIconFile", "raw",
		filepath.Join(app, "Contents", "Info.plist")).Output()
	if n := strings.TrimSpace(string(name)); n != "" {
		if filepath.Ext(n) == "" {
			n += ".icns"
		}
		file := filepath.Join(res, filepath.Base(n))
		if data, err := os.ReadFile(file); err == nil {
			if ic, err := Normalize(data, file); err == nil {
				return ic, nil
			}
		}
		if ic, err := convert(ctx, file); err == nil {
			return ic, nil
		}
	}
	return quickLook(ctx, app)
}

func convert(ctx context.Context, file string) (Icon, error) {
	dir, err := os.MkdirTemp("", "lieferling-icon-")
	if err != nil {
		return Icon{}, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "icon.png")
	if err := exec.CommandContext(ctx, "sips", "-s", "format", "png", "-Z", "128", file, "--out", out).Run(); err != nil {
		return Icon{}, ErrNone
	}
	return readPNG(out)
}

func quickLook(ctx context.Context, app string) (Icon, error) {
	dir, err := os.MkdirTemp("", "lieferling-icon-")
	if err != nil {
		return Icon{}, err
	}
	defer os.RemoveAll(dir)
	if err := exec.CommandContext(ctx, "qlmanage", "-t", "-s", "128", "-o", dir, app).Run(); err != nil {
		return Icon{}, ErrNone
	}
	return readPNG(filepath.Join(dir, filepath.Base(app)+".png"))
}

func readPNG(file string) (Icon, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Icon{}, ErrNone
	}
	return Normalize(data, file)
}
