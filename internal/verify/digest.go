package verify

import (
	"encoding/hex"
	"sort"
	"strings"
)

// FormatDigest is "sha256:" + lowercase hex of sum.
func FormatDigest(sum []byte) string { return "sha256:" + hex.EncodeToString(sum) }

// Key is the key of config.App.Digests: version + "/" + asset.
func Key(version, asset string) string { return version + "/" + asset }

// SplitKey cuts a key at the last "/", because a tag can hold a slash and an
// asset name cannot.
func SplitKey(key string) (version, asset string) {
	i := strings.LastIndex(key, "/")
	if i < 0 {
		return key, ""
	}
	return key[:i], key[i+1:]
}

// Seen splits a synced value into its digests. One digest is the normal case;
// more than one means the computers disagree.
func Seen(value string) []string { return strings.Fields(value) }

// Merge is the union of two synced values, sorted, joined by one space.
func Merge(a, b string) string {
	set := map[string]bool{}
	for _, d := range append(Seen(a), Seen(b)...) {
		set[normalize(d)] = true
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// normalize lowercases the hex of a "sha256:" digest.
func normalize(d string) string {
	if algo, hexPart, ok := strings.Cut(d, ":"); ok {
		return strings.ToLower(algo) + ":" + strings.ToLower(hexPart)
	}
	return strings.ToLower(d)
}

// isSHA256 reports whether d is "sha256:" and 64 hex digits.
func isSHA256(d string) bool {
	algo, h, ok := strings.Cut(d, ":")
	return ok && strings.EqualFold(algo, "sha256") && isHex64(h)
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func sameDigest(a, b string) bool { return normalize(a) == normalize(b) }

// ForgeDigest compares the digest the forge lists for the asset with got.
//
//	listed == ""                -> skip "none"
//	listed not "sha256:<64hex>" -> skip "algorithm"
//	equal (hex without case)    -> pass "match"
//	else                        -> fail "mismatch" (Want listed, Got got)
func ForgeDigest(forgeName, listed, got string) Check {
	c := Check{ID: CheckForgeDigest, Source: forgeName, Got: got}
	switch {
	case listed == "":
		c.Result, c.Code = Skip, "none"
	case !isSHA256(listed):
		c.Result, c.Code, c.Want = Skip, "algorithm", listed
	case sameDigest(listed, got):
		c.Result, c.Code, c.Want = Pass, "match", listed
	default:
		c.Result, c.Code, c.Want = Fail, "mismatch", listed
	}
	return c
}

// SyncedDigest compares what the synced list holds for this version and asset.
//
//	seen == ""                       -> skip "first"
//	more than one digest in seen     -> fail "contested"
//	equal                            -> pass "match"
//	else                             -> fail "mismatch"
func SyncedDigest(seen, got string) Check {
	c := Check{ID: CheckSynced, Got: got, Want: seen}
	ds := Seen(seen)
	switch {
	case len(ds) == 0:
		c.Result, c.Code, c.Want = Skip, "first", ""
	case len(ds) > 1:
		c.Result, c.Code = Fail, "contested"
	case sameDigest(ds[0], got):
		c.Result, c.Code = Pass, "match"
	default:
		c.Result, c.Code = Fail, "mismatch"
	}
	return c
}
