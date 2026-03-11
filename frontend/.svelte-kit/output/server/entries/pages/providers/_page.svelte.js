import { e as ensure_array_like, c as escape_html, a as attr_class, ae as clsx } from "../../../chunks/index2.js";
import { T as TopBar } from "../../../chunks/TopBar.js";
function _page($$renderer) {
  const providers = [
    {
      id: "codex",
      label: "OpenAI Codex CLI",
      subtitle: "Execute OpenAI models via Codex CLI."
    }
    // Add others as needed for simulation
  ];
  TopBar($$renderer, { title: "Providers" });
  $$renderer.push(`<!----> <section id="provider-list-panel"><div class="provider-card-grid"><!--[-->`);
  const each_array = ensure_array_like(providers);
  for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
    let provider = each_array[$$index];
    $$renderer.push(`<article class="provider-card"><div class="provider-card-head"><span class="provider-card-name">${escape_html(provider.label)}</span></div> <div class="provider-card-desc">${escape_html(provider.subtitle)}</div> <div class="provider-card-actions"><button type="button"${attr_class(clsx(provider.id === "codex" ? "primary" : ""))}>configure</button> <span class="provider-card-status">available</span></div></article>`);
  }
  $$renderer.push(`<!--]--></div></section>`);
}
export {
  _page as default
};
