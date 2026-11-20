import type {Status} from "./backend";

// The list as the window knows it. Whoever brings a new list calls setApps;
// whoever draws from it listens with onApps.
let apps: Status[] = [];
const listeners: ((apps: Status[]) => void)[] = [];

export function getApps(): Status[] {
  return apps;
}

// findApp compares without case: the list never holds two repositories that
// differ only in case, and links may be written either way.
export function findApp(repo: string): Status | undefined {
  const wanted = repo.toLowerCase();
  return apps.find((a) => a.repo.toLowerCase() === wanted);
}

export function setApps(next: Status[]) {
  apps = next;
  for (const fn of listeners) fn(apps);
}

export function onApps(fn: (apps: Status[]) => void) {
  listeners.push(fn);
}

// repos being installed right now
export const busy = new Set<string>();
