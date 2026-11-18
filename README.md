# Elappma

Elappma installs and updates desktop apps from GitHub releases: a small app store for your own (often private, often unsigned) Electron apps. It is the install script, generalized: add a repository once, then install, update and remove its app with one click or one command.

- **App with menu bar icon:** a list of your apps with installed and newest version, one click to install or update. It checks for updates in the background (every 6 hours) and sends a notification when there are new ones.
- **Command line:** `elappma` does the same from the terminal.
- **Synced across your Macs:** the list of apps lives in iCloud Drive (`Elappma/config.json`), so every Mac signed in to the same iCloud account sees it. What is installed stays per Mac.

macOS only for now; Windows and Linux are prepared (see `internal/install`), not built.

## Install

Download the DMG from the [releases](https://github.com/pklaschka/elappma/releases) and drag Elappma into Applications. The app is not signed by Apple, so clear the quarantine flag once:

```sh
xattr -cr /Applications/Elappma.app
```

Add `pklaschka/elappma` to its own list and Elappma keeps itself up to date.

## Private repositories

Public repositories work without signing in. For private ones Elappma needs a GitHub token with read access to the repository's contents. It looks, in this order, at:

1. `GH_TOKEN` or `GITHUB_TOKEN`,
2. the GitHub CLI's login (`gh auth login`),
3. the Keychain item "Elappma GitHub":

   ```sh
   security add-generic-password -s "Elappma GitHub" -a github -w
   ```

The token is never synced; set it up on each Mac.

## Command line

```text
elappma list                         apps on the list, installed and newest version
elappma add <repo> [--prerelease]    put a repository on the list
elappma install <repo>               install (adds it to the list if needed)
elappma update [<repo>...]           update the given apps, or every app with an update
elappma channel <repo> stable|prerelease
elappma remove <repo> [--uninstall]  take it off the list (and off this computer)
elappma paths                        where the shared and the local files are
```

`<repo>` is `owner/name` or the repository's GitHub URL.

## How it picks what to install

- **Release:** the newest non-draft release; on the `stable` channel release candidates (GitHub "pre-releases") are skipped.
- **File:** the release asset that fits this Mac. A `.dmg` beats a `.zip`; a file naming this processor (`arm64`, `x64`, …) beats a `universal` one, which beats one naming no processor. On Apple Silicon an Intel-only build is the last resort (it runs through Rosetta). Files naming another system are never picked, and a bare `.zip` only when it names macOS.
- **Install:** like the original script. Quit the running app (it may ask about unsaved work), replace the copy in `/Applications` and clear the quarantine flag. When `/Applications` is not writable, Elappma asks for an administrator password.

If two Macs change the list at the same time, iCloud keeps both files ("config 2.json"); Elappma merges them the next time it writes. An app removed on one Mac can come back that way, so remove it again.

## Development

Requires Go 1.26, Node 22 and the Wails CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`).

```sh
wails3 dev                         # the app with hot reload
go test ./internal/... ./cmd/...   # the core
go run ./cmd/elappma list          # the command line
wails3 task darwin:package         # Elappma.app in bin/ (on a Mac)
```

`ELAPPMA_CONFIG`, `ELAPPMA_STATE` and `ELAPPMA_APPLICATIONS` move the shared list, the local state and the install folder, which helps when trying things out.

Releases: push a tag like `v0.1.0`; the release workflow builds the universal DMG and the command line and attaches them to the release.

## Layout

| Path | What |
|---|---|
| `internal/github` | GitHub releases and downloads, token lookup |
| `internal/pick` | channel, version and asset choice |
| `internal/config` | the synced list (iCloud Drive) |
| `internal/state` | what is installed on this computer |
| `internal/install` | putting an app in place (macOS so far) |
| `internal/store` | the core both front ends use |
| `cmd/elappma` | the command line |
| `main.go`, `service.go`, `frontend/` | the Wails app |
