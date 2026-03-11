import { f as attr_style, c as escape_html, a as attr_class, d as stringify, g as derived, h as bind_props, b as attr, s as store_get, e as ensure_array_like, u as unsubscribe_stores } from "../../../chunks/index2.js";
import { a as appState, p as pushToast } from "../../../chunks/stores.js";
import { T as TopBar } from "../../../chunks/TopBar.js";
import { E as EmptyState } from "../../../chunks/EmptyState.js";
import "codemirror-ssr";
import "codemirror-ssr/addon/display/placeholder.js";
import "codemirror-ssr/addon/edit/continuelist.js";
import "codemirror-ssr/addon/mode/overlay.js";
import "codemirror-ssr/mode/gfm/gfm.js";
import "codemirror-ssr/mode/markdown/markdown.js";
import "codemirror-ssr/mode/xml/xml.js";
import "codemirror-ssr/mode/yaml-frontmatter/yaml-frontmatter.js";
import "codemirror-ssr/mode/yaml/yaml.js";
import "select-files";
import { o as onDestroy } from "../../../chunks/index-server.js";
import "word-count";
import { defaultSchema } from "hast-util-sanitize";
import gfm from "@bytemd/plugin-gfm";
import "clsx";
async function fetchWithTimeout(url, options = {}, timeoutMs = 2e4, label) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    return await fetch(url, { ...options, signal: controller.signal });
  } catch (err) {
    if (err && err.name === "AbortError") {
      throw new Error(`Request timed out after ${Math.ceil(timeoutMs / 1e3)}s for ${url}`);
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}
async function postJSON(path, payload) {
  return requestJSON(path, "POST", payload);
}
async function requestJSON(path, method, payload) {
  const res = await fetchWithTimeout(`/api${path}`, {
    method,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json"
    },
    body: payload === void 0 ? void 0 : JSON.stringify(payload)
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const msg = body.error || `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return body;
}
function DocTile($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { doc } = $$props;
    const updatedAtLabel = derived(() => doc.updated_at ? new Date(doc.updated_at).toLocaleString() : "unknown date");
    $$renderer2.push(`<div class="pod-tile"${attr_style(doc.status === "archived" ? "opacity: 0.6" : "")}><div class="tile-head"><div class="tile-title loop-id">${escape_html(doc.title || "Untitled")}</div> <div${attr_class(`badge ${stringify(doc.status === "active" ? "state-synced" : "state-cancelled")}`)}>${escape_html(doc.status || "active")}</div></div> <div class="tile-meta"><span class="muted">${escape_html(doc.source_type || "unknown")}: ${escape_html(doc.source_ref || "direct")}</span> <span class="muted">${escape_html(updatedAtLabel())}</span></div> <div class="tile-footer" style="margin-top: 12px; display: flex; gap: 8px; justify-content: flex-start;"><button class="tile-action-button">Edit</button> <button class="tile-action-button primary">Build</button> <button class="tile-action-button">${escape_html(doc.status === "active" ? "Archive" : "Unarchive")}</button> <button class="tile-action-button danger">Delete</button></div></div>`);
  });
}
JSON.stringify(defaultSchema);
function DocEditorModal($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { open, title = void 0, content = void 0, onClose, onSave } = $$props;
    [gfm()];
    onDestroy(() => {
    });
    if (open) {
      $$renderer2.push("<!--[0-->");
      $$renderer2.push(`<div class="provider-drawer-overlay" style="opacity: 1; visibility: visible; pointer-events: auto; backdrop-filter: blur(8px);"></div> <aside class="doc-create-modal open svelte-1hkhnfw" aria-hidden="false"><div class="provider-drawer-head"><div class="provider-drawer-title">Edit Document</div> <button type="button" class="provider-drawer-close">×</button></div> <section class="panel" style="flex: 1; display: flex; flex-direction: column; overflow: hidden; gap: 12px;"><input type="text" placeholder="Document Title"${attr("value", title)} style="font-size: 1.2rem; font-weight: bold; padding: 8px;"/> <div class="editor-container svelte-1hkhnfw"></div> <div class="doc-actions" style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px;"><button>cancel</button> <button class="primary">save</button></div></section></aside>`);
    } else {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]-->`);
    bind_props($$props, { title, content });
  });
}
function DocChatModal($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    onDestroy(() => {
    });
    {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let showAll = false;
    let editorOpen = false;
    let editTitle = "";
    let editContent = "";
    const filteredDocs = derived(() => store_get($$store_subs ??= {}, "$appState", appState).documents.filter((d) => {
      if (!d) return false;
      const query = store_get($$store_subs ??= {}, "$appState", appState).docSearchQuery.toLowerCase();
      const matchesQuery = (d.title || "").toLowerCase().includes(query) || (d.id || "").toLowerCase().includes(query) || (d.project_id || "").toLowerCase().includes(query);
      const matchesProject = store_get($$store_subs ??= {}, "$appState", appState).docFilterProject === "all" || store_get($$store_subs ??= {}, "$appState", appState).docFilterProject === "" || d.project_id === store_get($$store_subs ??= {}, "$appState", appState).docFilterProject;
      const matchesStatus = d.status === "active" || d.status === "unresolved";
      return matchesQuery && matchesProject && matchesStatus;
    }));
    const projects = derived(() => Array.from(new Set(store_get($$store_subs ??= {}, "$appState", appState).documents.map((d) => d.project_id).filter(Boolean))).sort());
    const groupedDocs = derived(() => {
      if (store_get($$store_subs ??= {}, "$appState", appState).docFilterProject !== "all" && store_get($$store_subs ??= {}, "$appState", appState).docFilterProject !== "") {
        return {
          [store_get($$store_subs ??= {}, "$appState", appState).docFilterProject]: filteredDocs()
        };
      }
      const grouped = {};
      filteredDocs().forEach((doc) => {
        if (!doc.project_id) return;
        if (!grouped[doc.project_id]) grouped[doc.project_id] = [];
        grouped[doc.project_id].push(doc);
      });
      return grouped;
    });
    const sortedProjectIDs = derived(() => Object.keys(groupedDocs()).sort());
    function handleEditorSave(title, content) {
      {
        const projectID = store_get($$store_subs ??= {}, "$appState", appState).docFilterProject === "all" || store_get($$store_subs ??= {}, "$appState", appState).docFilterProject === "" ? store_get($$store_subs ??= {}, "$appState", appState).projects[0]?.id || "default" : store_get($$store_subs ??= {}, "$appState", appState).docFilterProject;
        postJSON("/v1/documents", {
          project_id: projectID,
          title: title || "New Document",
          content,
          format: "markdown",
          status: "active",
          source_type: "direct"
        }).then(() => {
          pushToast("Document created", "ok");
        }).catch((err) => {
          pushToast(err.message, "err");
        });
      }
      editorOpen = false;
    }
    function controls($$renderer3) {
      $$renderer3.push(`<input type="search" placeholder="Filter documents"${attr("value", store_get($$store_subs ??= {}, "$appState", appState).docSearchQuery)}/> <label class="muted" style="display: flex; align-items: center; gap: 4px; cursor: pointer;"><input type="checkbox"${attr("checked", showAll, true)}/> Show All</label> <button class="primary" style="margin-left: 8px;">Draft with AI</button> <button>New Doc</button>`);
    }
    let $$settled = true;
    let $$inner_renderer;
    function $$render_inner($$renderer3) {
      TopBar($$renderer3, { title: "Documents", controls });
      $$renderer3.push(`<!----> `);
      DocEditorModal($$renderer3, {
        onClose: () => editorOpen = false,
        onSave: handleEditorSave,
        get open() {
          return editorOpen;
        },
        set open($$value) {
          editorOpen = $$value;
          $$settled = false;
        },
        get title() {
          return editTitle;
        },
        set title($$value) {
          editTitle = $$value;
          $$settled = false;
        },
        get content() {
          return editContent;
        },
        set content($$value) {
          editContent = $$value;
          $$settled = false;
        }
      });
      $$renderer3.push(`<!----> `);
      DocChatModal($$renderer3);
      $$renderer3.push(`<!----> <div class="doc-container"><aside id="doc-sidebar" class="doc-sidebar"><div class="doc-sidebar-section"><div class="doc-sidebar-header">Projects</div> <div class="doc-sidebar-list"><button${attr_class("doc-sidebar-item svelte-220hbx", void 0, {
        "active": store_get($$store_subs ??= {}, "$appState", appState).docFilterProject === "all"
      })}>All Projects</button> `);
      if (projects().length === 0) {
        $$renderer3.push("<!--[0-->");
        $$renderer3.push(`<div class="doc-sidebar-item muted svelte-220hbx">📁 (Empty)</div>`);
      } else {
        $$renderer3.push("<!--[-1-->");
        $$renderer3.push(`<!--[-->`);
        const each_array = ensure_array_like(projects());
        for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
          let p = each_array[$$index];
          $$renderer3.push(`<button${attr_class("doc-sidebar-item svelte-220hbx", void 0, {
            "active": store_get($$store_subs ??= {}, "$appState", appState).docFilterProject === p
          })}>${escape_html(p)}</button>`);
        }
        $$renderer3.push(`<!--]-->`);
      }
      $$renderer3.push(`<!--]--></div></div></aside> <main class="doc-main">`);
      if (store_get($$store_subs ??= {}, "$appState", appState).docFilterProject === "") {
        $$renderer3.push("<!--[0-->");
        EmptyState($$renderer3, {
          title: "Document Explorer",
          description: "Select a project from the sidebar to view associated Product Requirement Documents (PRDs) and technical specs.",
          icon: "🔍"
        });
      } else if (sortedProjectIDs().length === 0) {
        $$renderer3.push("<!--[1-->");
        EmptyState($$renderer3, {
          title: "No Documents Found",
          description: "There are no documents matching your current filters for this project.",
          buttonText: "Draft with AI",
          icon: "📄"
        });
      } else {
        $$renderer3.push("<!--[-1-->");
        $$renderer3.push(`<div class="project-loop-list svelte-220hbx"><!--[-->`);
        const each_array_1 = ensure_array_like(sortedProjectIDs());
        for (let $$index_2 = 0, $$length = each_array_1.length; $$index_2 < $$length; $$index_2++) {
          let projectID = each_array_1[$$index_2];
          $$renderer3.push(`<details class="project-tile" open=""><summary class="collapsible-summary"><span class="collapsible-label"><span class="collapsible-caret">></span> <span class="project-name">${escape_html(projectID)}</span></span></summary> <div class="collapsible-body"><div class="pod-grid"><!--[-->`);
          const each_array_2 = ensure_array_like(groupedDocs()[projectID]);
          for (let $$index_1 = 0, $$length2 = each_array_2.length; $$index_1 < $$length2; $$index_1++) {
            let doc = each_array_2[$$index_1];
            DocTile($$renderer3, {
              doc
            });
          }
          $$renderer3.push(`<!--]--></div></div></details>`);
        }
        $$renderer3.push(`<!--]--></div>`);
      }
      $$renderer3.push(`<!--]--></main></div>`);
    }
    do {
      $$settled = true;
      $$inner_renderer = $$renderer2.copy();
      $$render_inner($$inner_renderer);
    } while (!$$settled);
    $$renderer2.subsume($$inner_renderer);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
