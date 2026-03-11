import { c as escape_html, b as attr } from "./index2.js";
function EmptyState($$renderer, $$props) {
  let { title, description, buttonText, buttonHref, icon = "📁" } = $$props;
  $$renderer.push(`<div class="empty-state-container svelte-13862ru"><div class="empty-state-icon svelte-13862ru">${escape_html(icon)}</div> <h2 class="empty-state-title svelte-13862ru">${escape_html(title)}</h2> <p class="empty-state-description svelte-13862ru">${escape_html(description)}</p> `);
  if (buttonText && buttonHref) {
    $$renderer.push("<!--[0-->");
    $$renderer.push(`<a${attr("href", buttonHref)} class="primary empty-state-cta svelte-13862ru">${escape_html(buttonText)}</a>`);
  } else {
    $$renderer.push("<!--[-1-->");
  }
  $$renderer.push(`<!--]--></div>`);
}
export {
  EmptyState as E
};
