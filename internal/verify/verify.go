// Package verify decides whether a downloaded package may be installed: the
// customs check ("Zollkontrolle"). It compares the download's SHA-256 with
// what the forge, a checksum file in the release and the other computers saw,
// and the package's sender with the one known so far. It only decides; the
// store gathers what it needs and the installer reads the bundle.
//
// See docs/adr/0002-customs-check-before-installing.md.
package verify

import (
	"errors"
	"strings"
	"time"
)

// Policy is how strict the customs check is.
type Policy string

const (
	Standard Policy = "standard" // check what is available; a sender change asks
	Strict   Policy = "strict"   // a published checksum is mandatory; a sender change holds
	Off      Policy = "off"      // no check; valid only as a per-app exception
)

// Resolve gives the policy for one app. app (config.App.Verify) wins when
// set. global (config.Config.Verify) is used otherwise; "" and "off" as global
// mean Standard, because off exists only per app. Any unknown value means
// Strict: a typo or a level from a newer version must not weaken the check.
func Resolve(global, app string) Policy {
	if app != "" {
		return parse(app)
	}
	switch p := parse(global); p {
	case Off:
		return Standard
	default:
		return p
	}
}

func parse(s string) Policy {
	switch Policy(strings.ToLower(strings.TrimSpace(s))) {
	case "", Standard:
		return Standard
	case Off:
		return Off
	}
	return Strict
}

// rank orders config values for Stricter: off < standard < "" < strict.
func rank(s string) int {
	if strings.TrimSpace(s) == "" {
		return 2
	}
	switch parse(s) {
	case Off:
		return 0
	case Standard:
		return 1
	}
	return 3
}

// Stricter returns the stricter of two config values, in the order off <
// standard < "" < strict; unknown counts as strict. "" ranks above standard
// because an app without an exception follows the global policy, which is
// standard or strict. Used when merging conflict copies.
func Stricter(a, b string) string {
	if rank(b) > rank(a) {
		return b
	}
	return a
}

// Result is the outcome of one check.
type Result string

const (
	Pass Result = "pass"
	Fail Result = "fail"
	Skip Result = "skip" // not checkable: nothing to compare with
)

// Check IDs. Delivery 2 adds "attestation", delivery 3 "signature" and
// "notarization".
const (
	CheckForgeDigest = "digest.forge"
	CheckChecksums   = "digest.checksums"
	CheckSynced      = "digest.synced"
	CheckSender      = "sender"
	CheckPolicy      = "policy" // a requirement of strict that nothing met
)

// Check is one line of the report. ID, Result and Code are stable; Text is
// filled by Localize and never stored.
type Check struct {
	ID     string `json:"id"`
	Result Result `json:"result"`
	Code   string `json:"code"`             // why, see text.go
	Source string `json:"source,omitempty"` // "GitHub", "codeberg.org", "SHA256SUMS"
	Want   string `json:"want,omitempty"`   // what was expected
	Got    string `json:"got,omitempty"`    // what was found
	Text   string `json:"text"`             // one sentence, see Localize
}

// Verdict is what the check decided about a package.
type Verdict string

const (
	Cleared   Verdict = "cleared"   // at least one check passed, none failed
	Unchecked Verdict = "unchecked" // nothing failed, but there was nothing to compare with
	Ask       Verdict = "ask"       // only the sender changed, policy standard: the user decides
	Held      Verdict = "held"      // not installed
	Disputed  Verdict = "disputed"  // installed, but another computer saw a different package for this version
	Exempt    Verdict = "exempt"    // policy off for this app
)

// Stopped reports whether v keeps a package out or needs attention: held,
// ask and disputed.
func (v Verdict) Stopped() bool { return v == Held || v == Ask || v == Disputed }

// Sender is who a package claims to come from.
type Sender struct {
	BundleID string `json:"bundleId"`
	TeamID   string `json:"teamId,omitempty"` // "" for unsigned and ad-hoc signed apps
}

// IsZero reports whether nothing is known about the sender.
func (s Sender) IsZero() bool { return s.BundleID == "" && s.TeamID == "" }

// Equal compares the bundle ID without case and the team ID exactly.
func (s Sender) Equal(o Sender) bool {
	return strings.EqualFold(s.BundleID, o.BundleID) && s.TeamID == o.TeamID
}

// String is "com.example.app (team ABCDE12345)" or "com.example.app (no team ID)".
func (s Sender) String() string { return s.text(English) }

func (s Sender) text(lang Lang) string {
	id := s.BundleID
	if id == "" {
		id = map[Lang]string{English: "no bundle ID", German: "ohne Bundle-ID"}[lang]
	}
	if s.TeamID == "" {
		return id + map[Lang]string{English: " (no team ID)", German: " (ohne Team-ID)"}[lang]
	}
	return id + map[Lang]string{English: " (team ", German: " (Team-ID "}[lang] + s.TeamID + ")"
}

// Report is the customs note for one package.
type Report struct {
	Version   string    `json:"version"`
	Asset     string    `json:"asset"`
	Digest    string    `json:"digest"` // "sha256:<64 lowercase hex>", computed while downloading
	Policy    Policy    `json:"policy"`
	Verdict   Verdict   `json:"verdict"`
	Checks    []Check   `json:"checks"`
	Before    *Sender   `json:"before,omitempty"` // set when the sender was compared and differs
	After     *Sender   `json:"after,omitempty"`
	// Sender is who the package says it comes from, when it could be read.
	Sender *Sender `json:"sender,omitempty"`
	CheckedAt time.Time `json:"checkedAt"`
	Summary   string    `json:"summary"` // the collapsed line, filled by Localize, never stored
}

// Failed reports whether any check failed.
func (r Report) Failed() bool {
	for _, c := range r.Checks {
		if c.Result == Fail {
			return true
		}
	}
	return false
}

// FirstFailure is the first failing check, or nil.
func (r *Report) FirstFailure() *Check {
	for i := range r.Checks {
		if r.Checks[i].Result == Fail {
			return &r.Checks[i]
		}
	}
	return nil
}

// Stored returns a copy without the localized text, as it is written to disk.
func (r Report) Stored() Report {
	r.Summary = ""
	r.Checks = append([]Check(nil), r.Checks...)
	for i := range r.Checks {
		r.Checks[i].Text = ""
	}
	return r
}

// Decide turns checks into a verdict. It may append a CheckPolicy line.
//
//	any digest.* fail                         -> Held
//	strict and neither digest.forge nor
//	  digest.checksums passed                 -> append {policy, fail, "checksum_required"}, Held
//	sender fail, standard                     -> Ask
//	sender fail, strict                       -> Held
//	otherwise: at least one pass              -> Cleared, else Unchecked
func Decide(p Policy, checks []Check) (Verdict, []Check) {
	var digestFail, senderFail, published, anyPass bool
	for _, c := range checks {
		switch {
		case c.Result == Fail && strings.HasPrefix(c.ID, "digest."):
			digestFail = true
		case c.Result == Fail && c.ID == CheckSender:
			senderFail = true
		case c.Result == Fail:
			digestFail = true // any other failing check holds as well
		}
		if c.Result == Pass {
			anyPass = true
			if c.ID == CheckForgeDigest || c.ID == CheckChecksums {
				published = true
			}
		}
	}
	if digestFail {
		return Held, checks
	}
	if p == Strict && !published {
		checks = append(checks, Check{ID: CheckPolicy, Result: Fail, Code: "checksum_required"})
		return Held, checks
	}
	if senderFail {
		if p == Strict {
			return Held, checks
		}
		return Ask, checks
	}
	if anyPass {
		return Cleared, checks
	}
	return Unchecked, checks
}

var (
	// ErrHeld means a package was stopped at customs and not installed.
	ErrHeld = errors.New("held at customs")
	// ErrAsk means only the sender changed and the user has to confirm it.
	ErrAsk = errors.New("the sender changed and needs your confirmation")
)
