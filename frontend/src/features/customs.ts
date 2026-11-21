import {type Check, type Report, type Sender, type Status} from "../backend";
import {ask} from "../lib/dialog";
import {$, clone} from "../lib/dom";
import {install, onInstalled} from "./install";
import {rowAction, rowSection} from "./row";

// The customs note under a parcel ("Zollvermerk"): what was checked before
// the package was let in, and why one was stopped. Collapsed it is one line;
// a held package, a sender change waiting for an answer and a dispute
// between Macs are always open.

const checkTemplate = $<HTMLTemplateElement>("#customs-check");
const senderDialog = $<HTMLDialogElement>("#confirm-sender");

const resultIcon = {pass: "#i-check", fail: "#i-alert", skip: "#i-dash"} as Record<string, string>;
const resultLabel = {pass: "Stimmt: ", fail: "Weicht ab: ", skip: "Nicht prüfbar: "} as Record<string, string>;

function stopped(r: Report | null | undefined) {
  return r?.verdict === "held" || r?.verdict === "ask" || r?.verdict === "disputed";
}

export function setupCustoms() {
  rowSection("customs", {setup, fill});
  // the row's main button while a sender change waits for an answer
  rowAction("review", {run: (app) => void review(app)});
  onInstalled((app, o) => {
    if (app.customs?.verdict === "ask" && !o.quiet) void review(app);
  });
}

function setup(el: HTMLElement) {
  const details = el as HTMLDetailsElement;
  // Enter and Space on a summary dispatch click too, so this covers the keyboard
  $("summary", details).addEventListener("click", (e) => {
    if (stopped(reportOf(details))) e.preventDefault();
  });
}

const reports = new WeakMap<HTMLElement, Report>();
const reportOf = (el: HTMLElement) => reports.get(el);

function fill(el: HTMLElement, app: Status, row: HTMLLIElement) {
  const details = el as HTMLDetailsElement;
  const r = app.customs;
  details.hidden = !r;
  if (!r) {
    reports.delete(details);
    return;
  }
  const was = stopped(reportOf(details));
  reports.set(details, r);
  details.dataset.verdict = r.verdict;
  $(".customs-summary", details).textContent = r.summary;

  const list = $<HTMLUListElement>(".customs-checks", details);
  list.replaceChildren(...(r.checks ?? []).map(checkLine));
  // a stopped note shows the alert where the chevron would be
  $(".chevron use", details).setAttribute("href", stopped(r) ? "#i-alert" : "#i-chevron");
  relabelAction(row, r);

  const summary = $<HTMLElement>("summary", details);
  if (stopped(r)) {
    details.open = true;
    summary.setAttribute("aria-disabled", "true");
    summary.tabIndex = -1; // nothing to toggle, so no tab stop
  } else {
    summary.removeAttribute("aria-disabled");
    summary.tabIndex = 0;
    // leaving a stopped verdict folds the note; otherwise a refresh keeps
    // whatever the user opened
    if (was) details.open = false;
  }
}

// relabelAction makes the row's main button fit the note: while a sender
// change waits, it opens the question; after a hold it loads again rather
// than promising an install. A row that offers "Öffnen" keeps it.
function relabelAction(row: HTMLLIElement, r: Report) {
  const action = $<HTMLButtonElement>(".action", row);
  if (action.dataset.mode !== "install") return;
  if (r.verdict === "ask") {
    action.textContent = "Absender ansehen …";
    action.className = "button action";
    action.dataset.mode = "review";
  } else if (r.verdict === "held") {
    action.textContent = "Erneut laden";
  } else {
    return;
  }
  const name = $(".name", row).textContent;
  action.setAttribute("aria-label", `${action.textContent}: ${name}`);
}

function checkLine(c: Check): HTMLLIElement {
  const li = clone<HTMLLIElement>(checkTemplate);
  li.dataset.result = c.result;
  $("use", li).setAttribute("href", resultIcon[c.result] ?? "#i-dash");
  $(".visually-hidden", li).textContent = resultLabel[c.result] ?? "";
  $(".customs-text", li).append(c.text);
  return li;
}

function showSender(dd: HTMLElement, s: Sender | null | undefined, other: Sender | null | undefined) {
  const bundle = $<HTMLElement>(".bundle", dd);
  const team = $<HTMLElement>(".team", dd);
  bundle.textContent = s?.bundleId || "ohne Bundle-ID";
  team.replaceChildren(s?.teamId ? "Team-ID " : "ohne Team-ID");
  if (s?.teamId) {
    const code = document.createElement("code");
    code.textContent = s.teamId;
    team.append(code);
  }
  // only what differs is marked, and only on the new side
  const after = dd.dataset.sender === "after";
  bundle.toggleAttribute("data-changed", after && (s?.bundleId ?? "").toLowerCase() !== (other?.bundleId ?? "").toLowerCase());
  team.toggleAttribute("data-changed", after && (s?.teamId ?? "") !== (other?.teamId ?? ""));
}

async function review(app: Status) {
  const r = app.customs;
  if (r?.verdict !== "ask") return;
  $("#sender-title", senderDialog).textContent = `${app.name} ${r.version} kommt von einem anderen Absender`;
  showSender($('[data-sender="before"]', senderDialog), r.before, r.after);
  showSender($('[data-sender="after"]', senderDialog), r.after, r.before);
  if (await ask(senderDialog) !== "accept") return;
  await install(app.repo, {accept: true});
}
