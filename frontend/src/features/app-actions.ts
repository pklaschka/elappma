import {AppService, type Status} from "../backend";
import {$} from "../lib/dom";
import {errorText} from "../lib/text";
import {flash, rowAction, showRowError} from "./row";

// The plain entries of a row's menu: open the app, follow pre-releases, open
// the release page.
export function setupAppActions() {
  rowAction("open", {
    run: open,
    update: (item, app) => (item.disabled = !app.installed || app.self),
  });
  rowAction("channel", {
    run: (app, row) => setChannel(row, app.channel !== "prerelease"),
    update: (item, app) => item.setAttribute("aria-checked", String(app.channel === "prerelease")),
  });
  rowAction("release", {
    run: (app) => {
      if (app.releaseUrl) AppService.OpenURL(app.releaseUrl);
    },
    update: (item, app) => {
      item.disabled = !app.releaseUrl;
      item.textContent = `Release auf ${app.forge || "GitHub"} öffnen`;
    },
  });
}

async function open(app: Status, li: HTMLLIElement) {
  showRowError(li, "");
  try {
    await AppService.Open(app.repo);
    flash(li, "Wird geöffnet …");
  } catch (err) {
    showRowError(li, errorText(err));
  }
}

async function setChannel(li: HTMLLIElement, prerelease: boolean) {
  const item = $<HTMLButtonElement>('[data-act="channel"]', li);
  item.setAttribute("aria-checked", String(prerelease));
  try {
    await AppService.SetChannel(li.dataset.repo!, prerelease);
  } catch (err) {
    item.setAttribute("aria-checked", String(!prerelease));
    showRowError(li, errorText(err));
  }
}
