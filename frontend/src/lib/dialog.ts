// ask shows a modal dialog and resolves with the value of the button that
// closed it, or "" for Esc and other close requests. Asking again while a
// question is still open ends the earlier one with "", so every answer
// belongs to its own question.
export function ask(dialog: HTMLDialogElement): Promise<string> {
  if (!listening.has(dialog)) {
    listening.add(dialog);
    dialog.addEventListener("close", () => {
      if (dialog.open) return; // the late close of a question that was replaced
      const value = dialog.returnValue;
      // Esc sets no value, and browsers differ on whether they clear the old one
      dialog.returnValue = "";
      settle(dialog, value);
    });
  }
  settle(dialog, "");
  if (dialog.open) dialog.close();
  dialog.returnValue = "";
  dialog.showModal();
  return new Promise((resolve) => pending.set(dialog, resolve));
}

const listening = new WeakSet<HTMLDialogElement>();
const pending = new WeakMap<HTMLDialogElement, (value: string) => void>();

function settle(dialog: HTMLDialogElement, value: string) {
  const resolve = pending.get(dialog);
  pending.delete(dialog);
  resolve?.(value);
}

// setDialogState names the dialog's state in data-state and shows only the
// elements whose data-show (a space-separated list) names it.
export function setDialogState(dialog: HTMLDialogElement, state: string) {
  dialog.dataset.state = state;
  for (const el of dialog.querySelectorAll<HTMLElement>("[data-show]")) {
    el.hidden = !el.dataset.show!.split(" ").includes(state);
  }
}
