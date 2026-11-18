package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseForms(t *testing.T) {
	for name, tc := range map[string]struct {
		in   string
		want []Entry
		cat  string
	}{
		"catalog": {
			in:   `{"lieferling":"catalog","version":1,"name":" Team ","apps":[{"repo":"a/one","channel":"prerelease","appName":"One"}]}`,
			want: []Entry{{Repo: "a/one", Channel: "prerelease", AppName: "One"}},
			cat:  "Team",
		},
		"config.json": {
			in:   `{"version":1,"apps":[{"repo":"a/one","channel":"stable"}],"checkIntervalHours":3}`,
			want: []Entry{{Repo: "a/one", Channel: "stable"}},
		},
		"array of strings and objects": {
			in:   `["a/one", {"repo":"b/two"}, ""]`,
			want: []Entry{{Repo: "a/one"}, {Repo: "b/two"}},
		},
		"text": {
			in:   "\xef\xbb\xbf# my apps\na/one\n- https://github.com/b/two  prerelease # nightly\n\n",
			want: []Entry{{Repo: "a/one"}, {Repo: "https://github.com/b/two", Channel: "prerelease"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := Parse([]byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if c.Name != tc.cat || c.Lieferling != Kind || len(c.Apps) != len(tc.want) {
				t.Fatalf("got %+v", c)
			}
			for i := range tc.want {
				if c.Apps[i] != tc.want[i] {
					t.Errorf("app %d: %+v, want %+v", i, c.Apps[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"", "  \n# nothing\n", `{"name":"x"}`, `{"apps":"a/one"}`, `[1, 2]`, "<!DOCTYPE html><html>", `{"apps":[]}`} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) accepted", in)
		}
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	data, err := New([]Entry{{Repo: "a/one", Channel: "stable"}}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"lieferling": "catalog"`) {
		t.Errorf("no marker: %s", data)
	}
	c, err := Parse(data)
	if err != nil || len(c.Apps) != 1 || c.Apps[0].Repo != "a/one" {
		t.Errorf("round trip: %+v %v", c, err)
	}
	empty, _ := New(nil).Encode()
	if !strings.Contains(string(empty), `"apps": []`) {
		t.Errorf("empty list written as %s", empty)
	}
}

func TestFetchURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/me/apps/blob/main/lists/team.json": "https://raw.githubusercontent.com/me/apps/main/lists/team.json",
		"https://github.com/me/apps/raw/main/team.txt":         "https://raw.githubusercontent.com/me/apps/main/team.txt",
		"https://example.com/apps.json?x=1":                    "https://example.com/apps.json?x=1",
	} {
		if got, err := FetchURL(in); err != nil || got != want {
			t.Errorf("FetchURL(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"http://example.com/a.json", "file:///etc/passwd", "a/one", ""} {
		if _, err := FetchURL(bad); err == nil {
			t.Errorf("FetchURL(%q) accepted", bad)
		}
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.Write([]byte("a/one\nb/two\n"))
		case "/big":
			w.Write([]byte(strings.Repeat("a/one\n", MaxSize/6+10)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	c, err := Fetch(ctx, srv.Client(), srv.URL+"/ok")
	if err != nil || len(c.Apps) != 2 {
		t.Fatalf("ok: %+v %v", c, err)
	}
	if _, err := Fetch(ctx, srv.Client(), srv.URL+"/missing"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("missing: %v", err)
	}
	if _, err := Fetch(ctx, srv.Client(), srv.URL+"/big"); err == nil {
		t.Error("read a catalog over the size limit")
	}
}
