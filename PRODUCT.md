# Elappma

## What it is

A small app store for one person's own computers. You add GitHub repositories whose releases carry desktop apps (mostly Electron apps, often unsigned, often private), and Elappma installs them, keeps them up to date and tells you when something new is out.

## Who uses it

The owner of the repositories, on a few Macs signed in to the same iCloud account. Technically fluent, comfortable with the command line (there is a CLI with the same core), but wants the routine of "is there a new build, install it" to take no thought.

## What success looks like

- Adding a repository takes one field and one click.
- At a glance: which apps are installed here, which have an update, which are not installed on this Mac yet.
- One click installs or updates; errors say what went wrong and what to do (for example: a private repository needs a token).
- The list is the same on every Mac (iCloud Drive); what is installed stays per Mac.

## Surface and mode

One desktop window plus a menu bar icon. Mode: Operate. It is a utility that opens, gets a job done, and closes; it should feel at home next to macOS's own utilities.

## Constraints

- Runs in WKWebView (Wails v3) on macOS first, Windows and Linux later.
- UI language: German.
- Accessibility: full keyboard use, visible focus, WCAG AA contrast in light and dark.

## Assumptions

Written from the planning conversation without a separate interview: audience size (one person, a handful of apps), the German UI, and the "native utility" feel are inferred from it.
