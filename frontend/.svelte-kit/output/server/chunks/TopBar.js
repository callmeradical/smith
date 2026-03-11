import { b as attr, s as store_get, c as escape_html, u as unsubscribe_stores } from "./index2.js";
import { s as sidebarOpen } from "./stores.js";
function TopBar($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let { title, controls } = $$props;
    $$renderer2.push(`<header class="topbar"><div class="topbar-left"><button type="button" class="sidebar-toggle" aria-label="Toggle sidebar"${attr("aria-expanded", store_get($$store_subs ??= {}, "$sidebarOpen", sidebarOpen))}>☰</button> <div class="topbar-title">${escape_html(title)}</div></div> <div class="controls">`);
    if (controls) {
      $$renderer2.push("<!--[0-->");
      controls($$renderer2);
      $$renderer2.push(`<!---->`);
    } else {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]--></div></header>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  TopBar as T
};
