package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/pklaschka/lieferling/internal/store"
)

// listApps prints the apps on the list with their installed and newest version.
func listApps(ctx context.Context, s *store.Store, out io.Writer) error {
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
}

// updateApps installs the newest release of the given apps, or of every app
// with an update; it goes on after a failure and names all that failed.
func updateApps(ctx context.Context, s *store.Store, repos []string, out io.Writer) error {
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
}

// install installs or updates one app, showing the download's progress.
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
