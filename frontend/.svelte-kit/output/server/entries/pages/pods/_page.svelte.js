import { a as attr_class, c as escape_html, d as stringify, s as store_get, e as ensure_array_like, u as unsubscribe_stores, g as derived, b as attr } from "../../../chunks/index2.js";
import { a as appState } from "../../../chunks/stores.js";
import { T as TopBar } from "../../../chunks/TopBar.js";
import "@sveltejs/kit/internal";
import "../../../chunks/exports.js";
import "../../../chunks/utils.js";
import "clsx";
import "@sveltejs/kit/internal/server";
import "../../../chunks/root.js";
import "../../../chunks/state.svelte.js";
import { E as EmptyState } from "../../../chunks/EmptyState.js";
function PodTile($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { loop, selected } = $$props;
    $$renderer2.push(`<article${attr_class("pod-tile", void 0, { "selected": selected })} role="listitem"><div class="tile-head"><div class="tile-title loop-id">${escape_html(loop.loopID)}</div></div> <div class="tile-loop">${escape_html(loop.project)}</div> <div class="tile-reason">${escape_html(loop.reason || "no recent update")}</div> <div class="tile-footer"><span${attr_class(`badge state-${stringify(loop.status)}`)}>${escape_html(loop.status)}</span> <div class="tile-meta"><span>ATT ${escape_html(loop.attempt)}</span> <span>REV ${escape_html(loop.revision)}</span></div></div></article>`);
  });
}
function PodCreateModal($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let stateFilter = "all";
    let searchQuery = "";
    let autoRefresh = true;
    const filteredLoops = derived(() => store_get($$store_subs ??= {}, "$appState", appState).loops.filter((loop) => {
      const matchesState = stateFilter === "all";
      const matchesSearch = !searchQuery;
      return matchesState && matchesSearch;
    }));
    const stats = derived(() => ({
      total: store_get($$store_subs ??= {}, "$appState", appState).loops.length,
      active: store_get($$store_subs ??= {}, "$appState", appState).loops.filter((l) => l.status === "unresolved" || l.status === "overwriting").length,
      flatline: store_get($$store_subs ??= {}, "$appState", appState).loops.filter((l) => l.status === "flatline").length
    }));
    function controls($$renderer3) {
      $$renderer3.select({ value: stateFilter, "aria-label": "State filter" }, ($$renderer4) => {
        $$renderer4.option({ value: "all" }, ($$renderer5) => {
          $$renderer5.push(`All States`);
        });
        $$renderer4.option({ value: "active" }, ($$renderer5) => {
          $$renderer5.push(`Active Only`);
        });
        $$renderer4.option({ value: "unresolved" }, ($$renderer5) => {
          $$renderer5.push(`Unresolved`);
        });
        $$renderer4.option({ value: "overwriting" }, ($$renderer5) => {
          $$renderer5.push(`Overwriting`);
        });
        $$renderer4.option({ value: "synced" }, ($$renderer5) => {
          $$renderer5.push(`Synced`);
        });
        $$renderer4.option({ value: "flatline" }, ($$renderer5) => {
          $$renderer5.push(`Flatline`);
        });
        $$renderer4.option({ value: "cancelled" }, ($$renderer5) => {
          $$renderer5.push(`Cancelled`);
        });
      });
      $$renderer3.push(` <input type="search" placeholder="Filter loop id"${attr("value", searchQuery)}/> <label class="muted"><input type="checkbox"${attr("checked", autoRefresh, true)}/> auto-refresh</label> <button>refresh</button>`);
    }
    TopBar($$renderer2, { title: "Pods", controls });
    $$renderer2.push(`<!----> <section class="stats"><div class="stat"><small>Total</small><strong>${escape_html(stats().total)}</strong></div> <div class="stat"><small>Active</small><strong>${escape_html(stats().active)}</strong></div> <div class="stat"><small>Flatline</small><strong>${escape_html(stats().flatline)}</strong></div> <div class="stat stat-action"><small>New Loop</small> <button type="button" class="stat-add-button" aria-label="Start loop">+</button></div></section> `);
    PodCreateModal($$renderer2);
    $$renderer2.push(`<!----> <section class="board"><section class="tiles-shell"><div class="pod-grid" role="list">`);
    if (store_get($$store_subs ??= {}, "$appState", appState).projects.length === 0) {
      $$renderer2.push("<!--[0-->");
      EmptyState($$renderer2, {
        title: "Welcome to SMITH",
        description: "To get started, you'll need to configure a project. Projects connect your repositories and enable autonomous development loops.",
        buttonText: "Configure Project",
        buttonHref: "/projects",
        icon: "🚀"
      });
    } else {
      $$renderer2.push("<!--[-1-->");
      const each_array = ensure_array_like(filteredLoops());
      if (each_array.length !== 0) {
        $$renderer2.push("<!--[-->");
        for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
          let loop = each_array[$$index];
          PodTile($$renderer2, {
            loop,
            selected: store_get($$store_subs ??= {}, "$appState", appState).selectedLoop === loop.loopID
          });
        }
      } else {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`<div class="empty">No pods found.</div>`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div></section></section>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
