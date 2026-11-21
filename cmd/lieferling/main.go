// Command lieferling manages apps installed from GitHub, Forgejo and Gitea
// releases.
//
//	lieferling list                         apps on the list, installed and newest version
//	lieferling add <repo> [--prerelease]    put a repository on the list
//	lieferling install <repo> [--accept-sender]  install (adds it to the list if needed)
//	lieferling update [<repo>...] [--accept-sender]  update the given apps, or every app with an update
//	lieferling open <repo>                  start the installed app
//	lieferling channel <repo> stable|prerelease
//	lieferling remove <repo> [--uninstall]  take it off the list (and off this computer)
//	lieferling export [<file>]              write the list as a catalog (to stdout without a file)
//	lieferling import <file|url|-> [--dry-run]  add the apps of a catalog to the list
//	lieferling paths                        where the shared and the local files are
//
// <repo> is owner/name for GitHub, host/owner/name for a Forgejo or Gitea
// server (codeberg.org/owner/name), or the repository's URL. Private
// repositories need a token: on GitHub GH_TOKEN, a GitHub CLI login (gh auth
// login) or the Keychain item "Lieferling GitHub"; on other servers
// LIEFERLING_TOKEN_<HOST> or the Keychain item "Lieferling <host>".
//
// Every package goes through a customs check before it is installed (see
// package verify). A package that fails is not installed, and the command
// exits with 3. When only the sender changed, install and update ask on a
// terminal; --accept-sender accepts the new sender without asking.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/pklaschka/lieferling/internal/pick"
	"github.com/pklaschka/lieferling/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, store.New(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "lieferling:", err)
		os.Exit(exitCode(err))
	}
}

// exitCode is 2 for a usage error, 3 when customs stopped a package, else 1.
func exitCode(err error) int {
	switch {
	case errors.Is(err, errUsage):
		return 2
	case isHeld(err):
		return 3
	}
	return 1
}

var errUsage = errors.New("usage")

func usage() error {
	return fmt.Errorf("%w: lieferling list | add <repo> [--prerelease] | install <repo> [--accept-sender] | update [<repo>...] [--accept-sender] | open <repo> | channel <repo> stable|prerelease | remove <repo> [--uninstall] | export [<file>] | import <file|url|-> [--dry-run] | paths", errUsage)
}

func run(ctx context.Context, s *store.Store, args []string, out io.Writer) error {
	if len(args) == 0 {
		return usage()
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	prerelease := fs.Bool("prerelease", false, "follow release candidates too")
	uninstall := fs.Bool("uninstall", false, "also remove the app from this computer")
	dryRun := fs.Bool("dry-run", false, "only show what importing would add")
	acceptSender := fs.Bool("accept-sender", false, "install even when the sender changed")
	if err := fs.Parse(reorder(rest)); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	pos := fs.Args()

	switch cmd {
	case "list", "ls":
		return listApps(ctx, s, out)

	case "add":
		if len(pos) != 1 {
			return usage()
		}
		st, err := s.Add(ctx, pos[0], channel(*prerelease))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Added %s (%s, newest %s: %s). Install it with: lieferling install %s\n", st.Repo, st.Channel, st.Latest, st.Asset, st.Repo)
		return nil

	case "install":
		if len(pos) != 1 {
			return usage()
		}
		if *prerelease {
			if _, err := s.Add(ctx, pos[0], pick.Prerelease); err != nil {
				return err
			}
		}
		return install(ctx, s, pos[0], *acceptSender, out)

	case "update", "upgrade":
		return updateApps(ctx, s, pos, *acceptSender, out)

	case "open":
		if len(pos) != 1 {
			return usage()
		}
		return s.Open(ctx, pos[0])

	case "channel":
		if len(pos) != 2 {
			return usage()
		}
		if err := s.SetChannel(pos[0], pos[1]); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s now follows %s.\n", pos[0], pos[1])
		return nil

	case "remove", "rm":
		if len(pos) != 1 {
			return usage()
		}
		if err := s.Remove(ctx, pos[0], *uninstall); err != nil {
			return err
		}
		fmt.Fprintf(out, "Removed %s from the list.\n", pos[0])
		return nil

	case "export":
		if len(pos) > 1 {
			return usage()
		}
		file := ""
		if len(pos) == 1 && pos[0] != "-" {
			file = pos[0]
		}
		return exportCatalog(s, file, out)

	case "import":
		if len(pos) != 1 {
			return usage()
		}
		return importCatalog(ctx, s, pos[0], *dryRun, out)

	case "paths":
		fmt.Fprintf(out, "shared (synced): %s\nthis computer:   %s\n", s.ConfigPath, s.StatePath)
		return nil

	case "help", "-h", "--help":
		fmt.Fprintln(out, strings.TrimSpace(help))
		return nil
	}
	return usage()
}

func channel(prerelease bool) string {
	if prerelease {
		return pick.Prerelease
	}
	return pick.Stable
}

// reorder moves flags before the positional arguments, so that
// "lieferling add owner/name --prerelease" works as well.
func reorder(args []string) []string {
	var flags, pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

const help = `
lieferling manages apps installed from GitHub, Forgejo and Gitea releases.

  lieferling list                         apps on the list, installed and newest version
  lieferling add <repo> [--prerelease]    put a repository on the list
  lieferling install <repo> [--accept-sender]
                                          install (adds it to the list if needed)
  lieferling update [<repo>...] [--accept-sender]
                                          update the given apps, or every app with an update
  lieferling open <repo>                  start the installed app
  lieferling channel <repo> stable|prerelease
  lieferling remove <repo> [--uninstall]  take it off the list (and off this computer)
  lieferling export [<file>]              write the list as a catalog (to stdout without a file)
  lieferling import <file|url|-> [--dry-run]
                                          add the apps of a catalog to the list
  lieferling paths                        where the shared and the local files are

<repo> is owner/name for GitHub, host/owner/name for a Forgejo or Gitea server
(such as codeberg.org/owner/name), or the repository's URL.

Private repositories need a token with read access:
  GitHub          GH_TOKEN, a GitHub CLI login (gh auth login), or on macOS the
                  Keychain item "Lieferling GitHub"
                  (security add-generic-password -s "Lieferling GitHub" -a github -w)
  Forgejo, Gitea  LIEFERLING_TOKEN_<HOST> (LIEFERLING_TOKEN_CODEBERG_ORG), or on
                  macOS the Keychain item "Lieferling <host>"
                  (security add-generic-password -s "Lieferling codeberg.org" -a token -w)

A catalog is a JSON file with a list of repositories (what export writes), a
JSON array, or plain text with one repository per line. import takes a file, an
https URL or - for stdin; it adds the apps that are not on the list yet and
leaves the others as they are. It installs nothing.

Before installing, Lieferling checks the package at customs: the SHA-256 of
the download against what the forge lists, a checksum file in the release
(SHA256SUMS, checksums.txt, <file>.sha256) and what your other computers saw
for the same version; and the app's bundle ID and team ID against the last
version. A package that fails is not installed and the command exits with 3.
When only the sender changed, install and update ask; --accept-sender accepts
the new sender without asking.

The list of apps is shared through iCloud Drive (Lieferling/config.json); what is
installed is recorded on each computer.`
