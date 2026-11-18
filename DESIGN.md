---
name: Lieferling
description: A dispatch list for your own desktop apps, set like a parcel label.
colors:
  kraft: "light-dark(oklch(70% 0.075 68), oklch(37% 0.05 60))"
  kraft-ink: "light-dark(oklch(21% 0.03 55), oklch(95% 0.015 75))"
  kraft-line: "light-dark(oklch(60% 0.07 65), oklch(46% 0.05 60))"
  label-stock: "light-dark(oklch(99.2% 0 0), oklch(19% 0.006 60))"
  well: "light-dark(oklch(96.2% 0.002 60), oklch(23.5% 0.006 60))"
  ink: "light-dark(oklch(18% 0.01 60), oklch(95% 0.004 75))"
  ink-2: "light-dark(oklch(43% 0.01 60), oklch(74% 0.008 70))"
  cut-line: "light-dark(oklch(80% 0.006 60), oklch(36% 0.008 60))"
  express: "light-dark(oklch(53% 0.2 27), oklch(70% 0.17 27))"
  express-ink: "light-dark(oklch(99% 0 0), oklch(17% 0.04 27))"
  express-soft: "light-dark(oklch(95.5% 0.03 27), oklch(29% 0.07 27))"
  delivered: "light-dark(oklch(44% 0.09 152), oklch(76% 0.11 152))"
  pending: "light-dark(oklch(82% 0.004 60), oklch(38% 0.006 60))"
  focus: "light-dark(oklch(50% 0.14 250), oklch(76% 0.12 250))"
typography:
  headline:
    fontFamily: "system-ui, -apple-system, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 750
    letterSpacing: "-0.02em"
  title:
    fontFamily: "system-ui, -apple-system, sans-serif"
    fontSize: "1rem"
    fontWeight: 650
  tracking-number:
    fontFamily: "ui-monospace, 'SF Mono', Menlo, monospace"
    fontSize: "1rem"
    fontWeight: 600
    letterSpacing: "0.02em"
  body:
    fontFamily: "system-ui, -apple-system, sans-serif"
    fontSize: "0.875rem"
    lineHeight: 1.45
  label:
    fontFamily: "system-ui, -apple-system, sans-serif"
    fontSize: "0.75rem"
rounded:
  tag: "3px"
  item: "4px"
  stamp: "7px"
  control: "6px"
  sheet: "10px"
spacing:
  row: "1rem"
  gutter: "1rem"
  gap: "0.5rem"
components:
  button:
    backgroundColor: "{colors.label-stock}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "0.25rem 0.8rem"
  button-ink:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.label-stock}"
    rounded: "{rounded.control}"
  button-express:
    backgroundColor: "{colors.express}"
    textColor: "{colors.express-ink}"
    rounded: "{rounded.control}"
  button-on-kraft:
    backgroundColor: "transparent"
    textColor: "{colors.kraft-ink}"
    rounded: "{rounded.control}"
---

# Lieferling design system

## Overview

**Creative North Star: the parcel label.** Every app on the list is a shipment. The window is a dispatch list: a kraft-paper title bar, white label stock below, black ink, version numbers set like tracking numbers, and a three-stop tracking strip (Erschienen, Geladen, Installiert) that shows where each app stands. It is a macOS utility in Operate mode: calm when nothing is due, unmistakable when something is.

Mood: practical, printed, a little warm. Not a store front, not a dashboard, no cards, no gradients, no glass.

## Colors

### Primary
- **Kraft** (`kraft`): the title bar only, the paper the label sits on. Its own ink (`kraft-ink`) and rule (`kraft-line`) belong to it.

### Neutral
- **Label stock** (`label-stock`): the window ground. **Well** (`well`): pressed and hovered surfaces.
- **Ink** (`ink`) for text and the primary "Installieren" button, **Ink 2** (`ink-2`) for secondary text, **Cut line** (`cut-line`) for the dashed separators between parcels.

### Status
- **Express red** (`express`): appears only while an update is waiting or something failed. The new version number, the "Erschienen" stop and the "Aktualisieren" button turn red together.
- **Delivered green** (`delivered`): a completed tracking strip on an app that is current.
- **Pending** (`pending`): stops not yet reached.

### Named Rules
- **Red means action.** Express red is never decoration. If no update waits and nothing failed, there is no red on screen.
- **Kraft stays up top.** Kraft is the bar, never a row, button fill or highlight.

## Typography

System UI for everything a person reads, the mono stack for everything that is an identifier (versions, paths).

### Hierarchy
- **Headline** (1.25rem, 750): the window name in the bar.
- **Title** (1rem, 650): app name.
- **Tracking number** (mono 1rem, 600, +0.02em): installed → newest version.
- **Body** (0.875rem / 1.45): everything else.
- **Label** (0.75rem): repository, status text, footer; the route labels are 0.6875rem.

### Named Rules
- **Numbers are mono.** A version or path is always set in `--mono`; prose never is.

## Layout

One column of parcels on a four-row window grid: bar, add form, scrolling list, footer. Each parcel is a subgrid row with four columns: who (stamp, name, repo), shipment (number and route), action, more menu. Parcels are separated by dashed cut lines, not boxes. Below 640px the shipment moves to a second row, indented to line up with the name, and the bar summary hides. The bar leaves 5.5rem on the left for the macOS traffic lights and is the window's drag region.

## Elevation & Depth

Flat. The label lies on the table. Only things that float above the window cast a shadow: the more menu (popover) and the remove dialog.

## Shapes

Small radii: 6px for controls, 4px for menu items, 3px for the "Vorab" tag, 7px for the empty stamp field, 10px for the dialog sheet. Route stops are 9px dots.

## Components

### Buttons
Outlined on label stock by default; a current row uses it for "Öffnen", which starts the installed app (hidden on Lieferling's own row). Variants: **ink** (filled black, the one action on a row that is not current), **express** (filled red, "Aktualisieren" and "Alle aktualisieren"), **on-kraft** (outlined, in the bar), **danger** (red text, red-soft on hover), **icon-only** (borderless, the more button).

### Inputs
One text field with the hint in the placeholder, a checkbox for prerelease, an error line below that reserves its height so the form does not jump.

### Menu
A popover under the more button with "Öffnen" (disabled while the app is not installed here) above a rule, a checkable item (Vorabversionen verfolgen), a GitHub link and, after a rule, the destructive item.

### Stamp
Where a parcel label carries its postage, each parcel carries the app's own icon, 36px, left of the name. It comes from the installed .app bundle, or else from the icon file the repository builds the app from (such as `build/icon.png`); Lieferling keeps it in its cache and asks GitHub again after a week. Until it arrives, and for apps without one, the stamp field stays empty: a dashed `cut-line` frame on the macOS icon grid (inset a tenth on each side, so empty and filled stamps line up) with the app's initial in `ink-2`. A loaded icon fades in over the frame. No ring, shadow or tint is added to real icons; they bring their own shape.

### Tracking strip (signature)
An ordered list of three stops drawn as dots joined by legs. States come from `data-state` on the parcel: `current` (all green), `missing` (released in ink, rest pending), `update` (released in red), `failed` (strip hidden, "Kein Release erreichbar"). While installing, `data-busy` takes over: the first leg fills with download progress (`--progress`), the "Geladen" stop pulses, then the second leg half-fills during install. Motion stops under reduced motion.

### Loading parcels
Until the first list arrives, blank parcels stand on the same subgrid as the real rows and have the same height, as many as the list had last time (kept per Mac in local storage, 1 to 6, 3 the first time). The stamp, name, repository, number and stop labels are slips (`pending` mixed into label stock, 3px radius), the action is a dashed outline the size of a button. The one moving thing: a short `ink-2` segment travels along each blank tracking strip, rows a little out of step; under reduced motion it stands still in the middle. The bar reads "Sendungen werden abgerufen …". The placeholders stay at least 700 ms so a fast answer does not flicker, then the rows fade in from 3px below. Refreshing later keeps the current rows instead.

## Do's and Don'ts

### Do:
- Let the row state drive color through `data-state`; never hand-color a single element.
- Keep one action per row: filled when something is due, outlined "Öffnen" when the app is current.
- Write German, short, in full words ("Wird geladen …", not icons alone).

### Don't:
- Add cards, badges or counters to rows; the tracking strip already carries the state.
- Use express red or delivered green outside their meaning.
- Use kraft below the bar.
