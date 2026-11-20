import {AppService, type Status} from "../backend";
import {ask} from "../lib/dialog";
import {$} from "../lib/dom";
import {errorText} from "../lib/text";
import {findRow, rowAction, showRowError} from "./row";

const removeDialog = $<HTMLDialogElement>("#confirm-remove");

export function setupRemove() {
  rowAction("remove", {run: confirmRemove});
}

async function confirmRemove(app: Status) {
  $("#remove-title", removeDialog).textContent = `${app.name} von der Liste entfernen?`;
  const uninstall = $<HTMLInputElement>('input[name="uninstall"]', removeDialog);
  uninstall.checked = false;
  uninstall.closest("label")!.hidden = !app.managedByLieferling;
  if (await ask(removeDialog) !== "remove") return;
  try {
    await AppService.Remove(app.repo, uninstall.checked);
  } catch (err) {
    const li = findRow(app.repo);
    if (li) showRowError(li, errorText(err));
  }
}
