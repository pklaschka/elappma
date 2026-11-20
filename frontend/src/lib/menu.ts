// placeUnder keeps a popover menu under its button, right-aligned (anchor
// positioning is not in every WebKit yet).
export function placeUnder(menu: HTMLElement, button: HTMLElement) {
  menu.addEventListener("beforetoggle", (e) => {
    if ((e as ToggleEvent).newState !== "open") return;
    const r = button.getBoundingClientRect();
    const {clientWidth, clientHeight} = document.documentElement;
    // below the button, or above it when the window is too short there
    const space = clientHeight - r.bottom;
    const below = space >= 180 || space >= r.top;
    Object.assign(menu.style, {
      top: below ? `${r.bottom + 4}px` : "auto",
      bottom: below ? "auto" : `${clientHeight - r.top + 4}px`,
      left: "auto",
      right: `${clientWidth - r.right}px`,
    });
  });
}
