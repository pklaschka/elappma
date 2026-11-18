package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pklaschka/lieferling/internal/catalog"
	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/store"
)

var stdin io.Reader = os.Stdin

func exportCatalog(s *store.Store, file string, out io.Writer) error {
	c, err := s.Export()
	if err != nil {
		return err
	}
	data, err := c.Encode()
	if err != nil {
		return err
	}
	if file == "" {
		_, err := out.Write(data)
		return err
	}
	if err := config.WriteFileAtomic(file, data); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %d apps to %s.\n", len(c.Apps), file)
	return nil
}

// readCatalog reads a catalog from an https URL, a file or stdin ("-").
func readCatalog(ctx context.Context, src string) (catalog.Catalog, error) {
	switch {
	case src == "-":
		data, err := io.ReadAll(io.LimitReader(stdin, catalog.MaxSize+1))
		if err != nil {
			return catalog.Catalog{}, err
		}
		if len(data) > catalog.MaxSize {
			return catalog.Catalog{}, fmt.Errorf("the catalog is larger than 1 MB")
		}
		return catalog.Parse(data)
	case strings.Contains(src, "://"):
		return catalog.Fetch(ctx, nil, src)
	}
	f, err := os.Open(src)
	if err != nil {
		return catalog.Catalog{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, catalog.MaxSize+1))
	if err != nil {
		return catalog.Catalog{}, err
	}
	if len(data) > catalog.MaxSize {
		return catalog.Catalog{}, fmt.Errorf("%s is larger than 1 MB", src)
	}
	return catalog.Parse(data)
}

func importCatalog(ctx context.Context, s *store.Store, src string, dryRun bool, out io.Writer) error {
	c, err := readCatalog(ctx, src)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	plan, err := s.PlanImport(c)
	if err != nil {
		return err
	}
	if c.Name != "" {
		fmt.Fprintf(out, "Catalog %q\n", c.Name)
	}
	onList, invalid := 0, 0
	for _, it := range plan.Items {
		switch {
		case it.Error != "":
			invalid++
			fmt.Fprintf(out, "  ! %s: %s\n", it.Repo, it.Error)
		case it.OnList:
			onList++
			fmt.Fprintf(out, "  = %s (already on the list)\n", it.Repo)
		default:
			fmt.Fprintf(out, "  + %s (%s)\n", it.Repo, it.Channel)
		}
	}
	fresh := plan.New()
	var added int
	if dryRun {
		added = len(fresh)
	} else if added, err = s.Import(fresh); err != nil {
		return err
	}
	verb := "Added"
	if dryRun {
		verb = "Would add"
	}
	fmt.Fprintf(out, "%s %d, %d already on the list, %d skipped.\n", verb, added, onList, invalid)
	if added > 0 && !dryRun {
		fmt.Fprintln(out, "Nothing is installed yet: lieferling list shows them, lieferling install <repo> installs one.")
	}
	return nil
}
