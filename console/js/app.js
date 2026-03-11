import { state, apiBase } from "./modules/state.js";
import { initEventListeners } from "./modules/events.js";
import { initApp } from "./modules/init.js";
import { setActivePage } from "./modules/ui.js";
import * as elements from "./modules/elements.js";

// Legacy exposure for tests
window.state = state;
window.setActivePage = setActivePage;
window.sidebarEl = document.querySelector(".sidebar");
window.pages = elements.getPages();

function pageFromHash() {
  const rawValue = String(window.location.hash || "").replace(/^#/, "").trim();
  const raw = rawValue.toLowerCase();
  if (raw.startsWith("pod-view/")) {
    const encoded = rawValue.slice("pod-view/".length);
    try {
      state.pendingPodViewLoopID = decodeURIComponent(encoded);
    } catch (_) {
      state.pendingPodViewLoopID = encoded;
    }
    return "podView";
  }
  if (raw === "pod-view") {
    state.pendingPodViewLoopID = "";
    return "podView";
  }
  if (raw.startsWith("doc-view/")) {
    const encoded = rawValue.slice("doc-view/".length);
    try {
      state.selectedDocument = decodeURIComponent(encoded);
    } catch (_) {
      state.selectedDocument = encoded;
    }
    return "docView";
  }
  const validPages = ["pods", "podView", "documents", "docView", "projects", "providers", "controls"];
  return validPages.includes(raw) ? raw : "pods";
}

// Global initialization
console.log('APP: Module loading...');
const apiUrlEl = document.getElementById("api-url");
if (apiUrlEl) apiUrlEl.textContent = apiBase || "(api unset)";

console.log('APP: Initializing event listeners...');
initEventListeners();
console.log('APP: Initializing app data...');
void initApp();

setActivePage(pageFromHash(), false);

window.addEventListener("hashchange", () => {
  setActivePage(pageFromHash(), false);
});
