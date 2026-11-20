import {$, clone} from "./dom";

// A pick list is a list of tick boxes set like a packing list, one line per
// entry (template #manifest-item, classes .manifest*).
export interface PickItem {
  value: string; // the tick box's value, also shown as the label
  state: string; // the line's data-state, such as "new", "known" or "invalid"
  tag?: boolean; // shows the template's tag (»Vorab«)
  note?: string; // after the label, or on a line of its own for "invalid"
  checked?: boolean;
  disabled?: boolean;
}

const template = $<HTMLTemplateElement>("#manifest-item");

export function fillPickList(list: HTMLUListElement, items: PickItem[]) {
  list.replaceChildren(...items.map((it) => {
    const li = clone<HTMLLIElement>(template);
    li.dataset.state = it.state;
    const box = $<HTMLInputElement>("input", li);
    box.value = it.value;
    box.checked = !!it.checked;
    box.disabled = !!it.disabled;
    $(".manifest-repo", li).textContent = it.value;
    $<HTMLElement>(".channel-tag", li).hidden = !it.tag;
    $(".manifest-note", li).textContent = it.note ?? "";
    return li;
  }));
}

// picked lists the values that are ticked and can still be changed.
export function picked(list: HTMLUListElement): string[] {
  return [...list.querySelectorAll<HTMLInputElement>("input:checked:not(:disabled)")].map((b) => b.value);
}
