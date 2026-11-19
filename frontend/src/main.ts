import {Clipboard, Events} from "@wailsio/runtime";
import {AppService} from "../bindings/github.com/pklaschka/lieferling";
import {type Command, CommandState} from "../bindings/github.com/pklaschka/lieferling/internal/install/models";
import type {Link} from "../bindings/github.com/pklaschka/lieferling/internal/deeplink/models";
import type {ImportItem, ImportPlan, Status} from "../bindings/github.com/pklaschka/lieferling/internal/store/models";

const $ = <T extends Element>(sel: string, root: ParentNode = document) => root.querySelector(sel) as T;

const list = $<HTMLUListElement>("#apps");
const loading = $<HTMLUListElement>("#loading");
const ghostTemplate = $<HTMLTemplateElement>("#ghost");
const empty = $<HTMLDivElement>("#empty");
const summary = $<HTMLParagraphElement>("#summary");
const refreshButton = $<HTMLButtonElement>("#refresh");
const updateAllButton = $<HTMLButtonElement>("#update-all");
const rowTemplate = $<HTMLTemplateElement>("#row");
const addForm = $<HTMLFormElement>("#add");
const repoInput = $<HTMLInputElement>("#repo");
const addError = $<HTMLParagraphElement>("#add-error");
const removeDialog = $<HTMLDialogElement>("#confirm-remove");
const linkDialog = $<HTMLDialogElement>("#confirm-link");

let apps: Status[] = [];
let noting = false; // the bar shows a passing note instead of the summary
const busy = new Set<string>(); // repos being installed right now

function errorText(err: unknown): string {
  const msg = err instanceof Error ? err.message : String(err);
  return msg.replace(/^[a-z]+:\s*/i, "") || "Unbekannter Fehler";
}

function plural(n: number, one: string, many: string) {
  return `${n} ${n === 1 ? one : many}`;
}

// ---- list -----------------------------------------------------------------

// The first load asks the forges about every app and can take a moment. Until
// they answer, placeholder parcels stand where the rows will be, as many as last
// time, and they stay at least this long so the window does not flicker.
const MIN_LOADING_MS = 700;
const COUNT_KEY = "lieferling:count";
let loaded = false;

function showLoading() {
  let count = 3;
  try {
    const saved = Number(localStorage.getItem(COUNT_KEY));
    if (Number.isFinite(saved) && localStorage.getItem(COUNT_KEY) !== null) count = saved;
  } catch {
    // no storage: keep the default
  }
  count = Math.min(Math.max(count, 1), 6);
  const ghosts = Array.from({length: count}, (_, i) => {
    const li = (ghostTemplate.content.cloneNode(true) as DocumentFragment).firstElementChild as HTMLLIElement;
    li.style.setProperty("--i", String(i));
    return li;
  });
  loading.replaceChildren(...ghosts);
  loading.hidden = false;
  summary.textContent = "Sendungen werden abgerufen …";
}

async function load() {
  refreshButton.setAttribute("aria-busy", "true");
  refreshButton.disabled = true;
  const first = !loaded;
  if (first) showLoading();
  const shown = performance.now();
  try {
    const next = (await AppService.Apps()) ?? [];
    if (first) {
      await new Promise((r) => setTimeout(r, MIN_LOADING_MS - (performance.now() - shown)));
      list.dataset.arrive = "";
      setTimeout(() => delete list.dataset.arrive, 1200);
    }
    loaded = true;
    loading.hidden = true;
    render(next);
    try {
      localStorage.setItem(COUNT_KEY, String(next.length));
    } catch {
      // only a convenience
    }
  } catch (err) {
    loading.hidden = true;
    summary.textContent = `Liste nicht lesbar: ${errorText(err)}`;
  } finally {
    refreshButton.removeAttribute("aria-busy");
    refreshButton.disabled = false;
    list.setAttribute("aria-busy", "false");
  }
}

function render(next: Status[]) {
  apps = next;
  if (!loaded) return; // the placeholders stay until the first load is done
  empty.hidden = apps.length > 0;
  list.hidden = apps.length === 0;

  const updates = apps.filter((a) => a.updateAvailable);
  if (!noting) summary.textContent = apps.length === 0 ? "" :
    updates.length === 0 ? `${plural(apps.length, "App", "Apps")}, alle aktuell` :
      `${plural(apps.length, "App", "Apps")}, ${plural(updates.length, "Update", "Updates")}`;
  updateAllButton.hidden = updates.length < 2;

  const rows = new Map<string, HTMLLIElement>();
  for (const li of list.querySelectorAll<HTMLLIElement>("li.parcel")) rows.set(li.dataset.repo!, li);
  const order: HTMLLIElement[] = [];
  for (const app of apps) {
    const li = rows.get(app.repo) ?? createRow(app.repo);
    rows.delete(app.repo);
    fillRow(li, app);
    order.push(li);
  }
  for (const stale of rows.values()) stale.remove();
  list.replaceChildren(...order);
}

let rowCount = 0;

// placeUnder keeps a popover menu under its button, right-aligned (anchor
// positioning is not in every WebKit yet).
function placeUnder(menu: HTMLElement, button: HTMLElement) {
  menu.addEventListener("beforetoggle", (e) => {
    if ((e as ToggleEvent).newState !== "open") return;
    const r = button.getBoundingClientRect();
    const {clientWidth, clientHeight} = document.documentElement;
    // below the button, or above it when the window is too short there
    const space = clientHeight - r.bottom;
    const below = space >= 180 || space >= r.top;
    Object.assign(menu.style, {
      top: below ? `${r.bottom + 4}px` : "auto",
      bottom: below ? "auto" : `${clientHeight - r.top + 4}px`,
      left: "auto",
      right: `${clientWidth - r.right}px`,
    });
  });
}

function createRow(repo: string): HTMLLIElement {
  const li = (rowTemplate.content.cloneNode(true) as DocumentFragment).firstElementChild as HTMLLIElement;
  li.dataset.repo = repo;
  const id = `menu-${++rowCount}`;
  const menu = $<HTMLDivElement>(".menu", li);
  const more = $<HTMLButtonElement>(".more", li);
  menu.id = id;
  more.setAttribute("popovertarget", id);
  more.setAttribute("aria-label", `Mehr zu ${repo}`);

  placeUnder(menu, more);
  menu.addEventListener("click", (e) => {
    const act = (e.target as HTMLElement).closest<HTMLButtonElement>("button")?.dataset.act;
    menu.hidePopover();
    const app = apps.find((a) => a.repo === li.dataset.repo);
    if (!app) return;
    if (act === "open") open(li, app);
    if (act === "release" && app.releaseUrl) AppService.OpenURL(app.releaseUrl);
    if (act === "remove") confirmRemove(app);
    if (act === "channel") setChannel(li, app.channel !== "prerelease");
    if (act === "link") copyLink(li, app);
  });

  $<HTMLAnchorElement>(".repo", li).addEventListener("click", async (e) => {
    e.preventDefault();
    const href = (e.currentTarget as HTMLAnchorElement).href;
    if (!href) return;
    try {
      await AppService.OpenURL(href);
    } catch (err) {
      showRowError(li, errorText(err));
    }
  });

  $<HTMLButtonElement>(".action", li).addEventListener("click", (e) => {
    const app = apps.find((a) => a.repo === li.dataset.repo);
    if ((e.currentTarget as HTMLElement).dataset.mode === "open") {
      if (app) open(li, app);
    } else install(li.dataset.repo!);
  });
  return li;
}

async function setChannel(li: HTMLLIElement, prerelease: boolean) {
  const item = $<HTMLButtonElement>('[data-act="channel"]', li);
  item.setAttribute("aria-checked", String(prerelease));
  try {
    await AppService.SetChannel(li.dataset.repo!, prerelease);
  } catch (err) {
    item.setAttribute("aria-checked", String(!prerelease));
    showRowError(li, errorText(err));
  }
}

function fillRow(li: HTMLLIElement, app: Status) {
  const state = app.error && !app.latest ? "failed" :
    !app.installed ? "missing" : app.updateAvailable ? "update" : "current";
  li.dataset.state = state;
  $(".name", li).textContent = app.name;
  showIcon(li, app);
  const repo = $<HTMLAnchorElement>(".repo", li);
  repo.textContent = app.repo;
  if (app.repoUrl) repo.href = app.repoUrl;
  else repo.removeAttribute("href");
  repo.title = app.repoUrl ? `${app.repoUrl} im Browser öffnen` : "";
  $(".installed", li).textContent = app.installed || "–";
  $(".latest", li).textContent = app.latest || "Kein Release erreichbar";
  $<HTMLElement>(".channel-tag", li).hidden = app.channel !== "prerelease";
  $('[data-act="channel"]', li).setAttribute("aria-checked", String(app.channel === "prerelease"));
  const route = $<HTMLOListElement>(".route", li);
  route.setAttribute("aria-label", {
    update: `Version ${app.latest} ist erschienen, installiert ist ${app.installed}`,
    current: `Version ${app.installed} ist installiert`,
    missing: `Version ${app.latest} ist erschienen, auf diesem Mac nicht installiert`,
    failed: "Kein Release gefunden",
  }[state]);

  const action = $<HTMLButtonElement>(".action", li);
  action.className = "button action";
  action.disabled = false;
  if (state === "update") {
    action.textContent = "Aktualisieren";
    action.classList.add("express");
  } else if (state === "current") {
    action.textContent = "Öffnen";
  } else {
    action.textContent = "Installieren";
    action.classList.add("ink");
  }
  action.dataset.mode = state === "current" ? "open" : "install";
  // opening the window you are looking at does nothing
  action.dataset.unavailable = String(state === "failed" || state === "current" && app.self);
  action.setAttribute("aria-label", `${action.textContent!.trim()}: ${app.name}`);

  $<HTMLButtonElement>('[data-act="open"]', li).disabled = !app.installed || app.self;
  const releaseItem = $<HTMLButtonElement>('[data-act="release"]', li);
  releaseItem.disabled = !app.releaseUrl;
  releaseItem.textContent = `Release auf ${app.forge || "GitHub"} öffnen`;

  showRowError(li, app.error);
  setBusy(li, busy.has(app.repo));
}

// Icons come from the installed bundle or the repository and can take a
// moment; the stamp field with the initial stands in until then, and for
// good when an app has none. Asking again only when the app was installed,
// moved or removed keeps refreshes free.
const icons = new Map<string, Promise<string>>();

function showIcon(li: HTMLLIElement, app: Status) {
  const initial = $<HTMLSpanElement>(".initial", li);
  initial.textContent = Array.from(app.name.trim())[0]?.toLocaleUpperCase("de") ?? "?";
  const key = `${app.repo}\n${app.path}`;
  if (li.dataset.icon === key) return;
  li.dataset.icon = key;
  let url = icons.get(key);
  if (!url) {
    url = AppService.Icon(app.repo).catch(() => "");
    icons.set(key, url);
  }
  url.then((src) => {
    if (li.dataset.icon !== key) return; // the app changed meanwhile
    const stamp = $<HTMLSpanElement>(".stamp", li);
    const shown = stamp.querySelector("img");
    if (shown?.getAttribute("src") === src) return;
    shown?.remove();
    delete li.dataset.hasIcon;
    if (!src) return;
    // created only once there is a source, so no empty image is ever shown
    const img = new Image(36, 36);
    img.alt = "";
    img.decoding = "async";
    img.onload = () => (li.dataset.hasIcon = "");
    img.onerror = () => img.remove();
    img.src = src;
    stamp.append(img);
  });
}

function showRowError(li: HTMLLIElement, message: string) {
  const el = $<HTMLParagraphElement>(".problem", li);
  el.hidden = !message;
  $("span", el).textContent = message;
}

function setBusy(li: HTMLLIElement, on: boolean) {
  const action = $<HTMLButtonElement>(".action", li);
  action.hidden = on || action.dataset.unavailable === "true";
  const route = $<HTMLOListElement>(".route", li);
  const status = $<HTMLSpanElement>(".status", li);
  status.hidden = !on;
  if (on) {
    li.dataset.busy ??= "download";
    route.setAttribute("aria-busy", "true");
    status.textContent = li.dataset.busy === "install" ? "Wird installiert …" : "Wird geladen …";
  } else {
    delete li.dataset.busy;
    route.style.removeProperty("--progress");
    route.removeAttribute("aria-busy");
  }
}

// ---- actions --------------------------------------------------------------

async function open(li: HTMLLIElement, app: Status) {
  showRowError(li, "");
  try {
    await AppService.Open(app.repo);
    flash(li, "Wird geöffnet …");
  } catch (err) {
    showRowError(li, errorText(err));
  }
}

async function install(repo: string) {
  const li = list.querySelector<HTMLLIElement>(`li[data-repo="${CSS.escape(repo)}"]`);
  if (!li || busy.has(repo)) return;
  busy.add(repo);
  showRowError(li, "");
  setBusy(li, true);
  try {
    await AppService.Install(repo);
  } catch (err) {
    showRowError(li, errorText(err));
  } finally {
    busy.delete(repo);
    setBusy(li, false);
  }
}

Events.On("install:progress", (ev) => {
  const {repo, done, total} = ev.data;
  const li = list.querySelector<HTMLLIElement>(`li[data-repo="${CSS.escape(repo)}"]`);
  if (!li || total <= 0) return;
  $<HTMLOListElement>(".route", li).style.setProperty("--progress", String(done / total));
  li.dataset.busy = done >= total ? "install" : "download";
  $(".status", li).textContent = done >= total ? "Wird installiert …" : `Wird geladen … ${Math.round((done / total) * 100)} %`;
});

Events.On("apps:changed", (ev) => render(ev.data ?? []));

refreshButton.addEventListener("click", load);

updateAllButton.addEventListener("click", async () => {
  updateAllButton.disabled = true;
  for (const app of apps.filter((a) => a.updateAvailable)) await install(app.repo);
  updateAllButton.disabled = false;
});

addForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!repoInput.checkValidity()) {
    addError.textContent = repoInput.validity.valueMissing ?
      "Gib ein Repository an, zum Beispiel pklaschka/feldspur." :
      "Das sieht nicht nach einem Repository aus. Erwartet wird owner/name für GitHub, host/owner/name für Forgejo und Gitea oder die URL.";
    repoInput.setAttribute("aria-invalid", "true");
    repoInput.focus();
    return;
  }
  const submit = $<HTMLButtonElement>('button[type="submit"]', addForm);
  submit.disabled = true;
  addError.textContent = "";
  try {
    const prerelease = (addForm.elements.namedItem("prerelease") as HTMLInputElement).checked;
    await AppService.Add(repoInput.value, prerelease);
    addForm.reset();
    repoInput.removeAttribute("aria-invalid");
  } catch (err) {
    addError.textContent = errorText(err);
    repoInput.setAttribute("aria-invalid", "true");
  } finally {
    submit.disabled = false;
  }
});

repoInput.addEventListener("input", () => {
  if (repoInput.getAttribute("aria-invalid") === "true" && repoInput.checkValidity()) {
    repoInput.removeAttribute("aria-invalid");
    addError.textContent = "";
  }
});

let removing: Status | undefined;

function confirmRemove(app: Status) {
  removing = app;
  $("#remove-title", removeDialog).textContent = `${app.name} von der Liste entfernen?`;
  const uninstall = $<HTMLInputElement>('input[name="uninstall"]', removeDialog);
  uninstall.checked = false;
  uninstall.closest("label")!.hidden = !app.managedByLieferling;
  removeDialog.showModal();
}

removeDialog.addEventListener("close", async () => {
  const choice = removeDialog.returnValue;
  removeDialog.returnValue = "";
  const app = removing;
  removing = undefined;
  if (choice !== "remove" || !app) return;
  const uninstall = $<HTMLInputElement>('input[name="uninstall"]', removeDialog).checked;
  try {
    await AppService.Remove(app.repo, uninstall);
  } catch (err) {
    const li = list.querySelector<HTMLLIElement>(`li[data-repo="${CSS.escape(app.repo)}"]`);
    if (li) showRowError(li, errorText(err));
  }
});

// ---- deep links ------------------------------------------------------------

async function copyLink(li: HTMLLIElement, app: Status) {
  try {
    const url = await AppService.LinkFor(app.repo, true);
    await Clipboard.SetText(`[In Lieferling installieren](${url})`);
    flash(li, "Link kopiert");
  } catch (err) {
    showRowError(li, errorText(err));
  }
}

// flash shows a short note in the row's status slot.
function flash(li: HTMLLIElement, text: string) {
  if (busy.has(li.dataset.repo!)) return;
  const status = $<HTMLSpanElement>(".status", li);
  const action = $<HTMLButtonElement>(".action", li);
  status.textContent = text;
  status.hidden = false;
  action.hidden = true;
  setTimeout(() => {
    if (busy.has(li.dataset.repo!)) return;
    status.hidden = true;
    action.hidden = action.dataset.unavailable === "true";
  }, 1800);
}

let linked: Link | undefined;

async function takeLink() {
  const link = await AppService.TakeLink();
  if (!link) return;
  if (apps.length === 0) await load();
  const known = apps.find((a) => a.repo.toLowerCase() === link.repo.toLowerCase());
  const row = known && list.querySelector<HTMLLIElement>(`li[data-repo="${CSS.escape(known.repo)}"]`);
  // already there and nothing to do: show it
  if (known && (!link.install || known.installed && !known.updateAvailable)) {
    row?.scrollIntoView({block: "nearest"});
    row && flash(row, "Schon auf der Liste");
    return;
  }
  linked = link;
  const install = $<HTMLInputElement>('input[name="install"]', linkDialog);
  const prerelease = $<HTMLInputElement>('input[name="prerelease"]', linkDialog);
  install.checked = link.install;
  prerelease.checked = link.prerelease;
  $<HTMLElement>('[data-field="install"]', linkDialog).hidden = !!known;
  $<HTMLElement>('[data-field="prerelease"]', linkDialog).hidden = !!known;
  $("#link-title", linkDialog).textContent = known ?
    `${known.name} installieren?` : `${link.repo} zu Lieferling hinzufügen?`;
  $('button[value="ok"]', linkDialog).textContent = known ? "Installieren" : "Hinzufügen";
  if (linkDialog.open) linkDialog.close();
  linkDialog.returnValue = "";
  linkDialog.showModal();
}

linkDialog.addEventListener("close", async () => {
  const link = linked;
  linked = undefined;
  if (linkDialog.returnValue !== "ok" || !link) return;
  const known = apps.some((a) => a.repo.toLowerCase() === link.repo.toLowerCase());
  const wanted = known || $<HTMLInputElement>('input[name="install"]', linkDialog).checked;
  try {
    if (!known) {
      const prerelease = $<HTMLInputElement>('input[name="prerelease"]', linkDialog).checked;
      const added = await AppService.Add(link.repo, prerelease);
      render([...apps.filter((a) => a.repo !== added!.repo), added!]);
    }
    const app = apps.find((a) => a.repo.toLowerCase() === link.repo.toLowerCase());
    if (wanted && app) await install(app.repo);
  } catch (err) {
    addError.textContent = errorText(err);
  }
});

Events.On("link:opened", takeLink);

// ---- catalogs ---------------------------------------------------------------

const catalogMenu = $<HTMLDivElement>("#catalog-menu");
const catalogButton = $<HTMLButtonElement>("#catalog-button");
const urlDialog = $<HTMLDialogElement>("#import-url");
const urlForm = $<HTMLFormElement>("#url-form");
const urlInput = $<HTMLInputElement>("#catalog-url");
const urlError = $<HTMLParagraphElement>("#url-error");
const importDialog = $<HTMLDialogElement>("#import");
const manifest = $<HTMLUListElement>(".manifest", importDialog);
const manifestTemplate = $<HTMLTemplateElement>("#manifest-item");

placeUnder(catalogMenu, catalogButton);

let planned: ImportPlan | undefined;
let catalogBusy = false;

function catalogAction(act: string | undefined) {
  if (catalogBusy || importDialog.open || urlDialog.open) return;
  if (act === "import-file") chooseCatalog();
  if (act === "import-url") askForURL();
  if (act === "export") exportCatalog();
}

catalogMenu.addEventListener("click", (e) => {
  const act = (e.target as HTMLElement).closest<HTMLButtonElement>("button")?.dataset.catalog;
  catalogMenu.hidePopover();
  catalogAction(act);
});
for (const b of empty.querySelectorAll<HTMLButtonElement>("[data-catalog]")) {
  b.addEventListener("click", () => catalogAction(b.dataset.catalog));
}
Events.On("catalog:menu", (ev) => catalogAction(ev.data));

// note shows a passing message in the bar, then the usual summary again.
let noteTimer: number | undefined;
function note(text: string) {
  clearTimeout(noteTimer);
  noting = true;
  summary.dataset.note = "";
  summary.textContent = text;
  noteTimer = setTimeout(() => {
    noting = false;
    delete summary.dataset.note;
    render(apps);
  }, 5000);
}

async function exportCatalog() {
  catalogBusy = true;
  try {
    const path = await AppService.ExportCatalog();
    if (path) note(`${plural(apps.length, "App", "Apps")} als Katalog gesichert: ${path.split("/").pop()}`);
  } catch (err) {
    note(`Nicht gesichert: ${errorText(err)}`);
  } finally {
    catalogBusy = false;
  }
}

async function chooseCatalog() {
  catalogBusy = true;
  try {
    const plan = await AppService.ChooseCatalog();
    if (plan) showPlan(plan);
  } catch (err) {
    note(`Katalog nicht lesbar: ${errorText(err)}`);
  } finally {
    catalogBusy = false;
  }
}

function askForURL() {
  urlForm.reset();
  setURLError("");
  urlDialog.showModal();
}

function setURLError(message: string) {
  urlError.textContent = message;
  if (message) {
    urlInput.setAttribute("aria-invalid", "true");
    urlInput.setAttribute("aria-describedby", "url-note url-error");
  } else {
    urlInput.setAttribute("aria-invalid", "false");
    urlInput.setAttribute("aria-describedby", "url-note");
  }
}

urlDialog.querySelector("[data-close]")!.addEventListener("click", () => urlDialog.close());

urlForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!urlInput.checkValidity()) {
    setURLError(urlInput.validity.valueMissing ?
      "Gib die Adresse des Katalogs an." :
      "Das ist keine https-Adresse. Erwartet wird etwas wie https://example.com/apps.json.");
    urlInput.focus();
    return;
  }
  const submit = $<HTMLButtonElement>('button[type="submit"]', urlForm);
  submit.disabled = true;
  submit.textContent = "Wird geladen …";
  setURLError("");
  try {
    const plan = await AppService.CatalogFromURL(urlInput.value);
    urlDialog.close();
    if (plan) showPlan(plan);
  } catch (err) {
    setURLError(errorText(err));
    urlInput.focus();
  } finally {
    submit.disabled = false;
    submit.textContent = "Laden";
  }
});

urlInput.addEventListener("input", () => {
  if (urlInput.getAttribute("aria-invalid") === "true" && urlInput.checkValidity()) setURLError("");
});

function showPlan(plan: ImportPlan) {
  planned = plan;
  const items = plan.items ?? [];
  const fresh = items.filter((it) => !it.onList && !it.error);
  const known = items.filter((it) => it.onList);
  const bad = items.filter((it) => it.error);

  const catalogName = plan.name ? `»${plan.name}«` : "";
  $("#import-title", importDialog).textContent = fresh.length === 0 ?
    `Nichts Neues im Katalog ${catalogName}`.trim() :
    `${plural(fresh.length, "App", "Apps")} aus dem Katalog ${catalogName} übernehmen?`.replace("  ", " ");
  $(".import-source .path", importDialog).textContent = plan.source;

  const rows = [...fresh, ...known, ...bad].map((it) => manifestRow(it));
  manifest.replaceChildren(...rows);

  const parts: string[] = [];
  if (known.length) parts.push(`${plural(known.length, "App steht", "Apps stehen")} schon auf der Liste und ${known.length === 1 ? "bleibt" : "bleiben"}, wie ${known.length === 1 ? "sie ist" : "sie sind"}.`);
  if (bad.length) parts.push(`${plural(bad.length, "Eintrag ist", "Einträge sind")} nicht lesbar.`);
  if (fresh.length) parts.push("Füge nur Repositories hinzu, deren Apps du vertraust. Installiert wird erst, wenn du es in der Liste sagst.");
  $("#import-note", importDialog).textContent = parts.join(" ");

  const ok = $<HTMLButtonElement>('button[value="ok"]', importDialog);
  const cancel = $<HTMLButtonElement>('button[value="cancel"]', importDialog);
  ok.hidden = fresh.length === 0;
  cancel.textContent = fresh.length === 0 ? "Schließen" : "Abbrechen";
  updateImportButton();
  importDialog.returnValue = "";
  importDialog.showModal();
  manifest.scrollTop = 0;
}

function manifestRow(it: ImportItem): HTMLLIElement {
  const li = (manifestTemplate.content.cloneNode(true) as DocumentFragment).firstElementChild as HTMLLIElement;
  const box = $<HTMLInputElement>("input", li);
  box.value = it.repo;
  $(".manifest-repo", li).textContent = it.repo;
  $<HTMLElement>(".channel-tag", li).hidden = it.channel !== "prerelease" || !!it.error;
  const noteEl = $(".manifest-note", li);
  if (it.error) {
    li.dataset.state = "invalid";
    box.disabled = true;
    noteEl.textContent = it.error;
  } else if (it.onList) {
    li.dataset.state = "known";
    box.disabled = true;
    box.checked = true;
    noteEl.textContent = "Schon auf der Liste";
  } else {
    li.dataset.state = "new";
    box.checked = true;
  }
  return li;
}

function chosen(): ImportItem[] {
  const repos = new Set([...manifest.querySelectorAll<HTMLInputElement>('li[data-state="new"] input:checked')].map((b) => b.value));
  return (planned?.items ?? []).filter((it) => repos.has(it.repo) && !it.onList && !it.error);
}

function updateImportButton() {
  const n = chosen().length;
  const ok = $<HTMLButtonElement>('button[value="ok"]', importDialog);
  ok.textContent = n === 0 ? "Hinzufügen" : `${plural(n, "App", "Apps")} hinzufügen`;
  ok.disabled = n === 0;
}

manifest.addEventListener("change", updateImportButton);

importDialog.addEventListener("close", async () => {
  const items = chosen();
  const choice = importDialog.returnValue;
  importDialog.returnValue = "";
  planned = undefined;
  if (choice !== "ok" || items.length === 0) return;
  try {
    const added = await AppService.ImportCatalog(items);
    note(added === 0 ? "Keine neuen Apps hinzugefügt" : `${plural(added, "App", "Apps")} aus dem Katalog hinzugefügt`);
  } catch (err) {
    note(`Nicht übernommen: ${errorText(err)}`);
  }
});
// ---- command line ----------------------------------------------------------

const commandDialog = $<HTMLDialogElement>("#command");
const commandError = $<HTMLParagraphElement>(".field-error", commandDialog);
const commandInstall = $<HTMLButtonElement>('button[value="install"]', commandDialog);
const commandRemove = $<HTMLButtonElement>('button[value="remove"]', commandDialog);

const commandCopy: Record<string, {title: string; note: string; cancel?: string; ok?: string; done?: string}> = {
  [CommandState.CommandMissing]: {
    title: "Kommandozeilen-Tool installieren",
    note: "Mit dem Befehl lieferling erledigst du im Terminal alles, was dieses Fenster kann. " +
      "Lieferling legt dafür einen Verweis in /usr/local/bin an, macOS fragt nach deinem Passwort. " +
      "Mit jedem Update der App ist auch der Befehl aktuell.",
    cancel: "Abbrechen",
    ok: "Installieren",
  },
  [CommandState.CommandOther]: {
    title: "Kommandozeilen-Tool ersetzen?",
    note: "In /usr/local/bin liegt schon ein Befehl lieferling, vermutlich aus einem Release-Download. " +
      "Ersetzt du ihn durch den aus dieser App, bleibt er mit jedem Update aktuell.",
    cancel: "Behalten",
    ok: "Ersetzen",
  },
  [CommandState.CommandLinked]: {
    title: "Kommandozeilen-Tool ist installiert",
    note: "Öffne ein neues Terminalfenster und tippe lieferling list. " +
      "Der Befehl kommt aus dieser App und ist mit jedem Update aktuell.",
    done: "Fertig",
  },
  [CommandState.CommandUnavailable]: {
    title: "Kein Kommandozeilen-Tool dabei",
    note: "Diese Ausgabe von Lieferling bringt keine Kommandozeile mit, etwa weil sie direkt aus dem Quellcode läuft. " +
      "Die Releases auf GitHub haben sie.",
    done: "Schließen",
  },
};

// withCode sets text and puts the command and the folder in code.
function withCode(el: HTMLElement, text: string) {
  const parts = text.split(/(lieferling list|lieferling(?=[ ,])|\/usr\/local\/bin)/);
  el.replaceChildren(...parts.map((part, i) => {
    if (i % 2 === 0) return part;
    const code = document.createElement("code");
    code.textContent = part;
    return code;
  }));
}

// setPath writes a path that breaks after a slash rather than inside a name.
function setPath(el: HTMLElement, path: string) {
  el.replaceChildren(...path.split("/").flatMap((part, i) =>
    i === 0 ? [part] : [document.createElement("wbr"), "/" + part]));
}

function showCommand(c: Command) {
  const copy = commandCopy[c.state] ?? commandCopy[CommandState.CommandUnavailable];
  commandDialog.dataset.state = c.state;
  $("#command-title", commandDialog).textContent = copy.title;
  withCode($("#command-note", commandDialog), copy.note);
  const to = c.state === CommandState.CommandOther ? c.other || "eine eigene Datei, kein Verweis" : c.bundled;
  setPath($(".from", commandDialog), c.path);
  setPath($(".to", commandDialog), to);
  $(".manual", commandDialog).textContent = c.manual;
  if (c.state === CommandState.CommandLinked || !c.manual) commandManual.hidden = true;
  for (const b of commandDialog.querySelectorAll<HTMLButtonElement>("[data-show]")) {
    b.hidden = !b.dataset.show!.split(" ").includes(c.state);
  }
  const [cancel, done] = commandDialog.querySelectorAll<HTMLButtonElement>('button[value="cancel"]');
  cancel.textContent = copy.cancel ?? "";
  done.textContent = copy.done ?? "";
  commandInstall.textContent = copy.ok ?? "";
}

// the dialog's main button, as on macOS
function focusCommandDefault() {
  [...commandDialog.querySelectorAll<HTMLButtonElement>("[data-show]:not([hidden])")].at(-1)?.focus();
}

async function openCommand() {
  commandError.textContent = "";
  commandManual.hidden = true;
  try {
    showCommand(await AppService.CommandLine());
  } catch (err) {
    commandError.textContent = errorText(err);
  }
  if (!commandDialog.open) commandDialog.showModal();
  focusCommandDefault();
}

const commandWait = $<HTMLParagraphElement>(".command-wait", commandDialog);
const commandManual = $<HTMLDivElement>(".command-manual", commandDialog);
let commandCall: {cancel(): void} | undefined;

// the work happens with the dialog open: macOS asks for the password on top.
// Abbrechen stays live and stops the wait, so the window never hangs on a
// password dialog that did not show up.
async function runCommand(button: HTMLButtonElement, busyText: string, call: () => Promise<Command | null> & {cancel(): void}) {
  const label = button.textContent;
  commandError.textContent = "";
  commandManual.hidden = true;
  for (const b of commandDialog.querySelectorAll<HTMLButtonElement>("button:not([value=cancel])")) b.disabled = true;
  button.setAttribute("aria-busy", "true");
  button.textContent = busyText;
  commandWait.hidden = false;
  const p = call();
  commandCall = p;
  try {
    const c = await p;
    if (c) showCommand(c);
  } catch (err) {
    button.textContent = label;
    if (commandCall !== p) return; // canceled
    commandError.textContent = errorText(err);
    commandManual.hidden = !$(".manual", commandDialog).textContent || button !== commandInstall;
  } finally {
    if (commandCall === p) commandCall = undefined;
    commandWait.hidden = true;
    button.removeAttribute("aria-busy");
    button.textContent = button.textContent === busyText ? label : button.textContent;
    for (const b of commandDialog.querySelectorAll("button")) b.disabled = false;
    if (commandDialog.open) focusCommandDefault();
  }
}

function cancelCommand() {
  const p = commandCall;
  commandCall = undefined;
  p?.cancel();
}

commandInstall.addEventListener("click", (e) => {
  e.preventDefault();
  runCommand(commandInstall, "Wird installiert …", () => AppService.InstallCommandLine());
});
commandRemove.addEventListener("click", (e) => {
  e.preventDefault();
  runCommand(commandRemove, "Wird entfernt …", () => AppService.RemoveCommandLine());
});
commandDialog.addEventListener("close", cancelCommand);
$(".copy-manual", commandDialog).addEventListener("click", async (e) => {
  const b = e.currentTarget as HTMLButtonElement;
  await Clipboard.SetText($(".manual", commandDialog).textContent ?? "");
  b.textContent = "Kopiert";
  setTimeout(() => (b.textContent = "Kopieren"), 1800);
});

$("#open-command").addEventListener("click", openCommand);
Events.On("command:open", openCommand);

AppService.ConfigPath().then((p) => ($("#config-path").textContent = p));
load().then(takeLink);
