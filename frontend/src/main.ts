import {Events} from "@wailsio/runtime";
import {AppService} from "../bindings/github.com/pklaschka/elappma";
import type {Status} from "../bindings/github.com/pklaschka/elappma/internal/store/models";

const $ = <T extends Element>(sel: string, root: ParentNode = document) => root.querySelector(sel) as T;

const list = $<HTMLUListElement>("#apps");
const empty = $<HTMLDivElement>("#empty");
const summary = $<HTMLParagraphElement>("#summary");
const refreshButton = $<HTMLButtonElement>("#refresh");
const updateAllButton = $<HTMLButtonElement>("#update-all");
const rowTemplate = $<HTMLTemplateElement>("#row");
const addForm = $<HTMLFormElement>("#add");
const repoInput = $<HTMLInputElement>("#repo");
const addError = $<HTMLParagraphElement>("#add-error");
const removeDialog = $<HTMLDialogElement>("#confirm-remove");

let apps: Status[] = [];
const busy = new Set<string>(); // repos being installed right now

function errorText(err: unknown): string {
  const msg = err instanceof Error ? err.message : String(err);
  return msg.replace(/^[a-z]+:\s*/i, "") || "Unbekannter Fehler";
}

function plural(n: number, one: string, many: string) {
  return `${n} ${n === 1 ? one : many}`;
}

// ---- list -----------------------------------------------------------------

async function load() {
  refreshButton.setAttribute("aria-busy", "true");
  refreshButton.disabled = true;
  try {
    render((await AppService.Apps()) ?? []);
  } catch (err) {
    summary.textContent = `Liste nicht lesbar: ${errorText(err)}`;
  } finally {
    refreshButton.removeAttribute("aria-busy");
    refreshButton.disabled = false;
    list.setAttribute("aria-busy", "false");
  }
}

function render(next: Status[]) {
  apps = next;
  empty.hidden = apps.length > 0;
  list.hidden = apps.length === 0;

  const updates = apps.filter((a) => a.updateAvailable);
  summary.textContent = apps.length === 0 ? "" :
    updates.length === 0 ? `${plural(apps.length, "App", "Apps")}, alle aktuell` :
      `${plural(apps.length, "App", "Apps")}, ${plural(updates.length, "Update", "Updates")}`;
  updateAllButton.hidden = updates.length < 2;

  const rows = new Map<string, HTMLLIElement>();
  for (const li of list.querySelectorAll<HTMLLIElement>("li.app")) rows.set(li.dataset.repo!, li);
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

function createRow(repo: string): HTMLLIElement {
  const li = (rowTemplate.content.cloneNode(true) as DocumentFragment).firstElementChild as HTMLLIElement;
  li.dataset.repo = repo;
  const id = `menu-${++rowCount}`;
  const menu = $<HTMLDivElement>(".menu", li);
  const more = $<HTMLButtonElement>(".more", li);
  menu.id = id;
  more.setAttribute("popovertarget", id);
  more.setAttribute("aria-label", `Mehr zu ${repo}`);

  // place the menu under its button (anchor positioning is not in every WebKit yet)
  menu.addEventListener("beforetoggle", (e) => {
    if ((e as ToggleEvent).newState !== "open") return;
    const r = more.getBoundingClientRect();
    menu.style.top = `${r.bottom + 4}px`;
    menu.style.right = `${document.documentElement.clientWidth - r.right}px`;
  });
  menu.addEventListener("click", (e) => {
    const act = (e.target as HTMLElement).closest<HTMLButtonElement>("button")?.dataset.act;
    menu.hidePopover();
    const app = apps.find((a) => a.repo === li.dataset.repo);
    if (!app) return;
    if (act === "release" && app.releaseUrl) AppService.OpenURL(app.releaseUrl);
    if (act === "remove") confirmRemove(app);
  });

  $<HTMLButtonElement>(".action", li).addEventListener("click", () => install(li.dataset.repo!));
  $<HTMLInputElement>(".channel input", li).addEventListener("change", async (e) => {
    const box = e.target as HTMLInputElement;
    box.disabled = true;
    try {
      await AppService.SetChannel(li.dataset.repo!, box.checked);
    } catch (err) {
      box.checked = !box.checked;
      showRowError(li, errorText(err));
    } finally {
      box.disabled = false;
    }
  });
  return li;
}

function fillRow(li: HTMLLIElement, app: Status) {
  const state = !app.installed ? "missing" : app.updateAvailable ? "update" : "current";
  li.dataset.state = state;
  $(".name", li).textContent = app.name;
  $(".repo", li).textContent = app.repo;
  $(".installed", li).textContent = app.installed || "–";
  $(".latest", li).textContent = app.latest || "–";
  const versionLabel = state === "update" ? `Installiert ${app.installed}, neu ${app.latest}` :
    state === "current" ? `Installiert ${app.installed}` : app.latest ? `Neueste Version ${app.latest}` : "";
  $(".app-version", li).setAttribute("aria-label", versionLabel);

  const channel = $<HTMLInputElement>(".channel input", li);
  channel.checked = app.channel === "prerelease";
  channel.setAttribute("aria-label", `Vorabversionen für ${app.name}`);

  const action = $<HTMLButtonElement>(".action", li);
  action.className = "button action";
  action.disabled = false;
  if (state === "missing") {
    action.textContent = "Installieren";
  } else if (state === "update") {
    action.textContent = "Aktualisieren";
    action.classList.add("primary");
  } else {
    action.innerHTML = `<svg class="icon" aria-hidden="true"><use href="#i-check"/></svg> Aktuell`;
    action.disabled = true;
  }
  action.dataset.unavailable = String(Boolean(app.error && !app.latest));
  action.setAttribute("aria-label", `${action.textContent!.trim()}: ${app.name}`);

  const releaseItem = $<HTMLButtonElement>('[data-act="release"]', li);
  releaseItem.disabled = !app.releaseUrl;

  showRowError(li, app.error);
  setBusy(li, busy.has(app.repo));
}

function showRowError(li: HTMLLIElement, message: string) {
  const el = $<HTMLParagraphElement>(".app-error", li);
  el.hidden = !message;
  $("span", el).textContent = message;
}

function setBusy(li: HTMLLIElement, on: boolean) {
  const progress = $<HTMLProgressElement>("progress", li);
  const action = $<HTMLButtonElement>(".action", li);
  if (on) action.hidden = true;
  else action.hidden = action.dataset.unavailable === "true";
  progress.hidden = !on;
  if (on) {
    progress.setAttribute("aria-label", `${li.dataset.repo} wird geladen`);
  } else {
    progress.removeAttribute("value"); // back to indeterminate for next time
  }
}

// ---- actions --------------------------------------------------------------

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
  $<HTMLProgressElement>("progress", li).value = done / total;
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
      "Das sieht nicht nach einem GitHub-Repository aus. Erwartet wird owner/name oder die URL.";
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
  uninstall.closest("label")!.hidden = !app.managedByElappma;
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

AppService.ConfigPath().then((p) => ($("#config-path").textContent = p));
load();
