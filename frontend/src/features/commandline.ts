import {AppService, Clipboard, type Command, CommandState, Events} from "../backend";
import {setDialogState} from "../lib/dialog";
import {$} from "../lib/dom";
import {errorText} from "../lib/text";

// The dialog »Kommandozeile«: links the command line inside the app bundle to
// /usr/local/bin, or removes that link. Its states are the CommandState
// values; the buttons say in data-show in which states they appear.

const commandDialog = $<HTMLDialogElement>("#command");
const commandError = $<HTMLParagraphElement>(".field-error", commandDialog);
const commandInstall = $<HTMLButtonElement>('button[value="install"]', commandDialog);
const commandRemove = $<HTMLButtonElement>('button[value="remove"]', commandDialog);
const commandWait = $<HTMLParagraphElement>(".command-wait", commandDialog);
const commandManual = $<HTMLDivElement>(".command-manual", commandDialog);

export function setupCommandLine() {
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
}

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
  setDialogState(commandDialog, c.state);
  $("#command-title", commandDialog).textContent = copy.title;
  withCode($("#command-note", commandDialog), copy.note);
  const to = c.state === CommandState.CommandOther ? c.other || "eine eigene Datei, kein Verweis" : c.bundled;
  setPath($(".from", commandDialog), c.path);
  setPath($(".to", commandDialog), to);
  $(".manual", commandDialog).textContent = c.manual;
  if (c.state === CommandState.CommandLinked || !c.manual) commandManual.hidden = true;
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
