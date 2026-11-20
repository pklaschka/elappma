# ADR 0001: Frontend in Module teilen, Nahtstellen im Kern freilegen

- **Status:** Vorgeschlagen
- **Datum:** 2026-11-20
- **Entscheider:in:** Zuri

## Kontext

Vier Features stehen an: Konten (#28), Lieferschein (#29), Zollkontrolle (#30) und »Andere Version installieren« (#31). Alle vier greifen in dieselben Stellen:

- **Die Zeile einer App.** #29 und #30 hängen je eine aufklappbare Zeile unter den Paketschein, #28 setzt einen Knopf in die Problemzeile, #31 einen Eintrag ins Menü.
- **Dialoge.** #28 bringt drei neue (Code, Token, Konten), #31 einen mit einer Auswahlliste nach dem Muster des Katalog-Imports.
- **Den Installationsweg.** #31 installiert ein anderes Release über denselben Weg, #30 schiebt eine Prüfung zwischen »entpackt« und »ersetzen«.
- **Die Forge-Schicht.** #28 braucht feinere Fehler, #29 und #30 weitere Felder an `Release` und `Asset`.

Der heutige Stand macht das unnötig schwer:

- `frontend/src/main.ts` ist eine Datei mit 841 Zeilen. Liste, Zeile, Installation, fünf Dialoge, Katalog und Kommandozeile stehen hintereinander und teilen sich veränderliche Variablen auf Modulebene (`apps`, `busy`, `noting`, `removing`, `linked`, `planned`, `catalogBusy`, `commandCall`).
- Jeder bestätigende Dialog wiederholt dasselbe Muster von Hand: Variable merken, `showModal()`, im `close`-Listener `returnValue` lesen und zurücksetzen.
- Das Zeilenmenü ist eine `if`-Kette in `createRow`, der Zustand der Einträge wird an anderer Stelle in `fillRow` gesetzt.
- `frontend/src/style.css` hat 790 Zeilen in einer Datei.
- `store.Install` wählt Release und Asset, lädt, installiert und schreibt den Zustand in einer Funktion. `mac.Install` entpackt und ersetzt in einem Zug.
- Die Forge-Clients übersetzen HTTP-Status an vier Stellen einzeln in Fehler.

Randbedingungen: kein Framework, keine neue Laufzeit-Abhängigkeit, Vanilla TypeScript mit Vite. Jeder Merge nach `main` veröffentlicht ein Release, also muss der Umbau das sichtbare Verhalten von App und Kommandozeile unverändert lassen und als `refactor:` laufen. Die Gestaltungsregeln aus `DESIGN.md` bleiben unberührt.

## Entscheidung

1. `main.ts` wird nach Features in Module geteilt. Gemeinsames liegt in `lib/`, der geteilte Zustand in einem kleinen Modul `state.ts`.
2. Die Zeile bekommt zwei Erweiterungspunkte nach demselben Prinzip: **Markup steht in der Vorlage in `index.html`, Verhalten wird per Name registriert.** `rowAction(name, …)` für Menüeinträge und den Hauptknopf, `rowSection(name, …)` für Zeilen unter dem Paketschein. Beide werden schon vom Bestand genutzt (alle heutigen Menüeinträge, die Problemzeile), es entsteht kein leerer Rahmen.
3. Zwei Helfer für Dialoge: `ask(dialog)` als Promise statt Variable plus `close`-Listener, und `setDialogState(dialog, state)` für Dialoge mit mehreren Zuständen (heute die Kommandozeile).
4. Die Auswahlliste des Katalog-Imports wird zu `lib/picklist.ts`, unabhängig vom Katalog.
5. `style.css` wird entlang ihrer vorhandenen Abschnitte in Dateien unter `styles/` geteilt, die Tokens bleiben an einem Ort. Die Reihenfolge bleibt exakt gleich.
6. Im Go-Kern: Fehlerübersetzung der Forges an einer Stelle, `store.Install` in Schritte zerlegt, `mac.Install` in Entpacken und Ersetzen, `service.go` und `store.go` nach Themen auf Dateien verteilt.

Nicht Teil des Umbaus: neue Felder, neue Methoden, neue Dialoge, Stubs für die Features.

## Betrachtete Optionen

| | A: Eine Datei behalten | B: Module nach Feature, zwei kleine Register | C: Komponenten (Lit, Preact, Custom Elements) |
|---|---|---|---|
| Komplexität | niedrig heute, hoch nach vier Features | mittel | hoch |
| Aufwand jetzt | keiner | etwa 2 Tage | 5 Tage und mehr |
| Risiko für das Verhalten | keines | gering, gut prüfbar | hoch (neu gebautes UI) |
| Neue Abhängigkeiten | keine | keine | ja, oder eigene Komponentenbasis |
| Lesbarkeit der Features | schlecht | gut | gut |
| Passt zu `AGENTS.md` | ja | ja | nein |

**A: Eine Datei behalten.**
- Dafür: kein Risiko, kein Aufwand.
- Dagegen: `main.ts` wüchse auf geschätzt 1.500 Zeilen. Vier Features würden dieselben Funktionen (`createRow`, `fillRow`) und dieselben Variablen anfassen, parallele Arbeit endet in Konflikten.

**B: Module nach Feature (gewählt).**
- Dafür: jedes Feature ist eine Datei plus ein Block in `index.html`. Die Register machen sichtbar, wo man andockt. Der Umbau ist ein Verschieben mit wenig neuem Code und lässt sich mit Bildvergleich und identischem CSS-Bundle prüfen.
- Dagegen: mehr Dateien, und die Register sind eine Indirektion, die man einmal verstehen muss.

**C: Komponenten.**
- Dafür: gekapselter Zustand, deklaratives Rendern.
- Dagegen: widerspricht der Vorgabe »kein Framework«. Das Subgrid der Liste (`.parcel` übernimmt die Spalten von `.parcels`) verträgt kein Shadow DOM zwischen Liste und Zeile. Und es wäre ein Neubau statt eines Umbaus, also nicht verhaltensgleich prüfbar.

Für den Go-Kern stand außerdem eine allgemeine Pipeline mit Schritt-Interface (`[]Step`) für die Installation im Raum. Verworfen: Es gibt genau eine Reihenfolge, private Funktionen in dieser Reihenfolge sind lesbarer als eine Liste von Objekten.

## Abwägung

- **Register gegen direkte Aufrufe.** Ohne Register müsste `row.ts` jedes Feature importieren und jedes Feature `row.ts`, also Zyklen. Mit Register zeigt die Abhängigkeit in eine Richtung: Features kennen die Zeile, die Zeile kennt keine Features.
- **Markup in `index.html` statt im Code.** Die Vorlage bleibt die eine Stelle, an der man die Zeile als Ganzes sieht, und Dialoge bleiben prüfbares HTML. Der Preis: `index.html` wächst mit jedem Feature weiter. Teilen ließe sie sich nur mit einem Vite-Plugin, das lohnt sich noch nicht.
- **Kein Test-Framework fürs Frontend.** Vitest wäre eine neue Entwicklungsabhängigkeit und eine eigene Entscheidung. Geprüft wird mit `tsc`, Bildvergleich und Durchklicken (siehe Prüfplan).
- **Kleine Helfer statt eines Dialog-Systems.** `ask` deckt die drei bestätigenden Dialoge ab. Den Ablauf »Arbeit bei offenem Dialog, abbrechbar« gibt es bisher einmal (Kommandozeile). Er bleibt dort, bis #28 ihn ein zweites Mal braucht.
- **Token-Suche bleibt, wie sie ist.** `forge.New()` liest das GitHub-Token einmal beim Start, Gitea je Host mit Cache. Für das Anmelden in der App muss das neu gelesen werden können. Das zu vereinheitlichen ändert das Startverhalten und gehört zu #28.

## Zielstruktur Frontend

```
frontend/
  index.html                 Vorlagen und Dialoge, nach Feature gruppiert
  src/
    main.ts                  Einstieg: setup…() der Features, dann Start
    backend.ts               einzige Stelle, die bindings und @wailsio/runtime importiert
    state.ts                 apps, busy, onApps
    lib/
      dom.ts                 $, clone
      text.ts                errorText, plural
      dialog.ts              ask, setDialogState
      menu.ts                placeUnder
      picklist.ts            fillPickList, picked
    features/
      list.ts                Laden, Platzhalter, Rendern, Zusammenfassung, note
      row.ts                 Zeile bauen und füllen, rowAction, rowSection
      install.ts             Installieren, Fortschritt, »Alle aktualisieren«
      app-actions.ts         Öffnen, Release öffnen, Kanal
      add.ts                 Formular »Repository hinzufügen«
      remove.ts              Dialog »Entfernen«
      deeplink.ts            Link kopieren, Link-Dialog
      catalog.ts             Katalog-Menü, Export, Import-Dialoge
      commandline.ts         Dialog »Kommandozeile«
    styles/
      index.css              nur @import, in dieser Reihenfolge:
      tokens.css  base.css  bar.css  controls.css  add.css  parcel.css
      menu.css  loading.css  dialog.css  catalog.css  commandline.css
      foot.css  responsive.css
```

### `backend.ts`

Reicht durch, damit die langen Binding-Pfade an einer Stelle stehen und ein Test-Aufbau genau ein Modul ersetzen muss:

```ts
export {Clipboard, Events} from "@wailsio/runtime";
export {AppService} from "../bindings/github.com/pklaschka/lieferling";
export {type Command, CommandState} from "../bindings/github.com/pklaschka/lieferling/internal/install/models";
export type {Link} from "../bindings/github.com/pklaschka/lieferling/internal/deeplink/models";
export type {ImportItem, ImportPlan, Status} from "../bindings/github.com/pklaschka/lieferling/internal/store/models";
```

### `state.ts`

```ts
export function getApps(): Status[];
export function findApp(repo: string): Status | undefined;   // ohne Groß-/Kleinschreibung
export function setApps(next: Status[]): void;                // ruft alle onApps-Hörer
export function onApps(fn: (apps: Status[]) => void): void;
export const busy: Set<string>;                               // Repos, die gerade installiert werden
```

`findApp` vergleicht immer ohne Groß-/Kleinschreibung. Heute tun das nur die Link-Stellen, die Zeile vergleicht exakt. Das ist gleichwertig, weil `config.Find` zwei Einträge, die sich nur in der Schreibung unterscheiden, gar nicht zulässt.

### `lib/dom.ts`, `lib/text.ts`, `lib/menu.ts`

```ts
export const $: <T extends Element>(sel: string, root?: ParentNode) => T;   // unverändert
export function clone<T extends Element>(template: HTMLTemplateElement): T; // erstes Element der Vorlage

export function errorText(err: unknown): string;                            // unverändert
export function plural(n: number, one: string, many: string): string;       // unverändert

export function placeUnder(menu: HTMLElement, button: HTMLElement): void;   // unverändert
```

`clone` ersetzt die drei Stellen mit `(tpl.content.cloneNode(true) as DocumentFragment).firstElementChild` (`showLoading`, `createRow`, `manifestRow`).

### `lib/dialog.ts`

```ts
// ask zeigt einen modalen Dialog und liefert den Wert des Knopfs, der ihn
// geschlossen hat; "" bei Esc. Eine noch offene Frage im selben Dialog endet mit "".
export function ask(dialog: HTMLDialogElement): Promise<string>;

// setDialogState setzt dialog.dataset.state und zeigt nur die Elemente, deren
// data-show (durch Leerzeichen getrennt) den Zustand nennt.
export function setDialogState(dialog: HTMLDialogElement, state: string): void;
```

`ask` braucht Sorgfalt, weil `close` verzögert feuert. So umsetzen:

```ts
const pending = new WeakMap<HTMLDialogElement, (value: string) => void>();

export function ask(dialog: HTMLDialogElement): Promise<string> {
  if (!listening.has(dialog)) {
    listening.add(dialog);
    dialog.addEventListener("close", () => {
      if (dialog.open) return;          // das close einer abgelösten Frage
      const value = dialog.returnValue;
      dialog.returnValue = "";
      settle(dialog, value);
    });
  }
  settle(dialog, "");                   // eine frühere Frage gilt nicht mehr
  if (dialog.open) dialog.close();
  dialog.returnValue = "";
  dialog.showModal();
  return new Promise((resolve) => pending.set(dialog, resolve));
}

function settle(dialog: HTMLDialogElement, value: string) {
  const resolve = pending.get(dialog);
  pending.delete(dialog);
  resolve?.(value);
}
```

(`listening` ist ein `WeakSet<HTMLDialogElement>`.) Das Zurücksetzen von `returnValue` folgt der Empfehlung aus modern-web-guidance, Leitfaden `dialog`: Esc setzt keinen Wert, ohne Zurücksetzen bliebe ein alter stehen.

Damit entfallen `removing`, `linked` und `planned`. Ein Nebeneffekt, der gewollt ist: Kommt heute ein zweiter Link, während der Link-Dialog offen ist, löscht der verspätete `close`-Listener `linked`, und »Hinzufügen« im neuen Dialog tut nichts. Mit `ask` gehört jede Antwort zu ihrer Frage.

`setDialogState` übernimmt aus `showCommand` die Zeile `commandDialog.dataset.state = …` und die Schleife über `[data-show]`.

### `lib/picklist.ts`

```ts
export interface PickItem {
  value: string;        // Wert des Kästchens, zugleich die Beschriftung
  tag?: boolean;        // zeigt das Kennzeichen der Vorlage (»Vorab«)
  note?: string;        // rechts, oder bei state "invalid" als eigene Zeile
  state: string;        // data-state der Zeile: heute "new", "known", "invalid"
  checked?: boolean;
  disabled?: boolean;
}
export function fillPickList(list: HTMLUListElement, items: PickItem[]): void;
export function picked(list: HTMLUListElement): string[];   // Werte: angekreuzt und nicht gesperrt
```

`fillPickList` ist `manifestRow` ohne Katalogwissen (Vorlage `#manifest-item`, Klassen `.manifest*` bleiben). `catalog.ts` bildet `ImportItem` auf `PickItem` ab: neu → `{state: "new", checked: true}`, schon auf der Liste → `{state: "known", checked: true, disabled: true, note: "Schon auf der Liste"}`, fehlerhaft → `{state: "invalid", disabled: true, note: it.error}`; `tag` ist `channel === "prerelease" && !it.error`. `picked` ersetzt den Selektor in `chosen()`: bekannte Einträge sind gesperrt und fallen deshalb heraus, wie bisher.

### `features/row.ts`

```ts
export interface RowAction {
  run(app: Status, row: HTMLLIElement): void;
  // bei jedem Füllen: Eintrag sperren, ankreuzen oder umbenennen
  update?(item: HTMLButtonElement, app: Status): void;
}
// name ist das data-act des Menüeintrags in der Vorlage #row
export function rowAction(name: string, action: RowAction): void;

export interface RowSection {
  setup?(el: HTMLElement, row: HTMLLIElement): void;          // einmal je Zeile
  fill(el: HTMLElement, app: Status, row: HTMLLIElement): void; // bei jedem Füllen
}
// name ist das data-section eines direkten Kindes von li.parcel in der Vorlage
export function rowSection(name: string, section: RowSection): void;

export function createRow(repo: string): HTMLLIElement;
export function fillRow(row: HTMLLIElement, app: Status): void;
export function findRow(repo: string): HTMLLIElement | null;  // ersetzt die fünf querySelector mit CSS.escape
export function showRowError(row: HTMLLIElement, message: string): void;
export function setBusy(row: HTMLLIElement, on: boolean): void;
export function flash(row: HTMLLIElement, text: string): void;
```

- `createRow`: der Menü-Listener ruft `actions.get(act)?.run(app, row)` statt der `if`-Kette. Der Hauptknopf ruft dieselbe Tabelle mit `dataset.mode` (`"open"` oder `"install"`), denn beide Modi sind zugleich Aktionsnamen. Danach `setup` jeder Section.
- `fillRow`: bleibt, gibt aber die Zeilen zu den Menüeinträgen ab (`[data-act="open"].disabled`, Text und `disabled` von `release`, `aria-checked` von `channel`). Dafür am Ende: für jedes `[data-act]` der Zeile `update` rufen, für jedes `:scope > [data-section]` `fill` rufen, dann `setBusy`.
- Die Problemzeile wird die erste Section: in der Vorlage `<p class="problem" data-section="problem" hidden>`, in `row.ts` `rowSection("problem", {fill: (_, app, row) => showRowError(row, app.error)})`.
- `showIcon` und der Cache `icons` ziehen unverändert mit um.
- Wichtig: Aktionen und Sections müssen registriert sein, bevor die erste Zeile entsteht. `main.ts` ruft deshalb alle `setup…()` vor `load()`.

### Was wohin zieht

| Aus `main.ts` | Nach |
|---|---|
| `$` | `lib/dom.ts` |
| `errorText`, `plural` | `lib/text.ts` |
| `placeUnder` | `lib/menu.ts` |
| `apps`, `busy` | `state.ts` |
| `MIN_LOADING_MS`, `COUNT_KEY`, `loaded`, `showLoading`, `load`, `render`, `noting`, `noteTimer`, `note`, Listener `apps:changed` und `#refresh` | `features/list.ts` (exportiert `setupList`, `load`, `note`) |
| `rowCount`, `createRow`, `fillRow`, `icons`, `showIcon`, `showRowError`, `setBusy`, `flash` | `features/row.ts` |
| `install`, Listener `install:progress` und `#update-all` | `features/install.ts` (exportiert `setupInstall`, `install`; registriert `install`) |
| `open`, `setChannel`, Menüzweig `release` | `features/app-actions.ts` (registriert `open`, `release`, `channel` samt `update`) |
| Listener von `#add` und `#repo` | `features/add.ts` |
| `removing`, `confirmRemove`, `close`-Listener von `#confirm-remove` | `features/remove.ts` (registriert `remove`, nutzt `ask`) |
| `copyLink`, `linked`, `takeLink`, `close`-Listener von `#confirm-link`, Listener `link:opened` | `features/deeplink.ts` (registriert `link`, exportiert `takeLink`, nutzt `ask`) |
| `catalogMenu` bis zum `close`-Listener von `#import`, ohne `note` und `manifestRow` | `features/catalog.ts` (nutzt `ask`, `fillPickList`, `picked`) |
| `manifestRow`, `chosen` | `lib/picklist.ts` |
| `commandDialog` bis `Events.On("command:open")` | `features/commandline.ts` (nutzt `setDialogState`) |
| `AppService.ConfigPath()…`, `load().then(takeLink)` | `main.ts` |

Einzelheiten, die beim Verschieben leicht verloren gehen:

- `render(next)` setzt heute `apps` und zeichnet. Künftig: `setApps(next)` überall, wo bisher `render(next)` mit neuen Daten stand (`load`, `apps:changed`, nach `Add` im Link-Dialog). `list.ts` hängt sein Zeichnen mit `onApps` ein und behält die Prüfung `if (!loaded) return`. `note` ruft nach fünf Sekunden nur das Zeichnen, nicht `setApps`.
- `takeLink` ruft `load()`, wenn die Liste leer ist, `deeplink.ts` importiert es aus `list.ts`. Der Link-Dialog schreibt Fehler nach `#add-error`. Das bleibt so.
- `exportCatalog` nutzt `apps.length` und `note`.
- In `commandline.ts` wird `commandManual` vor seiner Deklaration benutzt (funktioniert heute durch die Reihenfolge der Aufrufe). Beim Umzug die `const` nach oben stellen.
- Jedes Feature exportiert ein `setup…()` und verdrahtet nur dort. `main.ts`:

```ts
import "@fontsource-variable/archivo/wdth.css";
setupList(); setupInstall(); setupAppActions(); setupAdd(); setupRemove();
setupDeeplink(); setupCatalog(); setupCommandLine();
AppService.ConfigPath().then((p) => ($("#config-path").textContent = p));
load().then(takeLink);
```

### `index.html`

Inhalt bleibt, zwei Änderungen (die dritte ist zurückgestellt):

1. `<link rel="stylesheet" href="/src/styles/index.css">`.
2. `.problem` bekommt `data-section="problem"`.
3. Zurückgestellt, damit der Umbau leicht zu prüfen bleibt: Reihenfolge mit Kommentarköpfen: Sprite, Fenster, Vorlagen (`#row`, `#ghost`, `#manifest-item`), dann die Dialoge je Feature. Die Reihenfolge der Dialoge untereinander ist ohne Wirkung, sie liegen im Top Layer.

Nach der Änderung die Tags auszählen (siehe Prüfplan): Ein früherer Merge hat einen Dialog durch fehlende schließende Tags zerstört.

### CSS

Schnitt an den vorhandenen Abschnittsköpfen von `style.css`, Reihenfolge unverändert. Die Zeilenangaben gelten für den Stand von `main` heute:

| Datei | Zeilen | Inhalt |
|---|---|---|
| `tokens.css` | 1–57 | Kopfkommentar, `:root`, dunkle Kraft-Texturen |
| `base.css` | 59–109 | Reset, `body`, Fokus, `.icon`, Auswahl |
| `bar.css` | 111–157 | Titelleiste |
| `controls.css` | 158–260 | Knöpfe, Felder, Kästchen |
| `add.css` | 261–278 | Streifen »Hinzufügen«, `.field-error` |
| `parcel.css` | 279–497 | Liste, Stempel, Sendungsnummer, Strahl, `.problem` |
| `menu.css` | 498–545 | Menü |
| `loading.css` | 546–649 | leere Liste, Platzhalter |
| `dialog.css` | 650–667 | Dialog allgemein |
| `catalog.css` | 669–705 | `#import-url`, `#import`, `.manifest` |
| `commandline.css` | 706–743 | Dialog »Kommandozeile« |
| `foot.css` | 745–769 | Fußzeile |
| `responsive.css` | 771–790 | schmale Fenster, reduzierte Bewegung |

Eine inhaltliche Änderung, ohne Wirkung auf die Darstellung: In `parcel.css` zieht `grid-column: 1 / -1` aus `.problem` in eine eigene Regel `.parcel > [data-section] { grid-column: 1 / -1; }` direkt davor. Damit sitzt jede künftige Zeile unter dem Paketschein von selbst im Raster. `tokens.css` bleibt die einzige Datei mit Farb- und Maßwerten.

Vite löst `@import` in CSS selbst auf, es braucht kein Plugin.

## Zielstruktur Go

```
service.go        AppService, Ereignisse, Apps, Add, Install, SetChannel, Remove, Open, Icon, ConfigPath, changed, remember
links.go          TakeLink, LinkFor, openLink, OpenURL, knownHost            (neu, aus service.go)
commandline.go    CommandLine, InstallCommandLine, RemoveCommandLine, commandDone, commandTimeout   (neu, aus service.go)
translate.go      translate, translateAll                                    (neu, aus service.go)
catalog.go        unverändert

internal/forge/
  errors.go       ErrNotFound, ErrNoForge, ErrBadRepo, statusError           (neu)
  forge.go        ohne die Fehler
  github.go       request nutzt statusError
  gitea.go        check ersetzt getOK und die Statusprüfungen; giteaRelease.release()

internal/install/
  install_darwin.go   Install = unpack + place

internal/store/
  store.go        Store, New, Config, NormalizeRepo, validChannel
  list.go         Status, List, status, Updates                              (neu, aus store.go)
  apps.go         Add, SetChannel, Remove, Open, installedPath               (neu, aus store.go)
  install.go      Progress, Install, newest, installRelease, download, record   (neu, aus store.go)

cmd/lieferling/
  main.go         run verteilt nur noch
  commands.go     list, update, install                                      (neu, aus main.go)
```

### `internal/forge`

```go
// statusError turns an HTTP answer into nil or an error. who names the
// server in the message: "GitHub" or the host.
func statusError(who string, resp *http.Response) error
```

404 wird `ErrNotFound`, alles ab 300 `"<who> answered <Status>: <Anfang des Body>"`. Das ersetzt die Prüfung in `GitHub.request` und `answered` in `gitea.go`; die Texte bleiben Zeichen für Zeichen gleich, weil `translate` und die Tests darauf prüfen. In `gitea.go` wird aus `getOK` und den Prüfungen in `Download` ein `check(r Repo, resp) error`. `Gitea.Releases` behält seinen Sonderweg (bei 404 erst `isForge` fragen), ruft danach aber dieselbe Funktion.

```go
func (g giteaRelease) release() Release   // die Abbildung aus der Schleife in Gitea.Releases
```

### `internal/install`

`mac.Install` wird zu zwei privaten Schritten, die Schnittstelle `Installer` bleibt:

```go
// unpacked is an app taken out of its download, not yet in place.
type unpacked struct {
	source string // the .app inside the mounted DMG or the unzipped folder
	name   string // bundle name without .app
	close  func() // detaches the DMG, removes the work folder
}

func (m *mac) unpack(ctx context.Context, file, assetName string) (unpacked, error)
func (m *mac) place(ctx context.Context, u unpacked) (Result, error) // quit, ditto, xattr, mv, reopen
```

`Install` ruft `unpack`, `defer u.close()`, `place`. Achtung auf die Reihenfolge beim Aufräumen: erst `hdiutil detach`, dann `os.RemoveAll(work)`, wie heute durch die `defer`-Reihenfolge.

### `internal/store`

```go
// newest picks the newest release of the app's channel and its asset for this computer.
func (s *Store) newest(ctx context.Context, app config.App) (forge.Release, forge.Asset, error)

// installRelease downloads asset, installs it and records the result.
func (s *Store) installRelease(ctx context.Context, app config.App, rel forge.Release, asset forge.Asset, progress Progress) (Status, error)

// download writes the asset to a temporary file; remove deletes it.
func (s *Store) download(ctx context.Context, app config.App, asset forge.Asset, progress Progress) (file string, remove func(), err error)

// record writes what was installed to state.json and the app name to config.json.
func (s *Store) record(app config.App, rel forge.Release, asset forge.Asset, res install.Result) (state.State, error)
```

`Install` bleibt: Repo normalisieren, App aus der Liste lesen, sperren, `newest`, `installRelease`. `newest` trägt die beiden Fehlertexte aus `Install` (»has no release in the … channel«, »has nothing to install on … (one still being built has no files yet)«) unverändert. `status` behält seine eigene Auswahl, weil es bei fehlendem Asset trotzdem `Latest` füllt und andere Texte hat.

### `service.go`, `cmd/lieferling`

Reines Verteilen auf Dateien, keine Änderung an Signaturen: Die Bindings müssen danach identisch erzeugt werden. In `cmd/lieferling` ziehen die langen Zweige `list` und `update` als Funktionen nach `commands.go`, `install` ebenfalls. `run` bleibt ein `switch`, in dem jeder Befehl eine Zeile ist.

## Wo die Features andocken

### #28 Konten

- **Fehlerarten:** neue Fehler (`ErrNoToken`, `ErrNoAccess`, `ErrNoRelease`) in `forge/errors.go`, unterschieden in `statusError` (401/403 gegen 404, mit und ohne Token). Der deutsche Satz dazu in `translate.go`. `Status` bekommt in `store/list.go` eine Fehlerart neben `Error`.
- **Icon ohne Code-Zugriff:** `store.repoIcon` behandelt `ErrNoAccess` wie `ErrNotFound`.
- **Token:** `forge/token.go` (Quelle, `Store`, `Forget`), dazu `internal/account`. Hier wird auch vereinheitlicht, dass GitHub sein Token nach dem Anmelden neu liest.
- **Dienst:** neue Datei `accounts.go` neben `links.go` mit `Accounts`, `Login`, `SubmitToken`, `Logout`.
- **Frontend:** `features/accounts.ts`. Der Knopf »Anmelden …« kommt in die Section `problem` (Markup in `#row`, Verhalten löst die Registrierung in `row.ts` ab). Die Dialoge nutzen `setDialogState` für ihre Schritte und `ask` für die Übersicht. Für das Warten auf den Browser dient `runCommand` in `commandline.ts` als Vorlage; wird es ein zweites Mal gebraucht, wandert es nach `lib/dialog.ts`. Die wartende Installation: `install(repo)` aus `features/install.ts` nach erfolgreichem Anmelden.
- **Repos entdecken:** `fillPickList` im vorhandenen Import-Dialog.
- **CLI:** zwei Zweige in `run`, Funktionen in `commands.go`.

### #29 Lieferschein

- **Forge:** `Body` an `Release` (GitHub per JSON-Tag, Gitea in `giteaRelease.release()`). `PublishedAt` und `HTMLURL` gibt es schon.
- **Kern:** `internal/notes`, `store/notes.go` mit `Notes` und `History`; `ReleasesBehind` und `Breaking` in `store/list.go`. Der Verlauf wird in `store.record` geschrieben.
- **Frontend:** `<details data-section="notes" hidden>` in `#row`, `features/notes.ts` mit `rowSection("notes", {setup, fill})`. `fill` zeigt oder versteckt die Zeile und setzt die Zusammenfassung, `setup` hängt den `toggle`-Listener für das Nachladen an (`toggle` steigt nicht auf, deshalb je Element). Styles in `styles/notes.css`.
- **HTML im WebView:** Go säubert. Im Frontend zusätzlich nie `innerHTML` mit rohem Text. `setHTML()` gibt es in Safari nicht (modern-web-guidance, `sanitize-untrusted-html`), also entweder nur das in Go erzeugte, auf eine feste Liste von Elementen beschränkte HTML einsetzen oder die Knoten im Frontend aus einer Struktur bauen. Das ist in #29 zu entscheiden.

### #30 Zollkontrolle

- **Digest beim Laden:** in `store.download`, mit `io.MultiWriter(datei, sha256.New())`. `forge.Download` nimmt schon einen `io.Writer`, die Forge-Schicht bleibt dafür unberührt. `Digest` an `Asset` wie `Body` oben.
- **Prüfung zwischen »entpackt« und »ersetzen«:** zwischen `unpack` und `place`. `Installer.Install` bekommt dafür einen Rückruf mit dem entpackten Bundle, den `store.installRelease` mit `internal/verify` füllt. Ob Quarantäne bleibt, wird ein Parameter von `place`.
- **Richtlinie und Digests:** `config.App` und `config.Config`, geschrieben in `store.record`.
- **Frontend:** zweite Section `<details data-section="customs" hidden>` und `features/customs.ts`, gleiches Muster wie #29. Die Rückfrage beim Absenderwechsel ist ein `ask`-Dialog.

### #31 Andere Version installieren

- **Kern:** `pick.ByVersion`, `store.Versions`, und `InstallVersion` als zweiter Aufrufer von `installRelease` neben `Install`.
- **Frontend:** `<button data-act="versions">` im Menü von `#row`, `features/versions.ts` mit `rowAction("versions", {run, update})` (`update` sperrt den Eintrag für Lieferling selbst). Der Dialog füllt seine Liste mit `fillPickList`. Dafür bekommt `PickItem` die Art des Kästchens (`radio`) und ein zweites Kennzeichen; ausgegraute Releases sind `disabled`.
- **CLI:** `install` in `commands.go` trennt `repo@version`.

## Umsetzung in Commits

Alle mit `refactor:`, jeder für sich baubar und geprüft:

1. `refactor(frontend): split the stylesheet by section` — `styles/`, `index.html`-Link, `[data-section]`-Regel.
2. `refactor(frontend): split main.ts into feature modules` — `backend.ts`, `state.ts`, `lib/*` mit `ask` und der Auswahlliste, `features/*`, `rowAction`, `rowSection`, `setup…()`. (Geplant waren hier drei Commits; Zwischenstände hätten Code gebraucht, der gleich wieder wegfällt.)
3. `refactor(forge): map HTTP answers to errors in one place`
4. `refactor(install): unpack and place as separate steps`
5. `refactor(store): split the store by topic and the install into steps`
6. `refactor: split the service and the command line by topic`

## Prüfplan

- **Go:** `go vet ./internal/... ./cmd/...` und `go test ./internal/... ./cmd/...` nach jedem Go-Commit. `GOOS=darwin GOARCH=arm64 go build ./cmd/lieferling` und `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -tags purego .`, weil `install_darwin.go` unter Linux sonst gar nicht übersetzt wird.
- **Bindings:** `CGO_ENABLED=0 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`, dann `wails3 generate bindings -ts -i -clean=true`. `frontend/bindings` ist nicht eingecheckt; vor und nach den Go-Commits erzeugen und mit `diff -r` vergleichen, es darf keinen Unterschied geben.
- **Frontend:** `cd frontend && npm ci && npm run build` nach jedem Frontend-Commit.
- **CSS:** Das gebaute Stylesheet in `frontend/dist/assets/` muss nach Commit 1 bis auf die verschobene `grid-column`-Regel dem von `main` gleichen. Am einfachsten: erst nur teilen und prüfen, dass Dateiname (Hash) und Inhalt identisch sind, dann die Regel verschieben.
- **Bildvergleich:** kein Aufbau im Repo. In einem Arbeitsordner außerhalb: eine Vite-Konfiguration mit `root` auf `frontend/`, die `@wailsio/runtime` und die Bindings auf Attrappen mit Beispieldaten umlenkt (auf `main` drei Binding-Module per `resolveId`-Plugin, nach Commit 2 nur noch `backend.ts`). Mit Playwright und Chromium bei `deviceScaleFactor: 2` aufnehmen: Liste hell und dunkel, Ladezustand, Zeilenmenü, Katalog-Menü, Dialoge Entfernen, Link, Adresse, Import und Kommandozeile (alle vier Zustände), schmales Fenster. Einmal auf `main`, einmal nach dem Umbau, Pixel vergleichen. Animationen dafür anhalten (`prefers-reduced-motion` emulieren, vor der Aufnahme warten).
- **Durchklicken im selben Aufbau:** Die Attrappen zeichnen jeden Aufruf auf. Skript: jeden Menüeintrag jeder Zeile, Hauptknopf in allen drei Zuständen, »Alle aktualisieren«, Hinzufügen mit gültiger und ungültiger Eingabe, Entfernen mit und ohne Häkchen, Esc in jedem Dialog, Import mit abgewählten Einträgen, Ereignisse `apps:changed`, `install:progress`, `link:opened`, `catalog:menu`, `command:open`. Die Folge der Aufrufe muss auf `main` und danach dieselbe sein. Fehlende Listener sind das Hauptrisiko des Teilens, und genau die fängt das.
- **`index.html`:** öffnende und schließende Tags je Element zählen (`dialog`, `form`, `template`, `div`, `ul`, `li`, `p`) und im Aufbau prüfen, dass `document.querySelectorAll("dialog").length` gleich bleibt und kein Dialog in einem anderen steckt.

## Folgen

- Ein Feature ist künftig eine Datei unter `features/`, ein Block in `index.html`, eine Datei unter `styles/` und ein `setup…()` in `main.ts`. `row.ts` muss für #29, #30 und #31 nicht angefasst werden.
- Zwei Features lassen sich parallel bauen, ohne dass beide dieselben Funktionen ändern. Berührungspunkte bleiben `index.html` und `main.ts`, dort aber zeilenweise.
- Der Merge veröffentlicht ein Patch-Release ohne sichtbare Änderung: `scripts/next-version.sh` zählt alles außer `feat` und Breaking Changes als Patch.
- Die Register verstecken, wer eine Aktion behandelt. Gegenmittel: Der Name im `data-act` oder `data-section` ist derselbe wie im Aufruf, eine Suche findet beide.
- `index.html` wächst weiter. Wenn sie mit den neuen Dialogen unhandlich wird, ist das Teilen eine eigene Entscheidung.
- Risiko beim Umsetzen: verlorene Listener und eine veränderte Reihenfolge beim Start. Der Prüfplan ist darauf ausgelegt.

## Offene Punkte

- Soll das Frontend eine Testumgebung bekommen (etwa der Aufbau aus dem Prüfplan, eingecheckt)? Das wäre eine neue Entwicklungsabhängigkeit und deshalb hier nicht entschieden.
- Die Doku im Repo (`README.md`, `DESIGN.md`, `AGENTS.md`) ist englisch, die Issues sind deutsch. Dieses ADR ist deutsch wie die Issues; falls ADRs englisch sein sollen, wird es übersetzt.
