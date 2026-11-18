package forge

import "testing"

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

func TestTokenVariable(t *testing.T) {
	if got := TokenVariable("git.example.com:3000"); got != "LIEFERLING_TOKEN_GIT_EXAMPLE_COM_3000" {
		t.Error(got)
	}
}
