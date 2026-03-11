import { a as attr_class, s as store_get, e as ensure_array_like, b as attr, c as escape_html, u as unsubscribe_stores, d as stringify } from "../../chunks/index2.js";
import { s as sidebarOpen, t as toastMessages } from "../../chunks/stores.js";
import { p as page } from "../../chunks/index3.js";
function Sidebar($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    const navItems = [
      { id: "pods", label: "Pods", href: "/pods" },
      { id: "documents", label: "Documents", href: "/documents" },
      { id: "projects", label: "Projects", href: "/projects" },
      { id: "providers", label: "Providers", href: "/providers" },
      { id: "controls", label: "Manual Controls", href: "/controls" }
    ];
    $$renderer2.push(`<aside${attr_class("sidebar svelte-129hoe0", void 0, {
      "open": store_get($$store_subs ??= {}, "$sidebarOpen", sidebarOpen)
    })} aria-label="Console sidebar"><section class="brand"><div class="brand-line"><span id="api-dot" class="dot" aria-hidden="true"></span> <span>SMITH</span></div> <div class="api-url" id="api-url">API URL</div></section> <nav class="quick-nav" aria-label="Configuration pages"><!--[-->`);
    const each_array = ensure_array_like(navItems);
    for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
      let item = each_array[$$index];
      $$renderer2.push(`<a${attr_class("nav-link", void 0, { "active": page.url.pathname.startsWith(item.href) })}${attr("href", item.href)}>${escape_html(item.label)}</a>`);
    }
    $$renderer2.push(`<!--]--></nav></aside>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
function Toast($$renderer) {
  var $$store_subs;
  $$renderer.push(`<div id="toast-region" class="toast-region" aria-live="polite" aria-atomic="false"><!--[-->`);
  const each_array = ensure_array_like(store_get($$store_subs ??= {}, "$toastMessages", toastMessages));
  for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
    let toast = each_array[$$index];
    $$renderer.push(`<div${attr_class(`toast ${stringify(toast.level)}`, void 0, { "show": toast.show })}>${escape_html(toast.message)}</div>`);
  }
  $$renderer.push(`<!--]--></div>`);
  if ($$store_subs) unsubscribe_stores($$store_subs);
}
function _layout($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let { children } = $$props;
    $$renderer2.push(`<div id="sidebar-overlay"${attr_class("sidebar-overlay svelte-12qhfyh", void 0, {
      "open": store_get($$store_subs ??= {}, "$sidebarOpen", sidebarOpen)
    })} aria-hidden="true"></div> <div id="provider-drawer-overlay" class="provider-drawer-overlay" aria-hidden="true"></div> `);
    Toast($$renderer2);
    $$renderer2.push(`<!----> <div class="shell">`);
    Sidebar($$renderer2);
    $$renderer2.push(`<!----> <main class="workspace">`);
    children($$renderer2);
    $$renderer2.push(`<!----></main></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _layout as default
};
