package install

import "testing"

func TestParseTeamID(t *testing.T) {
	for name, c := range map[string]struct{ out, want string }{
		"developer id": {`Executable=/Applications/Feldspur.app/Contents/MacOS/Feldspur
Identifier=de.pklaschka.feldspur
Format=app bundle with Mach-O thin (arm64)
CodeDirectory v=20500 size=1234 flags=0x10000(runtime) hashes=28+7 location=embedded
Signature size=9046
Authority=Developer ID Application: Example GmbH (ABCDE12345)
Authority=Developer ID Certification Authority
Authority=Apple Root CA
Timestamp=21. Nov 2026 at 10:00:00
Info.plist entries=30
TeamIdentifier=ABCDE12345
Runtime Version=14.0.0
Sealed Resources version=2 rules=13 files=40
Internal requirements count=1 size=212`, "ABCDE12345"},
		"ad hoc": {`Executable=/tmp/App.app/Contents/MacOS/App
Identifier=com.example.app
Signature=adhoc
TeamIdentifier=not set`, ""},
		"unsigned":  {`/tmp/App.app: code object is not signed at all`, ""},
		"malformed": {`TeamIdentifier=abc"; rm -rf /`, ""},
	} {
		if got := parseTeamID(c.out); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
}
