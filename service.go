package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/pklaschka/lieferling/internal/deeplink"
	"github.com/pklaschka/lieferling/internal/icon"
	"github.com/pklaschka/lieferling/internal/install"
	"github.com/pklaschka/lieferling/internal/pick"
	"github.com/pklaschka/lieferling/internal/store"
)

// Progress is sent to the window while a download runs.
type Progress struct {
	Repo  string `json:"repo"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

func init() {
	application.RegisterEvent[Progress]("install:progress")
	application.RegisterEvent[[]store.Status]("apps:changed")
	application.RegisterEvent[application.Void]("link:opened")
	application.RegisterEvent[application.Void]("command:open")
}

// AppService is what the window calls. Each method maps to one thing the
// user does; all of them leave the work to the store.
type AppService struct {
	store *store.Store
	app   *application.App

	self     string // this app's bundle, for updating itself
	relaunch func() // quits and opens the new copy

	mu   sync.Mutex
	last []store.Status // newest list, for the tray and notifications
	link *deeplink.Link // a deep link the window has not handled yet
}

// Apps returns the list with installed and newest versions.
func (s *AppService) Apps(ctx context.Context) ([]store.Status, error) {
	list, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	s.remember(list)
	return list, nil
}

func translateAll(list []store.Status) []store.Status {
	for i := range list {
		if list[i].Error != "" {
			list[i].Error = translate(fmt.Errorf("%s", list[i].Error)).Error()
		}
	}
	return list
}

// Add puts a repository on the list.
func (s *AppService) Add(ctx context.Context, repo string, prerelease bool) (store.Status, error) {
	ch := pick.Stable
	if prerelease {
		ch = pick.Prerelease
	}
	st, err := s.store.Add(ctx, repo, ch)
	if err != nil {
		return st, translate(err)
	}
	s.changed(ctx)
	return st, nil
}

// Install installs or updates one app.
func (s *AppService) Install(ctx context.Context, repo string) (store.Status, error) {
	st, err := s.store.Install(ctx, repo, func(repo string, done, total int64) {
		s.app.Event.Emit("install:progress", Progress{Repo: repo, Done: done, Total: total})
	})
	if err != nil {
		return st, translate(err)
	}
	if install.SamePath(st.Path, s.self) && s.relaunch != nil {
		// Lieferling just replaced itself: start the new copy
		go s.relaunch()
		return st, nil
	}
	s.changed(ctx)
	return st, nil
}

// TakeLink returns the deep link that opened Lieferling, once, or nil.
func (s *AppService) TakeLink() *deeplink.Link {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.link
	s.link = nil
	return l
}

// LinkFor returns the deep link that adds repo, for a README.
func (s *AppService) LinkFor(repo string, install bool) (string, error) {
	repo, err := store.NormalizeRepo(repo)
	if err != nil {
		return "", translate(err)
	}
	return deeplink.Link{Repo: repo, Install: install}.PageURL(), nil
}

// openLink keeps a deep link for the window and tells it.
func (s *AppService) openLink(raw string) {
	l, err := deeplink.Parse(raw)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.link = &l
	s.mu.Unlock()
	s.app.Event.Emit("link:opened")
}

// SetChannel switches between stable releases and release candidates.
func (s *AppService) SetChannel(ctx context.Context, repo string, prerelease bool) error {
	ch := pick.Stable
	if prerelease {
		ch = pick.Prerelease
	}
	if err := s.store.SetChannel(repo, ch); err != nil {
		return translate(err)
	}
	s.changed(ctx)
	return nil
}

// Remove takes an app off the list and, with uninstall, off this Mac.
func (s *AppService) Remove(ctx context.Context, repo string, uninstall bool) error {
	if err := s.store.Remove(ctx, repo, uninstall); err != nil {
		return translate(err)
	}
	s.changed(ctx)
	return nil
}

// Open starts an installed app.
func (s *AppService) Open(ctx context.Context, repo string) error {
	if err := s.store.Open(ctx, repo); err != nil {
		return translate(err)
	}
	return nil
}

// Icon returns the app's icon as a data URL, or "" when it has none.
func (s *AppService) Icon(ctx context.Context, repo string) (string, error) {
	ic, err := s.store.Icon(ctx, repo)
	if errors.Is(err, icon.ErrNone) {
		return "", nil
	}
	if err != nil {
		return "", translate(err)
	}
	return "data:" + ic.MIME + ";base64," + base64.StdEncoding.EncodeToString(ic.Data), nil
}

// OpenURL opens a release page in the browser.
func (s *AppService) OpenURL(url string) error {
	if !strings.HasPrefix(url, "https://github.com/") {
		return fmt.Errorf("only GitHub links are opened")
	}
	return s.app.Browser.OpenURL(url)
}

// CommandLine says whether the lieferling command is set up in the terminal.
func (s *AppService) CommandLine() install.Command {
	return install.CommandStatus(install.BundledCommand(s.self), install.CommandPath)
}

// InstallCommandLine links the command line in this app into the PATH,
// replacing an older copy there. macOS asks for an administrator password;
// closing that dialog changes nothing and is no error.
func (s *AppService) InstallCommandLine(ctx context.Context) (install.Command, error) {
	bundled := install.BundledCommand(s.self)
	err := install.LinkCommand(ctx, bundled, install.CommandPath, false)
	if err != nil && !errors.Is(err, install.ErrCanceled) {
		return s.CommandLine(), translate(err)
	}
	return s.CommandLine(), nil
}

// RemoveCommandLine takes the link out of the PATH again.
func (s *AppService) RemoveCommandLine(ctx context.Context) (install.Command, error) {
	err := install.UnlinkCommand(ctx, install.CommandPath, false)
	if err != nil && !errors.Is(err, install.ErrCanceled) {
		return s.CommandLine(), translate(err)
	}
	return s.CommandLine(), nil
}

// ConfigPath says where the shared list lives.
func (s *AppService) ConfigPath() string { return s.store.ConfigPath }

func (s *AppService) changed(ctx context.Context) {
	go func() {
		list, err := s.store.List(context.WithoutCancel(ctx))
		if err != nil {
			return
		}
		s.remember(list)
		s.app.Event.Emit("apps:changed", list)
	}()
}

func (s *AppService) remember(list []store.Status) {
	translateAll(list)
	for i := range list {
		list[i].Self = s.self != "" && install.SamePath(list[i].Path, s.self)
	}
	s.mu.Lock()
	s.last = list
	s.mu.Unlock()
}

// translate gives the store's English errors a German wording where the
// window shows them; anything unknown passes through.
func translate(err error) error {
	msg := err.Error()
	for en, de := range map[string]string{
		"repository or release not found (private repositories need a token)":           "Repository oder Release nicht gefunden. Private Repositories brauchen einen GitHub-Token.",
		"installing is not supported on this system yet":                                "Installieren geht auf diesem System noch nicht, bisher nur unter macOS.",
		"is not a GitHub repository (owner/name or its URL)":                            "ist kein GitHub-Repository. Erwartet wird owner/name oder die URL.",
		"has nothing to install on":                                                     "hat keine passende Datei für",
		"no release in the stable channel":                                              "hat noch kein stabiles Release",
		"no release in the prerelease channel":                                          "hat noch kein Release",
		"the app is not installed on this computer":                                     "Die App ist auf diesem Mac nicht installiert.",
		"Lieferling cannot remove itself while it runs: quit it and move it to the Bin": "Lieferling kann sich nicht selbst löschen, solange es läuft. Beende es und zieh es in den Papierkorb.",
		"this copy of Lieferling carries no command line":                               "Diese Ausgabe von Lieferling bringt keine Kommandozeile mit.",
		"is not a link Lieferling made: remove it yourself":                             "ist kein Verweis von Lieferling. Entferne die Datei selbst, wenn du sie nicht mehr brauchst.",
		"is a folder": "ist ein Ordner, kein Befehl. Lieferling lässt ihn unangetastet.",
	} {
		if strings.Contains(msg, en) {
			msg = strings.Replace(msg, en, de, 1)
		}
	}
	return fmt.Errorf("%s", msg)
}
