package deeplink

import "testing"

func TestParse(t *testing.T) {
	for raw, want := range map[string]Link{
		"elappma://add?repo=pklaschka/feldspur":                                    {Repo: "pklaschka/feldspur"},
		"elappma://add?repo=pklaschka/feldspur&install=1":                          {Repo: "pklaschka/feldspur", Install: true},
		"elappma://add?repo=https://github.com/pklaschka/feldspur&prerelease=true": {Repo: "pklaschka/feldspur", Prerelease: true},
		"elappma://add/pklaschka/feldspur":                                         {Repo: "pklaschka/feldspur"},
		"elappma://install/pklaschka/feldspur?channel=prerelease":                  {Repo: "pklaschka/feldspur", Install: true, Prerelease: true},
		"ELAPPMA://Install?repo=pklaschka%2Ffeldspur":                              {Repo: "pklaschka/feldspur", Install: true},
	} {
		got, err := Parse(raw)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", raw, got, err, want)
		}
	}
	for _, raw := range []string{
		"https://github.com/pklaschka/feldspur",
		"elappma://remove?repo=pklaschka/feldspur",
		"elappma://add",
		"elappma://add?repo=not-a-repo",
	} {
		if _, err := Parse(raw); err == nil {
			t.Errorf("Parse(%q) succeeded", raw)
		}
	}
}

func TestURLRoundTrip(t *testing.T) {
	l := Link{Repo: "pklaschka/feldspur", Install: true, Prerelease: true}
	got, err := Parse(l.URL())
	if err != nil || got != l {
		t.Fatalf("round trip of %s = %+v, %v", l.URL(), got, err)
	}
}
