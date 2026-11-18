---
version: 1
slug: "frontend-index-html"
primary_target: "frontend/index.html"
related_targets: []
---

# Surface: Elappma main window

Scope: the app window (list, add, install/update, remove). Mode: Operate.
Audience and job: one person, a few Macs; see which builds are new, install them in one click.
Constraints: German UI, WKWebView (Safari 17.5+), light and dark follow the system, keyboard and AA contrast.

## Direction contract

THESIS: Every app is a shipment from its GitHub repository to this Mac; the window is a dispatch list whose state is the release's journey. It refuses the App Store's icon-tile grid and the grey settings list.

OWN-WORLD: Parcel label: kraft-brown title bar, white label-stock rows with black ink and hairline cut lines, versions set in mono like tracking numbers, express red only for "update waiting", a dark delivered green for "installed". Restrained otherwise; system sans for everything else.

STORY: The visitor sees at once which shipments are delivered, which have a newer one waiting, which never arrived here; installs with one click and watches it travel.

FIRST VIEWPORT: Kraft title bar (name, status counts, check button). Below, an add strip. Then the label rows: name and repo left, tracking number centre, three-stop tracking strip (Erschienen, Geladen, Installiert), action right. Primary action: the row's Install/Update button; "Alle aktualisieren" in the bar when several wait.

FORM: parcel label (candidate 3 of 7); seed:6566657ab296c096

Signature interaction: the tracking strip; during install the line between Erschienen and Geladen fills with the download, then Installiert lands.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
