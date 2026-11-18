// Command lieferling manages apps installed from GitHub, Forgejo and Gitea
// releases.
//
//	lieferling list                         apps on the list, installed and newest version
//	lieferling add <repo> [--prerelease]    put a repository on the list
//	lieferling install <repo>               install (adds it to the list if needed)
//	lieferling update [<repo>...]           update the given apps, or every app with an update
//	lieferling open <repo>                  start the installed app
//	lieferling channel <repo> stable|prerelease
//	lieferling remove <repo> [--uninstall]  take it off the list (and off this computer)
//	lieferling paths                        where the shared and the local files are
//
// <repo> is owner/name for GitHub, host/owner/name for a Forgejo or Gitea
// server (codeberg.org/owner/name), or the repository's URL. Private
// repositories need a token: on GitHub GH_TOKEN, a GitHub CLI login (gh auth
// login) or the Keychain item "Lieferling GitHub"; on other servers
// LIEFERLING_TOKEN_<HOST> or the Keychain item "Lieferling <host>".
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
	"text/tabwriter"

	"github.com/pklaschka/lieferling/internal/pick"
	"github.com/pklaschka/lieferling/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, store.New(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "lieferling:", err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

var errUsage = errors.New("usage")

func usage() error {
	return fmt.Errorf("%w: lieferling list | add <repo> [--prerelease] | install <repo> | update [<repo>...] | open <repo> | channel <repo> stable|prerelease | remove <repo> [--uninstall] | paths", errUsage)
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
	if err := fs.Parse(reorder(rest)); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	pos := fs.Args()

	switch cmd {
	case "list", "ls":
		list, err := s.List(ctx)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(out, "No apps yet. Add one with: lieferling add owner/name (or host/owner/name)")
			return nil
		}
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "REPO\tCHANNEL\tINSTALLED\tNEWEST\t")
		for _, a := range list {
			installed := a.Installed
			if installed == "" {
				installed = "-"
			}
			newest := a.Latest
			switch {
			case a.Error != "":
				newest = "error: " + a.Error
			case a.UpdateAvailable:
				newest += "  (update)"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n", a.Repo, a.Channel, installed, newest)
		}
		return w.Flush()

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
		return install(ctx, s, pos[0], out)

	case "update", "upgrade":
		repos := pos
		if len(repos) == 0 {
			updates, err := s.Updates(ctx)
			if err != nil {
				return err
			}
			if len(updates) == 0 {
				fmt.Fprintln(out, "Everything is up to date.")
				return nil
			}
			for _, u := range updates {
				repos = append(repos, u.Repo)
			}
		}
		var failed []string
		for _, r := range repos {
			if err := install(ctx, s, r, out); err != nil {
				fmt.Fprintf(os.Stderr, "lieferling: %s: %v\n", r, err)
				failed = append(failed, r)
			}
		}
		if len(failed) > 0 {
			return fmt.Errorf("could not update %s", strings.Join(failed, ", "))
		}
		return nil

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

	case "paths":
		fmt.Fprintf(out, "shared (synced): %s\nthis computer:   %s\n", s.ConfigPath, s.StatePath)
		return nil

	case "help", "-h", "--help":
		fmt.Fprintln(out, strings.TrimSpace(help))
		return nil
	}
	return usage()
}

func install(ctx context.Context, s *store.Store, repo string, out io.Writer) error {
	last := -1
	st, err := s.Install(ctx, repo, func(_ string, done, total int64) {
		if total <= 0 {
			return
		}
		if pct := int(done * 100 / total); pct/5 != last/5 {
			last = pct
			fmt.Fprintf(out, "\rDownloading %s... %3d%%", repo, pct)
		}
	})
	if last >= 0 {
		fmt.Fprintln(out)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Installed %s %s at %s.\n", st.Name, st.Installed, st.Path)
	return nil
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
  lieferling install <repo>               install (adds it to the list if needed)
  lieferling update [<repo>...]           update the given apps, or every app with an update
  lieferling open <repo>                  start the installed app
  lieferling channel <repo> stable|prerelease
  lieferling remove <repo> [--uninstall]  take it off the list (and off this computer)
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

The list of apps is shared through iCloud Drive (Lieferling/config.json); what is
installed is recorded on each computer.`
