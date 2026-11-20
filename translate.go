package main

import (
	"fmt"
	"strings"

	"github.com/pklaschka/lieferling/internal/store"
)

// translate gives the store's English errors a German wording where the
// window shows them; anything unknown passes through.
func translate(err error) error {
	msg := err.Error()
	for en, de := range map[string]string{
		"repository or release not found (private repositories need a token)":           "Repository oder Release nicht gefunden. Private Repositories brauchen einen Token.",
		"does not answer like a Forgejo or Gitea server":                                "antwortet nicht wie ein Forgejo- oder Gitea-Server.",
		"is not a repository (owner/name, host/owner/name or its URL)":                  "ist kein Repository. Erwartet wird owner/name für GitHub, host/owner/name für Forgejo und Gitea oder die URL.",
		"installing is not supported on this system yet":                                "Installieren geht auf diesem System noch nicht, bisher nur unter macOS.",
		"has nothing to install on":                                                     "hat keine passende Datei für",
		"no release in the stable channel":                                              "hat noch kein stabiles Release",
		"no release in the prerelease channel":                                          "hat noch kein Release",
		"the app is not installed on this computer":                                     "Die App ist auf diesem Mac nicht installiert.",
		"Lieferling cannot remove itself while it runs: quit it and move it to the Bin": "Lieferling kann sich nicht selbst löschen, solange es läuft. Beende es und zieh es in den Papierkorb.",
		// catalogs
		"the catalog lists no repositories":                                          "Im Katalog steht kein Repository.",
		`not a catalog: there is no "apps" list`:                                     "Das ist kein Katalog: Die Liste „apps“ fehlt.",
		"this is a web page, not a catalog (for a file on GitHub, use its Raw link)": "Das ist eine Webseite, kein Katalog. Bei einer Datei auf GitHub hilft ihr Raw-Link.",
		"catalogs are only loaded over https":                                        "Kataloge werden nur über https geladen.",
		"is not a web address":                                                       "ist keine Webadresse",
		"the server answered":                                                        "Der Server antwortete mit",
		"the catalog is larger than 1 MB":                                            "Der Katalog ist größer als 1 MB.",
		"not valid JSON":                                                             "kein gültiges JSON",
		"unknown channel":                                                            "Unbekannter Kanal",
		"use stable or prerelease":                                                   "erlaubt sind stable und prerelease",
		"this copy of Lieferling carries no command line":                            "Diese Ausgabe von Lieferling bringt keine Kommandozeile mit.",
		"is not a link Lieferling made: remove it yourself":                          "ist kein Verweis von Lieferling. Entferne die Datei selbst, wenn du sie nicht mehr brauchst.",
		"is a folder":                                                                "ist ein Ordner, kein Befehl. Lieferling lässt ihn unangetastet.",
	} {
		if strings.Contains(msg, en) {
			msg = strings.Replace(msg, en, de, 1)
		}
	}
	return fmt.Errorf("%s", msg)
}

// translateAll translates the error of every app in the list.
func translateAll(list []store.Status) []store.Status {
	for i := range list {
		if list[i].Error != "" {
			list[i].Error = translate(fmt.Errorf("%s", list[i].Error)).Error()
		}
	}
	return list
}
