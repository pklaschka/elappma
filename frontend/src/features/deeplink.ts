import {AppService, Clipboard, Events, type Status} from "../backend";
import {ask} from "../lib/dialog";
import {$} from "../lib/dom";
import {errorText} from "../lib/text";
import {findApp, getApps, setApps} from "../state";
import {showAddError} from "./add";
import {install} from "./install";
import {load} from "./list";
import {findRow, flash, rowAction, showRowError} from "./row";

// Links of the form lieferling://install/owner/name: a row copies one for its
// README, and opening one asks before adding or installing anything, because a
// link comes from a web page.

const linkDialog = $<HTMLDialogElement>("#confirm-link");

export function setupDeeplink() {
  rowAction("link", {run: copyLink});
  Events.On("link:opened", takeLink);
}

async function copyLink(app: Status, li: HTMLLIElement) {
  try {
    const url = await AppService.LinkFor(app.repo, true);
    await Clipboard.SetText(`[In Lieferling installieren](${url})`);
    flash(li, "Link kopiert");
  } catch (err) {
    showRowError(li, errorText(err));
  }
}

export async function takeLink() {
  const link = await AppService.TakeLink();
  if (!link) return;
  if (getApps().length === 0) await load();
  const known = findApp(link.repo);
  const row = known && findRow(known.repo);
  // already there and nothing to do: show it
  if (known && (!link.install || known.installed && !known.updateAvailable)) {
    row?.scrollIntoView({block: "nearest"});
    row && flash(row, "Schon auf der Liste");
    return;
  }
  const wantInstall = $<HTMLInputElement>('input[name="install"]', linkDialog);
  const prerelease = $<HTMLInputElement>('input[name="prerelease"]', linkDialog);
  wantInstall.checked = link.install;
  prerelease.checked = link.prerelease;
  $<HTMLElement>('[data-field="install"]', linkDialog).hidden = !!known;
  $<HTMLElement>('[data-field="prerelease"]', linkDialog).hidden = !!known;
  $("#link-title", linkDialog).textContent = known ?
    `${known.name} installieren?` : `${link.repo} zu Lieferling hinzufügen?`;
  $('button[value="ok"]', linkDialog).textContent = known ? "Installieren" : "Hinzufügen";
  if (await ask(linkDialog) !== "ok") return;

  const onList = !!findApp(link.repo);
  const wanted = onList || wantInstall.checked;
  try {
    if (!onList) {
      const added = await AppService.Add(link.repo, prerelease.checked);
      setApps([...getApps().filter((a) => a.repo !== added!.repo), added!]);
    }
    const app = findApp(link.repo);
    if (wanted && app) await install(app.repo);
  } catch (err) {
    showAddError(errorText(err));
  }
}
