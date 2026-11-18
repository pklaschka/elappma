package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pklaschka/lieferling/internal/catalog"
	"github.com/pklaschka/lieferling/internal/config"
	"github.com/pklaschka/lieferling/internal/store"
)

// ExportCatalog asks where to save the list as a catalog and writes it
// there. It returns the file's path, or "" when the user cancelled.
func (s *AppService) ExportCatalog() (string, error) {
	c, err := s.store.Export()
	if err != nil {
		return "", translate(err)
	}
	data, err := c.Encode()
	if err != nil {
		return "", err
	}
	d := s.app.Dialog.SaveFile().
		SetMessage("Die Liste als Katalog sichern, etwa für einen anderen Mac oder für andere.").
		SetButtonText("Sichern").
		SetFilename("Lieferling-Katalog.json").
		CanCreateDirectories(true).
		AddFilter("Lieferling-Katalog", "*.json")
	if w := s.app.Window.Current(); w != nil {
		d.AttachToWindow(w)
	}
	path, err := d.PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	if filepath.Ext(path) == "" {
		path += ".json"
	}
	if err := config.WriteFileAtomic(path, data); err != nil {
		return "", translate(err)
	}
	return path, nil
}

// ChooseCatalog asks for a catalog file and returns what importing it
// would do, without changing the list. It returns nil when the user
// cancelled.
func (s *AppService) ChooseCatalog() (*store.ImportPlan, error) {
	d := s.app.Dialog.OpenFile().
		SetTitle("Katalog importieren").
		SetMessage("Wähle einen Lieferling-Katalog oder eine Textdatei mit einem Repository pro Zeile.").
		SetButtonText("Öffnen").
		CanChooseFiles(true).
		AddFilter("Kataloge", "*.json;*.txt;*.md").
		AddFilter("Alle Dateien", "*")
	if w := s.app.Window.Current(); w != nil {
		d.AttachToWindow(w)
	}
	path, err := d.PromptForSingleSelection()
	if err != nil || path == "" {
		return nil, err
	}
	c, err := readCatalogFile(path)
	if err != nil {
		return nil, translate(fmt.Errorf("%s: %w", filepath.Base(path), err))
	}
	return s.plan(c, filepath.Base(path))
}

// CatalogFromURL loads a catalog from an https address and returns what
// importing it would do, without changing the list.
func (s *AppService) CatalogFromURL(ctx context.Context, url string) (*store.ImportPlan, error) {
	c, err := catalog.Fetch(ctx, nil, url)
	if err != nil {
		return nil, translate(err)
	}
	return s.plan(c, strings.TrimSpace(url))
}

// ImportCatalog adds the chosen apps of a catalog to the list and returns
// how many it added.
func (s *AppService) ImportCatalog(ctx context.Context, items []store.ImportItem) (int, error) {
	n, err := s.store.Import(items)
	if err != nil {
		return n, translate(err)
	}
	if n > 0 {
		s.changed(ctx)
	}
	return n, nil
}

func (s *AppService) plan(c catalog.Catalog, source string) (*store.ImportPlan, error) {
	p, err := s.store.PlanImport(c)
	if err != nil {
		return nil, translate(err)
	}
	p.Source = source
	for i := range p.Items {
		if p.Items[i].Error != "" {
			p.Items[i].Error = translate(fmt.Errorf("%s", p.Items[i].Error)).Error()
		}
	}
	return &p, nil
}

func readCatalogFile(path string) (catalog.Catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return catalog.Catalog{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, catalog.MaxSize+1))
	if err != nil {
		return catalog.Catalog{}, err
	}
	if len(data) > catalog.MaxSize {
		return catalog.Catalog{}, fmt.Errorf("the catalog is larger than 1 MB")
	}
	return catalog.Parse(data)
}
