import {AppService, Events} from "../backend";
import {$} from "../lib/dom";
import {errorText} from "../lib/text";
import {busy, getApps} from "../state";
import {findRow, rowAction, setBusy, showRowError} from "./row";

const updateAllButton = $<HTMLButtonElement>("#update-all");

export function setupInstall() {
  rowAction("install", {run: (app) => install(app.repo)});

  Events.On("install:progress", (ev) => {
    const {repo, done, total} = ev.data;
    const li = findRow(repo);
    if (!li || total <= 0) return;
    $<HTMLOListElement>(".route", li).style.setProperty("--progress", String(done / total));
    li.dataset.busy = done >= total ? "install" : "download";
    $(".status", li).textContent = done >= total ? "Wird installiert …" : `Wird geladen … ${Math.round((done / total) * 100)} %`;
  });

  updateAllButton.addEventListener("click", async () => {
    updateAllButton.disabled = true;
    for (const app of getApps().filter((a) => a.updateAvailable)) await install(app.repo);
    updateAllButton.disabled = false;
  });
}

export async function install(repo: string) {
  const li = findRow(repo);
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
