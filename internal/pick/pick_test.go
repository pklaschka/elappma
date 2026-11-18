package pick

import (
	"testing"

	"github.com/pklaschka/lieferling/internal/forge"
)

func assets(names ...string) []forge.Asset {
	out := make([]forge.Asset, len(names))
	for i, n := range names {
		out[i] = forge.Asset{Name: n}
	}
	return out
}

func TestAsset(t *testing.T) {
	// as electron-builder and electron-forge name them
	builder := assets(
		"Feldspur-1.4.0-arm64.dmg", "Feldspur-1.4.0-arm64.dmg.blockmap",
		"Feldspur-1.4.0.dmg", "Feldspur-1.4.0-arm64-mac.zip", "Feldspur-1.4.0-mac.zip",
		"Feldspur-1.4.0.AppImage", "Feldspur-1.4.0-arm64.AppImage",
		"Feldspur Setup 1.4.0.exe", "Feldspur Setup 1.4.0.exe.blockmap",
		"latest-mac.yml", "latest.yml",
	)
	argmap := assets("therian.blog.Argmap-4.0.0-arm64.dmg", "therian.blog.Argmap-4.0.0-x64.dmg", "therian.blog.Argmap-4.0.0-win-x64.exe")
	universal := assets("App-2.0.0-universal.dmg", "App-2.0.0-linux-x86_64.AppImage")
	intelOnly := assets("Old-1.0.0-x64.dmg", "Old-1.0.0-x64.zip")

	cases := []struct {
		name       string
		assets     []forge.Asset
		goos, arch string
		want       string
	}{
		{"builder mac arm", builder, "darwin", "arm64", "Feldspur-1.4.0-arm64.dmg"},
		{"builder mac intel", builder, "darwin", "amd64", "Feldspur-1.4.0.dmg"},
		{"builder linux", builder, "linux", "amd64", "Feldspur-1.4.0.AppImage"},
		{"builder linux arm", builder, "linux", "arm64", "Feldspur-1.4.0-arm64.AppImage"},
		{"builder windows", builder, "windows", "amd64", "Feldspur Setup 1.4.0.exe"},
		{"argmap arm", argmap, "darwin", "arm64", "therian.blog.Argmap-4.0.0-arm64.dmg"},
		{"argmap intel", argmap, "darwin", "amd64", "therian.blog.Argmap-4.0.0-x64.dmg"},
		{"universal", universal, "darwin", "arm64", "App-2.0.0-universal.dmg"},
		{"linux x86_64", universal, "linux", "amd64", "App-2.0.0-linux-x86_64.AppImage"},
		{"rosetta fallback", intelOnly, "darwin", "arm64", "Old-1.0.0-x64.dmg"},
		{"no arm linux build", universal, "linux", "arm64", ""},
		{"nothing for windows", universal, "windows", "amd64", ""},
	}
	for _, c := range cases {
		got, ok := Asset(c.assets, c.goos, c.arch)
		if c.want == "" {
			if ok {
				t.Errorf("%s: got %s, want none", c.name, got.Name)
			}
			continue
		}
		if got.Name != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got.Name, c.want)
		}
	}
}

func TestRelease(t *testing.T) {
	rs := []forge.Release{
		{TagName: "v2.0.0", Draft: true},
		{TagName: "v1.1.0-rc.2", Prerelease: true},
		{TagName: "v1.0.0"},
	}
	if r, _ := Release(rs, Stable); r.TagName != "v1.0.0" {
		t.Errorf("stable: got %s", r.TagName)
	}
	if r, _ := Release(rs, Prerelease); r.TagName != "v1.1.0-rc.2" {
		t.Errorf("prerelease: got %s", r.TagName)
	}
	if _, ok := Release(rs[:2], Stable); ok {
		t.Error("stable: found a release among a draft and a release candidate")
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.1.0-rc.1", false}, // switching back to stable is no update
		{"1.1.0", "1.1.0-rc.15", true},
		{"1.1.0-rc.15", "1.1.0-rc.2", true},
		{"v2.0.0", "1.9.9", true},
		{"nightly-2", "nightly-1", true},
		{"", "1.0.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
