import { state } from "./state.js";
import { 
  getToastRegionEl, 
  getSidebarEl, 
  getSidebarToggleButtons, 
  getPages, 
  getPageLinks,
  getProviderDrawerOverlayEl,
  getJournalLatencyEl
} from "./elements.js";

export function pushToast(message, level) {
  const text = String(message || "").trim();
  if (!text) return;
  const tone = level === "ok" ? "ok" : level === "err" ? "err" : "muted";
  const toast = document.createElement("div");
  toast.className = "toast " + tone;
  toast.textContent = text;
  const region = getToastRegionEl();
  if (!region) return;
  region.appendChild(toast);
  while (region.children.length > 4) {
    region.removeChild(region.firstChild);
  }
  requestAnimationFrame(() => toast.classList.add("show"));
  window.setTimeout(() => {
    toast.classList.remove("show");
    window.setTimeout(() => {
      if (toast.parentNode === region) {
        region.removeChild(toast);
      }
    }, 160);
  }, 3200);
}

export function setSidebarOpen(open) {
  const isOpen = Boolean(open);
  document.body.classList.toggle("sidebar-open", isOpen);
  const sidebar = getSidebarEl();
  if (sidebar) sidebar.setAttribute("aria-hidden", isOpen ? "false" : "true");
  getSidebarToggleButtons().forEach((btn) => {
    btn.setAttribute("aria-expanded", isOpen ? "true" : "false");
  });
}

export function syncRightDrawerOverlay() {
  const open =
    document.body.classList.contains("provider-drawer-open") ||
    document.body.classList.contains("project-drawer-open") ||
    document.body.classList.contains("pod-drawer-open");
  const overlay = getProviderDrawerOverlayEl();
  if (overlay) overlay.setAttribute("aria-hidden", open ? "false" : "true");
}

export function escapeHtml(v) {
  return String(v).replace(/[&<>"']/g, (m) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[m]));
}

export function setActivePage(page, updateHash) {
  const pages = getPages();
  let target = pages[page] ? page : "pods";
  const requestedPodViewLoopID =
    target === "podView" ? String(state.pendingPodViewLoopID || "").trim() : "";
  if (
    target === "podView" &&
    !requestedPodViewLoopID &&
    !String(state.selectedLoop || "").trim()
  ) {
    target = "pods";
  }
  for (const [name, el] of Object.entries(pages)) {
    if (el) el.classList.toggle("active", name === target);
  }
  const navTarget = target === "podView" ? "pods" : target;
  for (const link of getPageLinks()) {
    link.classList.toggle("active", link.getAttribute("data-page-link") === navTarget);
  }
  if (updateHash) {
    if (target === "podView") {
      const selected = String(state.selectedLoop || "").trim();
      window.location.hash = selected
        ? "#pod-view/" + encodeURIComponent(selected)
        : "#pods";
    } else if (target === "docView") {
      const selected = String(state.selectedDocument || "").trim();
      window.location.hash = selected
        ? "#doc-view/" + encodeURIComponent(selected)
        : "#documents";
    } else {
      window.location.hash = "#" + target;
    }
  }
  if (target !== "providers") {
    document.body.classList.remove("provider-drawer-open");
  }
  if (target !== "projects") {
    document.body.classList.remove("project-drawer-open");
  }
  if (target !== "pods" && target !== "podView") {
    document.body.classList.remove("pod-drawer-open");
  }

  if (target === "podView" && requestedPodViewLoopID && requestedPodViewLoopID !== state.selectedLoop) {
    state.selectedLoop = requestedPodViewLoopID;
  }
  if (target === "docView" && state.selectedDocument) {
    import("./docs.js").then(m => m.openDocumentDetail(state.selectedDocument));
  }
  state.pendingPodViewLoopID = "";
  syncRightDrawerOverlay();
}

export function renderLatency(samplesMs) {
  const p95 = percentile(samplesMs, 95);
  const el = getJournalLatencyEl();
  if (!el) return;
  if (p95 == null) {
    el.textContent = "p95 --";
    return;
  }
  el.textContent =
    "p95 " + Math.round(p95) + "ms (n=" + samplesMs.length + ")";
}

export function percentile(values, p) {
  if (!Array.isArray(values) || values.length === 0) return null;
  const sorted = values.slice().sort((a, b) => a - b);
  const rank = Math.ceil((p / 100) * sorted.length);
  const idx = Math.min(sorted.length - 1, Math.max(0, rank - 1));
  return sorted[idx];
}
