package forge

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// GitHubToken finds a GitHub token: GH_TOKEN or GITHUB_TOKEN, then the GitHub CLI's
// login (gh auth token), then, on macOS, the Keychain item "Lieferling GitHub"
// (security add-generic-password -s "Lieferling GitHub" -a github -w <token>)
// or the one from when Lieferling was called Elappma.
// It returns "" when there is none; public repositories still work then.
func GitHubToken() string {
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	if t := commandOutput(ghPath(), "auth", "token"); t != "" {
		return t
	}
	if runtime.GOOS == "darwin" {
		for _, item := range []string{"Lieferling GitHub", "Elappma GitHub"} {
			if t := commandOutput("security", "find-generic-password", "-s", item, "-w"); t != "" {
				return t
			}
		}
	}
	return ""
}

// ghPath finds the GitHub CLI also when the app is started from Finder, where
// PATH lacks Homebrew's directories.
// TokenFor finds the token for a Forgejo or Gitea server: the variable
// LIEFERLING_TOKEN_<HOST> (LIEFERLING_TOKEN_CODEBERG_ORG for codeberg.org),
// then, on macOS, the Keychain item "Lieferling <host>"
// (security add-generic-password -s "Lieferling codeberg.org" -a token -w <token>).
// It returns "" when there is none; public repositories still work then.
func TokenFor(host string) string {
	if v := strings.TrimSpace(os.Getenv(TokenVariable(host))); v != "" {
		return v
	}
	if runtime.GOOS == "darwin" {
		return commandOutput("security", "find-generic-password", "-s", "Lieferling "+host, "-w")
	}
	return ""
}

// TokenVariable is the environment variable TokenFor reads for host.
func TokenVariable(host string) string {
	var b strings.Builder
	b.WriteString("LIEFERLING_TOKEN_")
	for _, c := range strings.ToUpper(host) {
		if c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func ghPath() string {
	if p, err := exec.LookPath("gh"); err == nil {
		return p
	}
	for _, p := range []string{"/opt/homebrew/bin/gh", "/usr/local/bin/gh"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func commandOutput(name string, args ...string) string {
	if name == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
