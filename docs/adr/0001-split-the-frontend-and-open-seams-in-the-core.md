# ADR 0001: Split the frontend into modules, open seams in the core

- **Status:** Proposed
- **Date:** 2026-11-20
- **Decider:** Zuri

## Context

Four features are coming: accounts (#28), delivery note (#29), customs check (#30) and "install another version" (#31). All four touch the same places:

- **An app's row.** #29 and #30 each hang an expandable line under the parcel label, #28 puts a button into the problem line, #31 adds a menu entry.
- **Dialogs.** #28 brings three new ones (code, token, accounts), #31 one with a pick list like the catalog import's.
- **The install path.** #31 installs another release the same way, #30 puts a check between "unpacked" and "replace".
- **The forge layer.** #28 needs finer errors, #29 and #30 more fields on `Release` and `Asset`.

The code as it stood made that harder than it needs to be:

- `frontend/src/main.ts` was one file of 841 lines. List, row, install, five dialogs, catalog and command line followed one another and shared mutable module-level variables (`apps`, `busy`, `noting`, `removing`, `linked`, `planned`, `catalogBusy`, `commandCall`).
- Every confirming dialog repeated the same pattern by hand: remember a variable, `showModal()`, read and reset `returnValue` in a `close` listener.
- The row menu was an `if` chain in `createRow`; the state of its entries was set elsewhere, in `fillRow`.
- `frontend/src/style.css` was 790 lines in one file.
- `store.Install` picked release and asset, downloaded, installed and wrote the state in one function. `mac.Install` unpacked and replaced in one go.
- The forge clients turned HTTP status codes into errors in four separate places.

Constraints: no framework, no new runtime dependency, vanilla TypeScript with Vite. Every merge to `main` publishes a release, so the refactor must leave the visible behaviour of app and command line unchanged and go in as `refactor:`. The design rules in `DESIGN.md` stay untouched.

## Decision

1. `main.ts` is split by feature into modules. Shared helpers live in `lib/`, the shared state in a small `state.ts`.
2. The row gets two extension points built on one principle: **markup lives in the template in `index.html`, behaviour is registered by name.** `rowAction(name, …)` for menu entries and the main button, `rowSection(name, …)` for lines under the parcel. Existing code already uses both (every current menu entry, the problem line), so there is no empty framework.
3. Two dialog helpers: `ask(dialog)` as a promise instead of a variable plus `close` listener, and `setDialogState(dialog, state)` for dialogs with several states (today the command line).
4. The catalog import's pick list becomes `lib/picklist.ts`, independent of catalogs.
5. `style.css` is split along its existing sections into files under `styles/`; tokens stay in one place and the order stays exactly the same.
6. In the Go core: forge error mapping in one place, `store.Install` split into steps, `mac.Install` into unpack and place, `service.go` and `store.go` spread over files by topic.

Not part of the refactor: new fields, new methods, new dialogs, stubs for the features.

## Options considered

| | A: Keep one file | B: Modules by feature, two small registries | C: Components (Lit, Preact, custom elements) |
|---|---|---|---|
| Complexity | low today, high after four features | medium | high |
| Effort now | none | about 2 days | 5 days or more |
| Risk to behaviour | none | low, easy to verify | high (rebuilt UI) |
| New dependencies | none | none | yes, or a home-made component base |
| Readability of the features | poor | good | good |
| Fits `AGENTS.md` | yes | yes | no |

**A: Keep one file.**
- For: no risk, no effort.
- Against: `main.ts` would grow to an estimated 1,500 lines. Four features would edit the same functions (`createRow`, `fillRow`) and variables; parallel work ends in conflicts.

**B: Modules by feature (chosen).**
- For: each feature is one file plus one block in `index.html`. The registries show where to plug in. The refactor is mostly moving code, with little new code, and can be verified with screenshot comparison and an identical CSS bundle.
- Against: more files, and the registries are an indirection you have to understand once.

**C: Components.**
- For: encapsulated state, declarative rendering.
- Against: contradicts "no framework". The list's subgrid (`.parcel` takes the columns of `.parcels`) does not survive a shadow DOM between list and row. And it would be a rebuild rather than a refactor, so it could not be verified as behaviour-preserving.

For the Go core a general install pipeline with a step interface (`[]Step`) was also considered and rejected: there is exactly one order, and private functions called in that order read better than a list of objects.

## Trade-offs

- **Registries vs. direct calls.** Without registries `row.ts` would import every feature and every feature `row.ts`: cycles. With registries the dependency points one way: features know the row, the row knows no feature.
- **Markup in `index.html` rather than in code.** The template stays the one place where you see the row as a whole, and dialogs stay checkable HTML. The price: `index.html` keeps growing with each feature. Splitting it would take a Vite plugin, which is not worth it yet.
- **No frontend test framework.** Vitest would be a new dev dependency and a decision of its own. Verification is `tsc`, screenshot comparison and clicking through (see the verification plan).
- **Small helpers instead of a dialog system.** `ask` covers the three confirming dialogs. "Work while the dialog is open, cancellable" exists once so far (command line) and stays there until #28 needs it a second time.
- **Token lookup stays as it is.** `forge.New()` reads the GitHub token once at startup; Gitea reads per host with a cache. Signing in from the app needs a re-read. Unifying that changes startup behaviour and belongs to #28.

## Frontend structure

```
frontend/
  index.html                 templates and dialogs
  src/
    main.ts                  entry: the features' setup…(), then start
    backend.ts               the only module importing bindings and @wailsio/runtime
    state.ts                 apps, busy, onApps
    lib/
      dom.ts                 $, clone
      text.ts                errorText, plural
      dialog.ts              ask, setDialogState
      menu.ts                placeUnder
      picklist.ts            fillPickList, picked
    features/
      list.ts                loading, placeholders, drawing, summary, note
      row.ts                 build and fill a row, rowAction, rowSection
      install.ts             install, progress, "Alle aktualisieren"
      app-actions.ts         open, open release, channel
      add.ts                 the "add repository" form
      remove.ts              the "remove" dialog
      deeplink.ts            copy link, link dialog
      catalog.ts             catalog menu, export, import dialogs
      commandline.ts         the command line dialog
    styles/
      index.css              only @import, in this order:
      tokens.css  base.css  bar.css  controls.css  add.css  parcel.css
      menu.css  loading.css  dialog.css  catalog.css  commandline.css
      foot.css  responsive.css
```

### `backend.ts`

Re-exports, so the long binding paths stand in one place and a test setup replaces exactly one module:

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
export function findApp(repo: string): Status | undefined;   // case-insensitive
export function setApps(next: Status[]): void;                // calls every onApps listener
export function onApps(fn: (apps: Status[]) => void): void;
export const busy: Set<string>;                               // repos being installed right now
```

`findApp` always compares without case. Before, only the link code did; the row compared exactly. That is equivalent, because `config.Find` never allows two entries that differ only in case.

### `lib/dom.ts`, `lib/text.ts`, `lib/menu.ts`

```ts
export const $: <T extends Element>(sel: string, root?: ParentNode) => T;    // unchanged
export function clone<T extends Element>(template: HTMLTemplateElement): T;  // first element of the template

export function errorText(err: unknown): string;                             // unchanged
export function plural(n: number, one: string, many: string): string;        // unchanged

export function placeUnder(menu: HTMLElement, button: HTMLElement): void;    // unchanged
```

`clone` replaces the three `(tpl.content.cloneNode(true) as DocumentFragment).firstElementChild` (`showLoading`, `createRow`, `manifestRow`).

### `lib/dialog.ts`

```ts
// ask shows a modal dialog and resolves with the value of the button that
// closed it; "" for Esc. A question still open in the same dialog ends with "".
export function ask(dialog: HTMLDialogElement): Promise<string>;

// setDialogState sets dialog.dataset.state and shows only the elements whose
// data-show (space-separated) names the state.
export function setDialogState(dialog: HTMLDialogElement, state: string): void;
```

`ask` needs care because `close` fires late:

```ts
const pending = new WeakMap<HTMLDialogElement, (value: string) => void>();

export function ask(dialog: HTMLDialogElement): Promise<string> {
  if (!listening.has(dialog)) {
    listening.add(dialog);
    dialog.addEventListener("close", () => {
      if (dialog.open) return;          // the close of a replaced question
      const value = dialog.returnValue;
      dialog.returnValue = "";
      settle(dialog, value);
    });
  }
  settle(dialog, "");                   // an earlier question no longer counts
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

(`listening` is a `WeakSet<HTMLDialogElement>`.) Resetting `returnValue` follows modern-web-guidance's `dialog` guide: Esc sets no value, so without the reset an old one would remain.

This removes `removing`, `linked` and `planned`. One side effect, intended: before, if a second link arrived while the link dialog was open, the late `close` listener cleared `linked` and "Hinzufügen" in the new dialog did nothing. With `ask` every answer belongs to its question.

`setDialogState` takes over from `showCommand` the line `commandDialog.dataset.state = …` and the loop over `[data-show]`.

### `lib/picklist.ts`

```ts
export interface PickItem {
  value: string;        // the tick box's value, also its label
  state: string;        // the line's data-state: today "new", "known", "invalid"
  tag?: boolean;        // shows the template's tag ("Vorab")
  note?: string;        // after the label, or for "invalid" on a line of its own
  checked?: boolean;
  disabled?: boolean;
}
export function fillPickList(list: HTMLUListElement, items: PickItem[]): void;
export function picked(list: HTMLUListElement): string[];   // values: ticked and not disabled
```

`fillPickList` is `manifestRow` without catalog knowledge (template `#manifest-item`, classes `.manifest*` stay). `catalog.ts` maps `ImportItem` to `PickItem`: new → `{state: "new", checked: true}`, already on the list → `{state: "known", checked: true, disabled: true, note: "Schon auf der Liste"}`, broken → `{state: "invalid", disabled: true, note: it.error}`; `tag` is `channel === "prerelease" && !it.error`. `picked` replaces the selector in `chosen()`: known entries are disabled and so drop out, as before.

### `features/row.ts`

```ts
export interface RowAction {
  run(app: Status, row: HTMLLIElement): void;
  // on every fill: disable, tick or rename the menu entry
  update?(item: HTMLButtonElement, app: Status): void;
}
// name is the data-act of the menu entry in the template #row
export function rowAction(name: string, action: RowAction): void;

export interface RowSection {
  setup?(el: HTMLElement, row: HTMLLIElement): void;            // once per row
  fill(el: HTMLElement, app: Status, row: HTMLLIElement): void; // on every fill
}
// name is the data-section of a direct child of li.parcel in the template
export function rowSection(name: string, section: RowSection): void;

export function createRow(repo: string): HTMLLIElement;
export function fillRow(row: HTMLLIElement, app: Status): void;
export function findRow(repo: string): HTMLLIElement | null;  // replaces five querySelector calls with CSS.escape
export function showRowError(row: HTMLLIElement, message: string): void;
export function setBusy(row: HTMLLIElement, on: boolean): void;
export function flash(row: HTMLLIElement, text: string): void;
```

- `createRow`: the menu listener looks the action up by name instead of the `if` chain. The main button uses the same table with its `dataset.mode` (`"open"` or `"install"`), since both modes are action names too. Then `setup` of every section.
- `fillRow`: no longer sets menu entries itself (`[data-act="open"].disabled`, text and `disabled` of `release`, `aria-checked` of `channel`). Instead, at the end: `update` for every `[data-act]` of the row, `fill` for every `:scope > [data-section]`, then `setBusy`.
- The problem line becomes the first section: in the template `<p class="problem" data-section="problem" hidden>`, in `row.ts` `rowSection("problem", {fill: (_, app, row) => showRowError(row, app.error)})`.
- `showIcon` and the `icons` cache move along unchanged.
- Important: actions and sections must be registered before the first row is built. `main.ts` therefore calls every `setup…()` before `load()`.

### What moved where

| From `main.ts` | To |
|---|---|
| `$` | `lib/dom.ts` |
| `errorText`, `plural` | `lib/text.ts` |
| `placeUnder` | `lib/menu.ts` |
| `apps`, `busy` | `state.ts` |
| `MIN_LOADING_MS`, `COUNT_KEY`, `loaded`, `showLoading`, `load`, `render`, `noting`, `noteTimer`, `note`, listeners for `apps:changed` and `#refresh` | `features/list.ts` (exports `setupList`, `load`, `note`) |
| `rowCount`, `createRow`, `fillRow`, `icons`, `showIcon`, `showRowError`, `setBusy`, `flash` | `features/row.ts` |
| `install`, listeners for `install:progress` and `#update-all` | `features/install.ts` (exports `setupInstall`, `install`; registers `install`) |
| `open`, `setChannel`, menu branch `release` | `features/app-actions.ts` (registers `open`, `release`, `channel` with `update`) |
| listeners of `#add` and `#repo` | `features/add.ts` |
| `removing`, `confirmRemove`, `close` listener of `#confirm-remove` | `features/remove.ts` (registers `remove`, uses `ask`) |
| `copyLink`, `linked`, `takeLink`, `close` listener of `#confirm-link`, listener for `link:opened` | `features/deeplink.ts` (registers `link`, exports `takeLink`, uses `ask`) |
| `catalogMenu` up to the `close` listener of `#import`, without `note` and `manifestRow` | `features/catalog.ts` (uses `ask`, `fillPickList`, `picked`) |
| `manifestRow`, `chosen` | `lib/picklist.ts` |
| `commandDialog` up to `Events.On("command:open")` | `features/commandline.ts` (uses `setDialogState`) |
| `AppService.ConfigPath()…`, `load().then(takeLink)` | `main.ts` |

Details that are easy to lose when moving:

- `render(next)` used to set `apps` and draw. Now `setApps(next)` stands wherever `render(next)` was called with new data (`load`, `apps:changed`, after `Add` in the link dialog). `list.ts` hooks its drawing in with `onApps` and keeps the check `if (!loaded) return`. After five seconds `note` only redraws; it does not call `setApps`.
- `takeLink` calls `load()` when the list is empty; `deeplink.ts` imports it from `list.ts`. The link dialog writes errors to `#add-error`. That stays.
- `exportCatalog` uses the number of apps and `note`.
- In `commandline.ts`, `commandManual` was used before its declaration (it worked through call order). The `const` moved up.
- Every feature exports a `setup…()` and wires itself up only there. `main.ts`:

```ts
import "@fontsource-variable/archivo/wdth.css";
setupList(); setupInstall(); setupAppActions(); setupAdd(); setupRemove();
setupDeeplink(); setupCatalog(); setupCommandLine();
AppService.ConfigPath().then((p) => ($("#config-path").textContent = p));
load().then(takeLink);
```

### `index.html`

Content stays; two changes:

1. `<link rel="stylesheet" href="/src/styles/index.css">`.
2. `.problem` gets `data-section="problem"`.

Deferred, to keep the refactor easy to verify: grouping the file with comment headers (sprite, window, templates, then the dialogs per feature).

Count the tags after any change to it (see the verification plan): an earlier merge broke a dialog by dropping closing tags.

### CSS

Cut at the existing section headers of `style.css`, order unchanged:

| File | Content |
|---|---|
| `tokens.css` | header comment, `:root`, dark kraft textures |
| `base.css` | reset, `body`, focus, `.icon`, selection |
| `bar.css` | title bar |
| `controls.css` | buttons, fields, tick boxes |
| `add.css` | "add" strip, `.field-error` |
| `parcel.css` | `main`, list, stamp, tracking number, route, `.problem` |
| `menu.css` | menu |
| `loading.css` | empty list, placeholders |
| `dialog.css` | dialogs in general |
| `catalog.css` | `#import-url`, `#import`, `.manifest` |
| `commandline.css` | command line dialog |
| `foot.css` | footer |
| `responsive.css` | narrow windows, reduced motion |

One change of content, with no visible effect: in `parcel.css`, `grid-column: 1 / -1` moves from `.problem` to its own rule `.parcel > [data-section] { grid-column: 1 / -1; }` just before it. Any future line under the parcel then sits in the grid by itself. `tokens.css` stays the only file with colour and size values.

Vite resolves `@import` in CSS itself; no plugin is needed.

## Go structure

```
service.go        AppService, events, Apps, Add, Install, SetChannel, Remove, Open, Icon, ConfigPath, changed, remember
links.go          TakeLink, LinkFor, openLink, OpenURL, knownHost                 (new, from service.go)
commandline.go    CommandLine, InstallCommandLine, RemoveCommandLine, commandDone, commandTimeout   (new, from service.go)
translate.go      translate, translateAll                                         (new, from service.go)
catalog.go        unchanged

internal/forge/
  errors.go       ErrNotFound, ErrNoForge, ErrBadRepo, statusError                (new)
  forge.go        without the errors
  github.go       request uses statusError
  gitea.go        getOK, Releases and Download use statusError; giteaRelease.release()

internal/install/
  install_darwin.go   Install = unpack + place

internal/store/
  store.go        Store, New, Config, NormalizeRepo, validChannel
  list.go         Status, List, status, Updates                                   (new, from store.go)
  apps.go         Add, SetChannel, Remove, Open, installedPath                    (new, from store.go)
  install.go      Progress, Install, newest, installRelease, download, record     (new, from store.go)

cmd/lieferling/
  main.go         run only dispatches
  commands.go     listApps, updateApps, install                                   (new, from main.go)
```

### `internal/forge`

```go
// statusError turns an HTTP answer into nil or an error. who names the
// server in the message: "GitHub" or the host.
func statusError(who string, resp *http.Response) error
```

404 becomes `ErrNotFound`, anything from 300 up `"<who> answered <Status>: <start of body>"`. That replaces the check in `GitHub.request` and `answered` in `gitea.go`; the messages stay the same character for character, because `translate` and the tests match on them. `Gitea.Releases` keeps its special path (on 404 first ask `isForge`), then calls the same function.

```go
func (g giteaRelease) release() Release   // the mapping from the loop in Gitea.Releases
```

### `internal/install`

`mac.Install` becomes two private steps; the `Installer` interface stays:

```go
// unpacked is an app taken out of its download, not yet in place.
type unpacked struct {
	source string // the .app inside the mounted DMG or the unzipped folder
	name   string // bundle name without .app
	close  func() // detaches the DMG, then removes the work folder
}

func unpack(ctx context.Context, file, assetName string) (unpacked, error)
func (m *mac) place(ctx context.Context, u unpacked) (Result, error) // quit, ditto, xattr, mv, reopen
```

`Install` calls `unpack`, `defer u.close()`, `place`. Mind the cleanup order: first `hdiutil detach`, then `os.RemoveAll(work)`, as the `defer` order did before.

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

`Install` stays: normalize the repo, read the app from the list, lock, `newest`, `installRelease`. `newest` carries the two error messages from `Install` ("has no release in the … channel", "has nothing to install on … (one still being built has no files yet)"); they now name the repo as the list writes it. `status` keeps its own selection, because it still fills `Latest` when no asset matches and words its errors differently.

### `service.go`, `cmd/lieferling`

Only spread over files, no change to signatures: the bindings must come out identical. In `cmd/lieferling` the long `list` and `update` branches become functions in `commands.go`, next to `install`. `run` stays a `switch` in which each command is one line.

## Where the features plug in

### #28 Accounts

- **Error kinds:** new errors (`ErrNoToken`, `ErrNoAccess`, `ErrNoRelease`) in `forge/errors.go`, told apart in `statusError` (401/403 vs. 404, with and without token). The German sentence for each goes into `translate.go`. `Status` in `store/list.go` gets an error kind next to `Error`.
- **Icon without code access:** `store.repoIcon` treats `ErrNoAccess` like `ErrNotFound`.
- **Token:** `forge/token.go` (source, `Store`, `Forget`), plus `internal/account`. This is also where GitHub learns to re-read its token after signing in.
- **Service:** a new file `accounts.go` next to `links.go` with `Accounts`, `Login`, `SubmitToken`, `Logout`.
- **Frontend:** `features/accounts.ts`. The "Anmelden …" button goes into the `problem` section (markup in `#row`; its registration replaces the one in `row.ts`). The dialogs use `setDialogState` for their steps and `ask` for the overview. For waiting on the browser, `runCommand` in `commandline.ts` is the model; once it is needed a second time it moves to `lib/dialog.ts`. The pending install: `install(repo)` from `features/install.ts` after signing in.
- **Discovering repos:** `fillPickList` in the existing import dialog.
- **CLI:** two branches in `run`, functions in `commands.go`.

### #29 Delivery note

- **Forge:** `Body` on `Release` (GitHub through the JSON tag, Gitea in `giteaRelease.release()`). `PublishedAt` and `HTMLURL` exist already.
- **Core:** `internal/notes`, `store/notes.go` with `Notes` and `History`; `ReleasesBehind` and `Breaking` in `store/list.go`. The history is written in `store.record`.
- **Frontend:** `<details data-section="notes" hidden>` in `#row`, `features/notes.ts` with `rowSection("notes", {setup, fill})`. `fill` shows or hides the line and sets its summary, `setup` attaches the `toggle` listener for loading on demand (`toggle` does not bubble, hence per element). Styles in `styles/notes.css`.
- **HTML in the web view:** Go sanitizes. In the frontend, additionally, never `innerHTML` with raw text. Safari has no `setHTML()` (modern-web-guidance, `sanitize-untrusted-html`), so either insert only HTML that Go produced from a fixed list of elements, or build the nodes in the frontend from a structure. To be decided in #29.

### #30 Customs check

- **Digest while downloading:** in `store.download`, with `io.MultiWriter(file, sha256.New())`. `forge.Download` already takes an `io.Writer`, so the forge layer stays as it is. `Digest` on `Asset` like `Body` above.
- **Check between "unpacked" and "replace":** between `unpack` and `place`. `Installer.Install` gets a callback with the unpacked bundle for this, which `store.installRelease` fills with `internal/verify`. Whether quarantine stays becomes a parameter of `place`.
- **Policy and digests:** `config.App` and `config.Config`, written in `store.record`.
- **Frontend:** a second section `<details data-section="customs" hidden>` and `features/customs.ts`, same pattern as #29. The question on a sender change is an `ask` dialog.

### #31 Install another version

- **Core:** `pick.ByVersion`, `store.Versions`, and `InstallVersion` as a second caller of `installRelease` next to `Install`.
- **Frontend:** `<button data-act="versions">` in the menu of `#row`, `features/versions.ts` with `rowAction("versions", {run, update})` (`update` disables the entry for Lieferling itself). The dialog fills its list with `fillPickList`. For that `PickItem` gets the kind of box (`radio`) and a second tag; greyed-out releases are `disabled`.
- **CLI:** `install` in `commands.go` splits `repo@version`.

## Commits

All `refactor:`, each one builds and was checked on its own:

1. `refactor(frontend): split the stylesheet by section` (built CSS byte for byte identical)
2. `refactor(frontend): split main.ts into feature modules` (`backend.ts`, `state.ts`, `lib/*` with `ask` and the pick list, `features/*`, `rowAction`, `rowSection`, `setup…()`, the `[data-section]` rule). Three commits were planned here; intermediate states would have needed code that disappears right away.
3. `refactor(forge): map HTTP answers to errors in one place`
4. `refactor(install): unpack and place as separate steps`
5. `refactor(store): split the store by topic and the install into steps`
6. `refactor: split the service and the command line by topic`

## Verification plan

- **Go:** `go vet ./internal/... ./cmd/...` and `go test ./internal/... ./cmd/...` after every Go commit. `GOOS=darwin GOARCH=arm64 go build ./cmd/lieferling` and `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go vet -tags purego .`, because `install_darwin.go` and the app package do not compile on Linux otherwise.
- **Bindings:** `CGO_ENABLED=0 go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`, then `wails3 generate bindings -ts -i -clean=true`. `frontend/bindings` is not checked in; generate before and after the Go commits and compare with `diff -r`. There must be no difference.
- **Frontend:** `cd frontend && npm ci && npm run build` after every frontend commit.
- **CSS:** after the split alone, the built stylesheet in `frontend/dist/assets/` must be identical to `main`'s (same hash and content); only then move the `grid-column` rule.
- **Screenshot comparison:** no setup in the repo. In a scratch folder: a Vite config with `root` on `frontend/` that redirects `@wailsio/runtime` and the bindings to mocks with sample data through a `resolveId` plugin. Playwright and Chromium at `deviceScaleFactor: 2`: list light and dark, loading, every row menu, catalog menu, the remove, link, address and import dialogs, the command line dialog in all four states, narrow window. Once on `main`, once after the refactor, compare pixels. Emulate `prefers-reduced-motion` and wait before each shot.
- **Clicking through in the same setup:** the mocks record every call. Script: every menu entry of every row, the main button in all three states, "Alle aktualisieren", adding with valid and invalid input, removing with and without the tick, Esc in every dialog, importing with entries unticked, the events `apps:changed`, `install:progress`, `link:opened`, `catalog:menu`, `command:open`. The sequence of calls must be the same on `main` and after. Lost listeners are the main risk of the split, and this is what catches them.
- **`index.html`:** count opening and closing tags per element (`dialog`, `form`, `template`, `div`, `ul`, `li`, `p`) and check in the setup that `document.querySelectorAll("dialog").length` stays the same and no dialog sits inside another.

## Consequences

- From now on a feature is one file under `features/`, a block in `index.html`, a file under `styles/` and a `setup…()` in `main.ts`. `row.ts` does not need to change for #29, #30 or #31.
- Two features can be built in parallel without both changing the same functions. Points of contact remain `index.html` and `main.ts`, but line by line.
- The merge publishes a patch release with no visible change: `scripts/next-version.sh` counts everything but `feat` and breaking changes as a patch.
- The registries hide who handles an action. Remedy: the name in `data-act` or `data-section` is the same as in the call, so one search finds both.
- `index.html` keeps growing. If the new dialogs make it unwieldy, splitting it is a decision of its own.

## Open points

- Should the frontend get a test setup (such as the one from the verification plan, checked in)? It would be a new dev dependency, so it is not decided here.
