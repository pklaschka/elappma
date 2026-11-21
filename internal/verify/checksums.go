package verify

import (
	"bufio"
	"bytes"
	"fmt"
	"path"
	"strings"
)

// MaxChecksumFile is the most Lieferling reads of a checksum file: 1 MiB.
const MaxChecksumFile = 1 << 20

// ChecksumFiles returns, in the order to try them, the names among
// assetNames that can hold a checksum for asset. All comparisons ignore case.
//
//  1. <asset>.sha256, <asset>.sha256sum          (per-file, may be a bare digest)
//  2. SHA256SUMS, SHA256SUMS.txt
//  3. any name ending in "checksums.txt"         (checksums.txt, app_1.2.0_checksums.txt)
func ChecksumFiles(assetNames []string, asset string) []string {
	var perFile, sums, lists []string
	for _, n := range assetNames {
		l := strings.ToLower(n)
		switch {
		case IsBare(n, asset):
			perFile = append(perFile, n)
		case l == "sha256sums" || l == "sha256sums.txt":
			sums = append(sums, n)
		case strings.HasSuffix(l, "checksums.txt"):
			lists = append(lists, n)
		}
	}
	return append(append(perFile, sums...), lists...)
}

// IsBare reports whether name is a per-file checksum of asset
// (<asset>.sha256 or <asset>.sha256sum).
func IsBare(name, asset string) bool {
	l, a := strings.ToLower(name), strings.ToLower(asset)
	return l == a+".sha256" || l == a+".sha256sum"
}

// ParseChecksums finds asset's SHA-256 in a checksum file. Per line, after
// trimming; empty lines and lines starting with "#" are skipped:
//
//	GNU   "<hex>  <name>" or "<hex> *<name>"   name compared by its base name
//	BSD   "SHA256 (<name>) = <hex>"
//	bare  "<hex>" alone, only when bare is true, and only if no named line matched
//
// Only 64-digit hex counts; other lengths (SHA-512 lines in a mixed file) are
// skipped. A file that names asset twice with different digests is an error.
// found is false when the file does not name asset.
func ParseChecksums(data []byte, asset string, bare bool) (digest string, found bool, err error) {
	var named, alone string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), MaxChecksumFile)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		h, name, ok := parseLine(line)
		if !ok {
			continue
		}
		if name == "" {
			if alone == "" {
				alone = h
			}
			continue
		}
		if !strings.EqualFold(path.Base(strings.ReplaceAll(name, `\`, "/")), asset) {
			continue
		}
		if named != "" && named != h {
			return "", false, fmt.Errorf("%s is listed twice with different checksums", asset)
		}
		named = h
	}
	if err := sc.Err(); err != nil {
		return "", false, err
	}
	switch {
	case named != "":
		return "sha256:" + named, true, nil
	case bare && alone != "":
		return "sha256:" + alone, true, nil
	}
	return "", false, nil
}

// parseLine reads one line; name is "" for a bare digest. h is lowercase.
func parseLine(line string) (h, name string, ok bool) {
	if rest, cut := strings.CutPrefix(line, "SHA256 ("); cut {
		i := strings.LastIndex(rest, ") = ")
		if i < 0 {
			return "", "", false
		}
		h = strings.TrimSpace(rest[i+4:])
		if !isHex64(h) {
			return "", "", false
		}
		return strings.ToLower(h), rest[:i], true
	}
	fields := strings.SplitN(line, " ", 2)
	h = fields[0]
	if !isHex64(h) {
		return "", "", false
	}
	if len(fields) == 1 {
		return strings.ToLower(h), "", true
	}
	name = strings.TrimLeft(fields[1], " ")
	name = strings.TrimPrefix(name, "*")
	if name == "" {
		return strings.ToLower(h), "", true
	}
	return strings.ToLower(h), name, true
}

// ChecksumCheck is the check for one parsed file.
//
//	err != nil  -> fail "ambiguous"
//	!found      -> skip "unlisted"
//	equal       -> pass "match"
//	else        -> fail "mismatch"
func ChecksumCheck(file, digest string, found bool, err error, got string) Check {
	c := Check{ID: CheckChecksums, Source: file, Got: got, Want: digest}
	switch {
	case err != nil:
		c.Result, c.Code, c.Want = Fail, "ambiguous", ""
	case !found:
		c.Result, c.Code, c.Want = Skip, "unlisted", ""
	case sameDigest(digest, got):
		c.Result, c.Code = Pass, "match"
	default:
		c.Result, c.Code = Fail, "mismatch"
	}
	return c
}

// NoChecksums is the check when the release has no checksum file, or none
// could be loaded (code "none" or "unreadable").
func NoChecksums(code, file string) Check {
	return Check{ID: CheckChecksums, Result: Skip, Code: code, Source: file}
}
