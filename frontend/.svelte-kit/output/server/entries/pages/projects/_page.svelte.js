import { s as store_get, e as ensure_array_like, c as escape_html, u as unsubscribe_stores } from "../../../chunks/index2.js";
import { a as appState } from "../../../chunks/stores.js";
import { T as TopBar } from "../../../chunks/TopBar.js";
import "clsx";
import { E as EmptyState } from "../../../chunks/EmptyState.js";
function ProjectEditorDrawer($$renderer, $$props) {
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
    function controls($$renderer3) {
      $$renderer3.push(`<button class="icon-button">+</button>`);
    }
    TopBar($$renderer2, { title: "Projects", controls });
    $$renderer2.push(`<!----> `);
    ProjectEditorDrawer($$renderer2);
    $$renderer2.push(`<!----> <section id="project-list-panel"><div class="project-list">`);
    if (store_get($$store_subs ??= {}, "$appState", appState).projects.length === 0) {
      $$renderer2.push("<!--[0-->");
      EmptyState($$renderer2, {
        title: "No Projects Configured",
        description: "Projects define the repositories and environments for your autonomous loops. Create one to get started.",
        buttonText: "Create Project",
        buttonHref: "#",
        icon: "🏗️"
      });
    } else {
      $$renderer2.push("<!--[-1-->");
      $$renderer2.push(`<!--[-->`);
      const each_array = ensure_array_like(store_get($$store_subs ??= {}, "$appState", appState).projects);
      for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
        let project = each_array[$$index];
        $$renderer2.push(`<details class="project-tile" open=""><summary class="collapsible-summary"><span class="collapsible-label"><span class="collapsible-caret">></span> <span class="project-name">${escape_html(project.name)}</span></span></summary> <div class="collapsible-body"><div class="project-repo">${escape_html(project.repo_url)}</div> <div class="project-card-actions"><button type="button" class="project-action-icon">✎</button></div></div></details>`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div></section>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
