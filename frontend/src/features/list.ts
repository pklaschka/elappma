import {AppService, Events, type Status} from "../backend";
import {$, clone} from "../lib/dom";
import {errorText, plural} from "../lib/text";
import {getApps, onApps, setApps} from "../state";
import {createRow, fillRow} from "./row";

const list = $<HTMLUListElement>("#apps");
const loading = $<HTMLUListElement>("#loading");
const ghostTemplate = $<HTMLTemplateElement>("#ghost");
const empty = $<HTMLDivElement>("#empty");
const summary = $<HTMLParagraphElement>("#summary");
const refreshButton = $<HTMLButtonElement>("#refresh");
const updateAllButton = $<HTMLButtonElement>("#update-all");

// The first load asks the forges about every app and can take a moment. Until
// they answer, placeholder parcels stand where the rows will be, as many as last
// time, and they stay at least this long so the window does not flicker.
const MIN_LOADING_MS = 700;
const COUNT_KEY = "lieferling:count";
let loaded = false;
let noting = false; // the bar shows a passing note instead of the summary

export function setupList() {
  onApps(draw);
  Events.On("apps:changed", (ev) => setApps(ev.data ?? []));
  refreshButton.addEventListener("click", load);
}

function showLoading() {
  let count = 3;
  try {
    const saved = Number(localStorage.getItem(COUNT_KEY));
    if (Number.isFinite(saved) && localStorage.getItem(COUNT_KEY) !== null) count = saved;
  } catch {
    // no storage: keep the default
  }
  count = Math.min(Math.max(count, 1), 6);
  const ghosts = Array.from({length: count}, (_, i) => {
    const li = clone<HTMLLIElement>(ghostTemplate);
    li.style.setProperty("--i", String(i));
    return li;
  });
  loading.replaceChildren(...ghosts);
  loading.hidden = false;
  summary.textContent = "Sendungen werden abgerufen …";
}

export async function load() {
  refreshButton.setAttribute("aria-busy", "true");
  refreshButton.disabled = true;
  const first = !loaded;
  if (first) showLoading();
  const shown = performance.now();
  try {
    const next = (await AppService.Apps()) ?? [];
    if (first) {
      await new Promise((r) => setTimeout(r, MIN_LOADING_MS - (performance.now() - shown)));
      list.dataset.arrive = "";
      setTimeout(() => delete list.dataset.arrive, 1200);
    }
    loaded = true;
    loading.hidden = true;
    setApps(next);
    try {
      localStorage.setItem(COUNT_KEY, String(next.length));
    } catch {
      // only a convenience
    }
  } catch (err) {
    loading.hidden = true;
    summary.textContent = `Liste nicht lesbar: ${errorText(err)}`;
  } finally {
    refreshButton.removeAttribute("aria-busy");
    refreshButton.disabled = false;
    list.setAttribute("aria-busy", "false");
  }
}

function draw(apps: Status[]) {
  if (!loaded) return; // the placeholders stay until the first load is done
  empty.hidden = apps.length > 0;
  list.hidden = apps.length === 0;

  const updates = apps.filter((a) => a.updateAvailable);
  if (!noting) summary.textContent = apps.length === 0 ? "" :
    updates.length === 0 ? `${plural(apps.length, "App", "Apps")}, alle aktuell` :
      `${plural(apps.length, "App", "Apps")}, ${plural(updates.length, "Update", "Updates")}`;
  updateAllButton.hidden = updates.length < 2;

  const rows = new Map<string, HTMLLIElement>();
  for (const li of list.querySelectorAll<HTMLLIElement>("li.parcel")) rows.set(li.dataset.repo!, li);
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

// note shows a passing message in the bar, then the usual summary again.
let noteTimer: number | undefined;
export function note(text: string) {
  clearTimeout(noteTimer);
  noting = true;
  summary.dataset.note = "";
  summary.textContent = text;
  noteTimer = setTimeout(() => {
    noting = false;
    delete summary.dataset.note;
    draw(getApps());
  }, 5000);
}
