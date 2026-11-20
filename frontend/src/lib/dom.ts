export const $ = <T extends Element>(sel: string, root: ParentNode = document) => root.querySelector(sel) as T;

// clone returns a fresh copy of the first element in a <template>.
export function clone<T extends Element>(template: HTMLTemplateElement): T {
  return (template.content.cloneNode(true) as DocumentFragment).firstElementChild as T;
}
