package verify

import (
	"reflect"
	"strings"
	"testing"
)

func TestChecksumFiles(t *testing.T) {
	names := []string{
		"App-1.0-arm64.dmg", "App-1.0-x64.dmg", "app_1.0_checksums.txt",
		"SHA256SUMS", "App-1.0-x64.dmg.sha256", "app-1.0-ARM64.dmg.SHA256",
		"App-1.0-arm64.dmg.sha256sum", "notes.txt",
	}
	got := ChecksumFiles(names, "App-1.0-arm64.dmg")
	want := []string{"app-1.0-ARM64.dmg.SHA256", "App-1.0-arm64.dmg.sha256sum", "SHA256SUMS", "app_1.0_checksums.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ChecksumFiles = %q, want %q", got, want)
	}
	if got := ChecksumFiles([]string{"App.dmg"}, "App.dmg"); len(got) != 0 {
		t.Errorf("no checksum files: %q", got)
	}
}

func TestParseChecksums(t *testing.T) {
	sha512 := strings.Repeat("ab", 64)
	for _, c := range []struct {
		name  string
		data  string
		bare  bool
		want  string
		found bool
		err   bool
	}{
		{"gnu text", hexB + "  Other.dmg\n" + hexA + "  App.dmg\n", false, digA, true, false},
		{"gnu binary", hexA + " *App.dmg\n", false, digA, true, false},
		{"bsd", "SHA256 (App.dmg) = " + hexA + "\n", false, digA, true, false},
		{"path prefix", hexA + "  ./dist/App.dmg\n", false, digA, true, false},
		{"crlf and comments", "# made by CI\r\n\r\n" + hexA + "  App.dmg\r\n", false, digA, true, false},
		{"upper case", strings.ToUpper(hexA) + "  App.dmg\n", false, digA, true, false},
		{"name without case", hexA + "  app.DMG\n", false, digA, true, false},
		{"sha512 skipped", sha512 + "  App.dmg\n" + hexA + "  App.dmg\n", false, digA, true, false},
		{"missing", hexB + "  Other.dmg\n", false, "", false, false},
		{"twice differing", hexA + "  App.dmg\n" + hexB + "  App.dmg\n", false, "", false, true},
		{"twice same", hexA + "  App.dmg\n" + hexA + "  App.dmg\n", false, digA, true, false},
		{"bare allowed", hexA + "\n", true, digA, true, false},
		{"bare not allowed", hexA + "\n", false, "", false, false},
		{"named beats bare", hexB + "\n" + hexA + "  App.dmg\n", true, digA, true, false},
	} {
		got, found, err := ParseChecksums([]byte(c.data), "App.dmg", c.bare)
		if (err != nil) != c.err || found != c.found || got != c.want {
			t.Errorf("%s: got %q %v %v, want %q %v err=%v", c.name, got, found, err, c.want, c.found, c.err)
		}
	}
}
