import {type Check, type Report, type Sender, type Status} from "../backend";
import {ask} from "../lib/dialog";
import {$, clone} from "../lib/dom";
import {findApp} from "../state";
import {install, onInstalled} from "./install";
import {rowSection} from "./row";

// The customs note under a parcel ("Zollvermerk"): what was checked before
// the package was let in, and why one was stopped. Collapsed it is one line;
// a held package, a sender change waiting for an answer and a dispute
// between Macs are always open.

const checkTemplate = $<HTMLTemplateElement>("#customs-check");
const senderDialog = $<HTMLDialogElement>("#confirm-sender");

const resultIcon = {pass: "#i-check", fail: "#i-alert", skip: "#i-dash"} as Record<string, string>;
const resultLabel = {pass: "Bestanden: ", fail: "Nicht bestanden: ", skip: "Nicht prüfbar: "} as Record<string, string>;

function stopped(r: Report | null | undefined) {
  return r?.verdict === "held" || r?.verdict === "ask" || r?.verdict === "disputed";
}

export function setupCustoms() {
  rowSection("customs", {setup, fill});
  onInstalled((app, o) => {
    if (app.customs?.verdict === "ask" && !o.quiet) void review(app);
  });
}

function setup(el: HTMLElement, row: HTMLLIElement) {
  const details = el as HTMLDetailsElement;
  // Enter and Space on a summary dispatch click too, so this covers the keyboard
  $("summary", details).addEventListener("click", (e) => {
    if (stopped(reportOf(details))) e.preventDefault();
  });
  $<HTMLButtonElement>('[data-customs="review"]', details).addEventListener("click", () => {
    const app = findApp(row.dataset.repo!);
    if (app) void review(app);
  });
}

const reports = new WeakMap<HTMLElement, Report>();
const reportOf = (el: HTMLElement) => reports.get(el);

function fill(el: HTMLElement, app: Status) {
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
  $<HTMLElement>(".customs-actions", details).hidden = r.verdict !== "ask";

  const summary = $("summary", details);
  if (stopped(r)) {
    details.open = true;
    summary.setAttribute("aria-disabled", "true");
  } else {
    summary.removeAttribute("aria-disabled");
    // leaving a stopped verdict folds the note; otherwise a refresh keeps
    // whatever the user opened
    if (was) details.open = false;
  }
}

function checkLine(c: Check): HTMLLIElement {
  const li = clone<HTMLLIElement>(checkTemplate);
  li.dataset.result = c.result;
  $("use", li).setAttribute("href", resultIcon[c.result] ?? "#i-dash");
  $(".visually-hidden", li).textContent = resultLabel[c.result] ?? "";
  $(".customs-text", li).append(c.text);
  return li;
}

function showSender(dd: HTMLElement, s: Sender | null | undefined) {
  $(".bundle", dd).textContent = s?.bundleId || "ohne Bundle-ID";
  $(".team", dd).textContent = s?.teamId ? `Team-ID ${s.teamId}` : "ohne Team-ID";
}

async function review(app: Status) {
  const r = app.customs;
  if (r?.verdict !== "ask") return;
  $("#sender-title", senderDialog).textContent = `${app.name} ${r.version} kommt von einem anderen Absender`;
  showSender($('[data-sender="before"]', senderDialog), r.before);
  showSender($('[data-sender="after"]', senderDialog), r.after);
  if (await ask(senderDialog) !== "accept") return;
  await install(app.repo, {accept: true});
}
