import {AppService, Events, type ImportItem, type ImportPlan} from "../backend";
import {ask} from "../lib/dialog";
import {$} from "../lib/dom";
import {placeUnder} from "../lib/menu";
import {fillPickList, picked, type PickItem} from "../lib/picklist";
import {errorText, plural} from "../lib/text";
import {getApps} from "../state";
import {note} from "./list";

// Catalogs: the »Katalog« menu, the Ablage menu (catalog:menu), and the
// buttons of the empty list. Importing shows the catalog as a packing list
// first; nothing is installed.

const catalogMenu = $<HTMLDivElement>("#catalog-menu");
const catalogButton = $<HTMLButtonElement>("#catalog-button");
const empty = $<HTMLDivElement>("#empty");
const urlDialog = $<HTMLDialogElement>("#import-url");
const urlForm = $<HTMLFormElement>("#url-form");
const urlInput = $<HTMLInputElement>("#catalog-url");
const urlError = $<HTMLParagraphElement>("#url-error");
const importDialog = $<HTMLDialogElement>("#import");
const manifest = $<HTMLUListElement>(".manifest", importDialog);

let catalogBusy = false;

export function setupCatalog() {
  placeUnder(catalogMenu, catalogButton);
  catalogMenu.addEventListener("click", (e) => {
    const act = (e.target as HTMLElement).closest<HTMLButtonElement>("button")?.dataset.catalog;
    catalogMenu.hidePopover();
    catalogAction(act);
  });
  for (const b of empty.querySelectorAll<HTMLButtonElement>("[data-catalog]")) {
    b.addEventListener("click", () => catalogAction(b.dataset.catalog));
  }
  Events.On("catalog:menu", (ev) => catalogAction(ev.data));

  urlDialog.querySelector("[data-close]")!.addEventListener("click", () => urlDialog.close());
  urlForm.addEventListener("submit", loadFromURL);
  urlInput.addEventListener("input", () => {
    if (urlInput.getAttribute("aria-invalid") === "true" && urlInput.checkValidity()) setURLError("");
  });
}

function catalogAction(act: string | undefined) {
  if (catalogBusy || importDialog.open || urlDialog.open) return;
  if (act === "import-file") chooseCatalog();
  if (act === "import-url") askForURL();
  if (act === "export") exportCatalog();
}

async function exportCatalog() {
  catalogBusy = true;
  try {
    const path = await AppService.ExportCatalog();
    if (path) note(`${plural(getApps().length, "App", "Apps")} als Katalog gesichert: ${path.split("/").pop()}`);
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
    if (plan) importPlan(plan);
  } catch (err) {
    note(`Katalog nicht lesbar: ${errorText(err)}`);
  } finally {
    catalogBusy = false;
  }
}

// ---- from an address ---------------------------------------------------------

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

async function loadFromURL(e: SubmitEvent) {
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
    if (plan) importPlan(plan);
  } catch (err) {
    setURLError(errorText(err));
    urlInput.focus();
  } finally {
    submit.disabled = false;
    submit.textContent = "Laden";
  }
}

// ---- the packing list ---------------------------------------------------------

async function importPlan(plan: ImportPlan) {
  const items = plan.items ?? [];
  const fresh = items.filter((it) => !it.onList && !it.error);
  const known = items.filter((it) => it.onList);
  const bad = items.filter((it) => it.error);

  const catalogName = plan.name ? `»${plan.name}«` : "";
  $("#import-title", importDialog).textContent = fresh.length === 0 ?
    `Nichts Neues im Katalog ${catalogName}`.trim() :
    `${plural(fresh.length, "App", "Apps")} aus dem Katalog ${catalogName} übernehmen?`.replace("  ", " ");
  $(".import-source .path", importDialog).textContent = plan.source;

  fillPickList(manifest, [...fresh, ...known, ...bad].map(pickItem));

  const parts: string[] = [];
  if (known.length) parts.push(`${plural(known.length, "App steht", "Apps stehen")} schon auf der Liste und ${known.length === 1 ? "bleibt" : "bleiben"}, wie ${known.length === 1 ? "sie ist" : "sie sind"}.`);
  if (bad.length) parts.push(`${plural(bad.length, "Eintrag ist", "Einträge sind")} nicht lesbar.`);
  if (fresh.length) parts.push("Füge nur Repositories hinzu, deren Apps du vertraust. Installiert wird erst, wenn du es in der Liste sagst.");
  $("#import-note", importDialog).textContent = parts.join(" ");

  const ok = $<HTMLButtonElement>('button[value="ok"]', importDialog);
  const cancel = $<HTMLButtonElement>('button[value="cancel"]', importDialog);
  ok.hidden = fresh.length === 0;
  cancel.textContent = fresh.length === 0 ? "Schließen" : "Abbrechen";
  const chosen = () => {
    const repos = new Set(picked(manifest));
    return fresh.filter((it) => repos.has(it.repo));
  };
  const updateButton = () => {
    const n = chosen().length;
    ok.textContent = n === 0 ? "Hinzufügen" : `${plural(n, "App", "Apps")} hinzufügen`;
    ok.disabled = n === 0;
  };
  updateButton();
  manifest.onchange = updateButton;
  const answer = ask(importDialog);
  manifest.scrollTop = 0;
  if (await answer !== "ok") return;

  const take = chosen();
  if (take.length === 0) return;
  try {
    const added = await AppService.ImportCatalog(take);
    note(added === 0 ? "Keine neuen Apps hinzugefügt" : `${plural(added, "App", "Apps")} aus dem Katalog hinzugefügt`);
  } catch (err) {
    note(`Nicht übernommen: ${errorText(err)}`);
  }
}

function pickItem(it: ImportItem): PickItem {
  const tag = it.channel === "prerelease" && !it.error;
  if (it.error) return {value: it.repo, state: "invalid", tag, note: it.error, disabled: true};
  if (it.onList) return {value: it.repo, state: "known", tag, note: "Schon auf der Liste", checked: true, disabled: true};
  return {value: it.repo, state: "new", tag, checked: true};
}
