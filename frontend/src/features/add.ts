import {AppService} from "../backend";
import {$} from "../lib/dom";
import {errorText} from "../lib/text";

const addForm = $<HTMLFormElement>("#add");
const repoInput = $<HTMLInputElement>("#repo");
const addError = $<HTMLParagraphElement>("#add-error");

// The strip above the list: add a repository by name or URL.
export function setupAdd() {
  addForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (!repoInput.checkValidity()) {
      showAddError(repoInput.validity.valueMissing ?
        "Gib ein Repository an, zum Beispiel pklaschka/feldspur." :
        "Das sieht nicht nach einem Repository aus. Erwartet wird owner/name für GitHub, host/owner/name für Forgejo und Gitea oder die URL.");
      repoInput.setAttribute("aria-invalid", "true");
      repoInput.focus();
      return;
    }
    const submit = $<HTMLButtonElement>('button[type="submit"]', addForm);
    submit.disabled = true;
    showAddError("");
    try {
      const prerelease = (addForm.elements.namedItem("prerelease") as HTMLInputElement).checked;
      await AppService.Add(repoInput.value, prerelease);
      addForm.reset();
      repoInput.removeAttribute("aria-invalid");
    } catch (err) {
      showAddError(errorText(err));
      repoInput.setAttribute("aria-invalid", "true");
    } finally {
      submit.disabled = false;
    }
  });

  repoInput.addEventListener("input", () => {
    if (repoInput.getAttribute("aria-invalid") === "true" && repoInput.checkValidity()) {
      repoInput.removeAttribute("aria-invalid");
      showAddError("");
    }
  });
}

// showAddError writes under the strip; adding from a link reports there too.
export function showAddError(message: string) {
  addError.textContent = message;
}
