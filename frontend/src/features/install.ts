import {AppService, Events, type Status} from "../backend";
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
    for (const app of getApps().filter((a) => a.updateAvailable)) await install(app.repo, {quiet: true});
    updateAllButton.disabled = false;
  });
}

export interface InstallOptions {
  accept?: boolean; // the user accepted a changed sender: call AcceptSender
  quiet?: boolean; // part of "Alle aktualisieren": open no dialog
}

type Listener = (app: Status, o: InstallOptions) => void;
const listeners: Listener[] = [];

// onInstalled runs after every install that returned a status, also when
// customs held the package. It keeps the dependency one way: features that
// react to an install know this module, not the other way round.
export function onInstalled(fn: Listener) {
  listeners.push(fn);
}

export async function install(repo: string, o: InstallOptions = {}): Promise<Status | undefined> {
  const li = findRow(repo);
  if (!li || busy.has(repo)) return;
  busy.add(repo);
  showRowError(li, "");
  setBusy(li, true);
  let st: Status | undefined;
  try {
    st = await (o.accept ? AppService.AcceptSender(repo) : AppService.Install(repo));
  } catch (err) {
    showRowError(li, errorText(err));
  } finally {
    busy.delete(repo);
    setBusy(li, false);
  }
  if (st) for (const fn of listeners) fn(st, o);
  return st;
}
