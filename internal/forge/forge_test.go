package forge

import (
	"encoding/json"
	"testing"
)

func TestParseRepo(t *testing.T) {
	for in, want := range map[string]Repo{
		"owner/name":                                {Owner: "owner", Name: "name"},
		"https://github.com/owner/name/releases":    {Owner: "owner", Name: "name"},
		"codeberg.org/me/app":                       {Host: "codeberg.org", Owner: "me", Name: "app"},
		"https://codeberg.org/me/app.git?tab=x":     {Host: "codeberg.org", Owner: "me", Name: "app"},
		"git@git.example.com:me/app.git":            {Host: "git.example.com", Owner: "me", Name: "app"},
		"ssh://git@git.example.com:2222/me/app.git": {Host: "git.example.com", Owner: "me", Name: "app"},
		"localhost:3000/me/app":                     {Host: "localhost:3000", Owner: "me", Name: "app"},
	} {
		if got, err := ParseRepo(in); err != nil || got != want {
			t.Errorf("ParseRepo(%q) = %+v, %v", in, got, err)
		}
	}
}

func TestForgeName(t *testing.T) {
	for repo, want := range map[string]string{
		"owner/name": "GitHub", "codeberg.org/o/n": "Codeberg", "git.example.com/o/n": "git.example.com",
	} {
		r, _ := ParseRepo(repo)
		if r.Forge() != want {
			t.Errorf("%s: %s, want %s", repo, r.Forge(), want)
		}
	}
}

func TestWebURL(t *testing.T) {
	for repo, want := range map[string]string{
		"owner/name":               "https://github.com/owner/name",
		"codeberg.org/o/n":         "https://codeberg.org/o/n",
		"git.example.com:3000/o/n": "https://git.example.com:3000/o/n",
	} {
		r, err := ParseRepo(repo)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.WebURL(); got != want {
			t.Errorf("%s: %s, want %s", repo, got, want)
		}
	}
}

func TestTokenVariable(t *testing.T) {
	if got := TokenVariable("git.example.com:3000"); got != "LIEFERLING_TOKEN_GIT_EXAMPLE_COM_3000" {
		t.Error(got)
	}
}

func TestAssetDigest(t *testing.T) {
	var rels []Release
	data := `[{"tag_name":"v1","assets":[{"name":"a.dmg","digest":"sha256:abc"},{"name":"b.dmg","digest":null}]}]`
	if err := json.Unmarshal([]byte(data), &rels); err != nil {
		t.Fatal(err)
	}
	if got := rels[0].Assets[0].Digest; got != "sha256:abc" {
		t.Errorf("digest = %q", got)
	}
	if got := rels[0].Assets[1].Digest; got != "" {
		t.Errorf("null digest = %q", got)
	}
	var g giteaRelease
	if err := json.Unmarshal([]byte(`{"tag_name":"v1","assets":[{"name":"a.dmg","browser_download_url":"x"}]}`), &g); err != nil {
		t.Fatal(err)
	}
	if got := g.release().Assets[0].Digest; got != "" {
		t.Errorf("gitea digest = %q", got)
	}
}
