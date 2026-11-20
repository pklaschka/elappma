import {AppService, type Status} from "../backend";
import {$, clone} from "../lib/dom";
import {placeUnder} from "../lib/menu";
import {errorText} from "../lib/text";
import {busy, getApps} from "../state";

// One parcel per app. The markup is the template #row in index.html; features
// add behaviour by name, so the row knows no feature and each feature is one
// file:
//
//   rowAction("name", …)   a menu entry <button data-act="name">, and the
//                          row's main button, whose data-mode is "open" or
//                          "install"
//   rowSection("name", …)  a line under the parcel, a direct child of
//                          li.parcel with data-section="name"
//
// Register before the first row is built, that is in a setup function that
// main.ts calls before loading the list.

export interface RowAction {
  run(app: Status, row: HTMLLIElement): void;
  // on every fill: disable, tick or rename the menu entry
  update?(item: HTMLButtonElement, app: Status): void;
}

export interface RowSection {
  setup?(el: HTMLElement, row: HTMLLIElement): void; // once per row
  fill(el: HTMLElement, app: Status, row: HTMLLIElement): void; // on every fill
}

const actions = new Map<string, RowAction>();
const sections = new Map<string, RowSection>();

export function rowAction(name: string, action: RowAction) {
  actions.set(name, action);
}

export function rowSection(name: string, section: RowSection) {
  sections.set(name, section);
}

const list = $<HTMLUListElement>("#apps");
const template = $<HTMLTemplateElement>("#row");
let rowCount = 0;

export function findRow(repo: string): HTMLLIElement | null {
  return list.querySelector<HTMLLIElement>(`li[data-repo="${CSS.escape(repo)}"]`);
}

function appOf(row: HTMLLIElement): Status | undefined {
  return getApps().find((a) => a.repo === row.dataset.repo);
}

function run(name: string | undefined, row: HTMLLIElement) {
  const action = name ? actions.get(name) : undefined;
  const app = appOf(row);
  if (action && app) action.run(app, row);
}

function eachSection(row: HTMLLIElement, fn: (section: RowSection, el: HTMLElement) => void) {
  for (const el of row.querySelectorAll<HTMLElement>(":scope > [data-section]")) {
    const section = sections.get(el.dataset.section!);
    if (section) fn(section, el);
  }
}

export function createRow(repo: string): HTMLLIElement {
  const li = clone<HTMLLIElement>(template);
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
    run(act, li);
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
    run((e.currentTarget as HTMLElement).dataset.mode, li);
  });

  eachSection(li, (section, el) => section.setup?.(el, li));
  return li;
}

export function fillRow(li: HTMLLIElement, app: Status) {
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

  for (const item of li.querySelectorAll<HTMLButtonElement>(".menu [data-act]")) {
    actions.get(item.dataset.act!)?.update?.(item, app);
  }
  eachSection(li, (section, el) => section.fill(el, app, li));
  setBusy(li, busy.has(app.repo));
}

// the problem line: what went wrong with this app, if anything
rowSection("problem", {fill: (_, app, row) => showRowError(row, app.error)});

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

export function showRowError(li: HTMLLIElement, message: string) {
  const el = $<HTMLParagraphElement>(".problem", li);
  el.hidden = !message;
  $("span", el).textContent = message;
}

export function setBusy(li: HTMLLIElement, on: boolean) {
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

// flash shows a short note in the row's status slot.
export function flash(li: HTMLLIElement, text: string) {
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
