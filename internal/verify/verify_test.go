package verify

import (
	"errors"
	"strings"
	"testing"
)

const (
	hexA = "9f2c000000000000000000000000000000000000000000000000000000000e41"
	hexB = "51aa000000000000000000000000000000000000000000000000000000000001"
	digA = "sha256:" + hexA
	digB = "sha256:" + hexB
)

func TestResolve(t *testing.T) {
	for _, c := range []struct {
		global, app string
		want        Policy
	}{
		{"", "", Standard},
		{"standard", "", Standard},
		{"strict", "", Strict},
		{"off", "", Standard}, // off only per app
		{"strict", "off", Off},
		{"strict", "standard", Standard},
		{"", "STRICT", Strict},
		{"paranoid", "", Strict},
		{"", "maybe", Strict},
	} {
		if got := Resolve(c.global, c.app); got != c.want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", c.global, c.app, got, c.want)
		}
	}
}

func TestStricter(t *testing.T) {
	for _, c := range []struct{ a, b, want string }{
		{"off", "standard", "standard"},
		{"standard", "", ""},
		{"", "strict", "strict"},
		{"strict", "off", "strict"},
		{"off", "off", "off"},
		{"weird", "standard", "weird"},
	} {
		if got := Stricter(c.a, c.b); got != c.want {
			t.Errorf("Stricter(%q, %q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestKey(t *testing.T) {
	k := Key("release/1.2", "App.dmg")
	if v, a := SplitKey(k); v != "release/1.2" || a != "App.dmg" {
		t.Errorf("SplitKey(%q) = %q, %q", k, v, a)
	}
}

func TestMerge(t *testing.T) {
	if got := Merge(digA, digB); got != digB+" "+digA {
		t.Errorf("Merge = %q", got)
	}
	if got := Merge(digA, "sha256:"+strings.ToUpper(hexA)); got != digA {
		t.Errorf("Merge of the same digest = %q", got)
	}
	if got := Merge(Merge(digA, digB), digA); got != digB+" "+digA {
		t.Errorf("Merge not idempotent: %q", got)
	}
	if got := Merge("", digA); got != digA {
		t.Errorf("Merge with empty = %q", got)
	}
}

func TestForgeDigest(t *testing.T) {
	for _, c := range []struct {
		listed string
		result Result
		code   string
	}{
		{"", Skip, "none"},
		{"sha512:abcd", Skip, "algorithm"},
		{"sha256:short", Skip, "algorithm"},
		{digA, Pass, "match"},
		{"sha256:" + strings.ToUpper(hexA), Pass, "match"},
		{digB, Fail, "mismatch"},
	} {
		got := ForgeDigest("GitHub", c.listed, digA)
		if got.Result != c.result || got.Code != c.code || got.ID != CheckForgeDigest || got.Source != "GitHub" {
			t.Errorf("ForgeDigest(%q) = %+v, want %s %s", c.listed, got, c.result, c.code)
		}
	}
}

func TestSyncedDigest(t *testing.T) {
	for _, c := range []struct {
		seen   string
		result Result
		code   string
	}{
		{"", Skip, "first"},
		{digA, Pass, "match"},
		{digB, Fail, "mismatch"},
		{digA + " " + digB, Fail, "contested"},
	} {
		got := SyncedDigest(c.seen, digA)
		if got.Result != c.result || got.Code != c.code {
			t.Errorf("SyncedDigest(%q) = %+v, want %s %s", c.seen, got, c.result, c.code)
		}
	}
}

func TestSenderCheck(t *testing.T) {
	a := Sender{BundleID: "de.example.app", TeamID: "ABCDE12345"}
	b := Sender{BundleID: "de.example.app"}
	for _, c := range []struct {
		name       string
		known, now Sender
		accepted   bool
		result     Result
		code       string
	}{
		{"unreadable", Sender{}, Sender{}, false, Skip, "unreadable"},
		{"first", Sender{}, a, false, Skip, "first"},
		{"same", a, a, false, Pass, "same"},
		{"same without case", a, Sender{BundleID: "DE.example.APP", TeamID: "ABCDE12345"}, false, Pass, "same"},
		{"changed", a, b, false, Fail, "changed"},
		{"accepted", a, b, true, Pass, "accepted"},
		{"no bundle id now", a, Sender{}, false, Fail, "changed"},
	} {
		got := SenderCheck(c.known, c.now, c.accepted)
		if got.Result != c.result || got.Code != c.code {
			t.Errorf("%s: %+v, want %s %s", c.name, got, c.result, c.code)
		}
	}
}

func TestDecide(t *testing.T) {
	pass := func(id string) Check { return Check{ID: id, Result: Pass} }
	fail := func(id string) Check { return Check{ID: id, Result: Fail} }
	skip := func(id string) Check { return Check{ID: id, Result: Skip} }
	for _, c := range []struct {
		name   string
		p      Policy
		checks []Check
		want   Verdict
		policy bool // a policy line was added
	}{
		{"all pass", Standard, []Check{pass(CheckForgeDigest), skip(CheckSynced), pass(CheckSender)}, Cleared, false},
		{"nothing to compare", Standard, []Check{skip(CheckForgeDigest), skip(CheckChecksums), skip(CheckSynced), skip(CheckSender)}, Unchecked, false},
		{"digest fails", Standard, []Check{fail(CheckForgeDigest), pass(CheckSender)}, Held, false},
		{"synced fails", Standard, []Check{pass(CheckForgeDigest), fail(CheckSynced)}, Held, false},
		{"sender standard", Standard, []Check{pass(CheckForgeDigest), fail(CheckSender)}, Ask, false},
		{"sender strict", Strict, []Check{pass(CheckChecksums), fail(CheckSender)}, Held, false},
		{"strict without checksum", Strict, []Check{skip(CheckForgeDigest), skip(CheckChecksums), pass(CheckSynced)}, Held, true},
		{"strict with checksum", Strict, []Check{pass(CheckChecksums), pass(CheckSynced)}, Cleared, false},
		{"digest fail beats sender", Standard, []Check{fail(CheckSynced), fail(CheckSender)}, Held, false},
	} {
		v, checks := Decide(c.p, c.checks)
		if v != c.want {
			t.Errorf("%s: verdict %s, want %s", c.name, v, c.want)
		}
		added := len(checks) > len(c.checks) && checks[len(checks)-1].ID == CheckPolicy
		if added != c.policy {
			t.Errorf("%s: policy line added = %v", c.name, added)
		}
	}
}

func TestStopped(t *testing.T) {
	for v, want := range map[Verdict]bool{Held: true, Ask: true, Disputed: true, Cleared: false, Unchecked: false, Exempt: false} {
		if v.Stopped() != want {
			t.Errorf("%s.Stopped() = %v", v, !want)
		}
	}
}

func TestStored(t *testing.T) {
	r := Report{Verdict: Cleared, Checks: []Check{{ID: CheckSender, Result: Pass, Code: "same", Got: "x (no team ID)"}}}
	Localize(&r, German)
	s := r.Stored()
	if s.Summary != "" || s.Checks[0].Text != "" {
		t.Errorf("Stored kept text: %+v", s)
	}
	if r.Checks[0].Text == "" {
		t.Error("Stored changed the original")
	}
}

func TestSenderString(t *testing.T) {
	if got := (Sender{BundleID: "a.b", TeamID: "ABCDE12345"}).String(); got != "a.b (team ABCDE12345)" {
		t.Errorf("String = %q", got)
	}
	r := Report{Verdict: Ask, Version: "1.0", Checks: []Check{SenderCheck(Sender{BundleID: "a.b", TeamID: "ABCDE12345"}, Sender{BundleID: "a.b"}, false)}}
	Localize(&r, German)
	want := "Der Absender hat sich geändert: bisher a.b (Team-ID ABCDE12345), jetzt a.b (ohne Team-ID)."
	if r.Checks[0].Text != want {
		t.Errorf("German sender line = %q, want %q", r.Checks[0].Text, want)
	}
	if r.Summary != "1.0 angehalten: Der Absender hat sich geändert" {
		t.Errorf("summary = %q", r.Summary)
	}
}

// TestWording checks that every outcome the constructors can produce has a
// sentence in both languages, that no placeholder is left, and that no line
// of these checks claims anything Apple did.
func TestWording(t *testing.T) {
	a := Sender{BundleID: "de.example.app", TeamID: "ABCDE12345"}
	var checks []Check
	for _, l := range []string{"", "sha512:x", digA, digB} {
		checks = append(checks, ForgeDigest("GitHub", l, digA))
	}
	for _, s := range []string{"", digA, digB, digA + " " + digB} {
		checks = append(checks, SyncedDigest(s, digA))
	}
	checks = append(checks,
		ChecksumCheck("SHA256SUMS", digA, true, nil, digA),
		ChecksumCheck("SHA256SUMS", digB, true, nil, digA),
		ChecksumCheck("SHA256SUMS", "", false, nil, digA),
		ChecksumCheck("SHA256SUMS", "", false, errors.New("twice"), digA),
		NoChecksums("none", ""),
		NoChecksums("unreadable", "SHA256SUMS"),
		SenderCheck(Sender{}, Sender{}, false),
		SenderCheck(Sender{}, a, false),
		SenderCheck(a, a, false),
		SenderCheck(a, Sender{BundleID: "x"}, false),
		SenderCheck(a, Sender{BundleID: "x"}, true),
	)
	_, withPolicy := Decide(Strict, nil)
	checks = append(checks, withPolicy...)

	seen := map[textKey]bool{}
	for _, lang := range []Lang{English, German} {
		for v := range summaries {
			r := Report{Verdict: v, Version: "1.2.0", Checks: checks}
			Localize(&r, lang)
			if r.Summary == "" || strings.Contains(r.Summary, "{") {
				t.Errorf("summary %s in %d: %q", v, lang, r.Summary)
			}
			for _, c := range r.Checks {
				seen[textKey{c.ID, c.Result, c.Code}] = true
				if c.Text == "" {
					t.Errorf("no sentence for %s %s %s in %d", c.ID, c.Result, c.Code, lang)
				}
				if strings.Contains(c.Text, "{") {
					t.Errorf("placeholder left: %q", c.Text)
				}
				l := strings.ToLower(c.Text)
				for _, w := range []string{"apple", "notar", "gatekeeper"} {
					if strings.Contains(l, w) {
						t.Errorf("%q mentions %s", c.Text, w)
					}
				}
			}
		}
	}
	for k := range sentences {
		if !seen[k] {
			t.Errorf("sentence %v is never produced", k)
		}
	}
}
