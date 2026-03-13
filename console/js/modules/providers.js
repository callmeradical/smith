import { state } from "./state.js";
import { fetchJSON, postJSON } from "./api.js";
import { pushToast, escapeHtml } from "./ui.js";
import { providerCatalog } from "./constants.js";
import {
  getProviderListEl,
  getProviderEmptyEl,
  getProviderConfigPanelEl,
  getProviderConfigTitleEl,
  getProviderCodexPanelEl,
  getProviderCatalogStatusEl,
  getAuthAPIKeyEl,
  getAuthAccountIDEl
} from "./elements.js";

export function setProviderConfigOpen(open) {
  const isOpen = Boolean(open);
  document.body.classList.toggle("provider-drawer-open", isOpen);
  const panel = getProviderConfigPanelEl();
  if (panel) {
    panel.classList.toggle("open", isOpen);
    panel.setAttribute("aria-hidden", isOpen ? "false" : "true");
  }
  if (!isOpen) {
    const codexPanel = getProviderCodexPanelEl();
    if (codexPanel) codexPanel.classList.add("hidden");
    state.activeProvider = "";
    state.providerCredentialRevealed = false;
    renderProviderList();
  }
}

export function renderProviderList() {
  const listEl = getProviderListEl();
  if (!listEl) return;
  listEl.innerHTML = "";
  const visibleProviders = providerCatalog.filter(p => p.id === "codex" || state.showComingSoonProviders);
  const emptyEl = getProviderEmptyEl();
  if (emptyEl) emptyEl.classList.toggle("hidden", visibleProviders.length > 0);
  
  for (const providerInfo of visibleProviders) {
    const providerID = providerInfo.id;
    const isCodex = providerID === "codex";
    const providerStatus = state.providerStatus[providerID] || {};
    const connected = Boolean(providerStatus.connected);
    const card = document.createElement("article");
    card.className = "provider-card" + (connected ? " connected" : "") + (state.activeProvider === providerID ? " active" : "");
    card.innerHTML = `
      <div class="provider-card-head">
        <span class="provider-card-name">${escapeHtml(providerInfo.label)}</span>
      </div>
      <div class="provider-card-desc">${escapeHtml(providerInfo.subtitle || "")}</div>
      <div class="provider-card-actions">
        <button type="button" class="${isCodex ? "primary" : ""}" data-provider-config="${escapeHtml(providerID)}">configure</button>
        <span class="provider-card-status">${connected ? "configured" : isCodex ? "available" : "coming soon"}</span>
      </div>
    `;
    listEl.appendChild(card);
  }
}

export function configureProvider(providerID) {
  const target = String(providerID || "").trim().toLowerCase();
  if (target !== "codex") {
    const statusEl = getProviderCatalogStatusEl();
    if (statusEl) statusEl.textContent = (providerCatalog.find((p) => p.id === target)?.label || target) + " configuration is coming soon.";
    return;
  }

  const codexPanel = getProviderCodexPanelEl();
  if (codexPanel) codexPanel.classList.remove("hidden");

  state.activeProvider = target;
  setProviderConfigOpen(true);
}

export async function saveAPIKey() {
  const keyEl = getAuthAPIKeyEl();
  const accountEl = getAuthAccountIDEl();
  const key = keyEl ? keyEl.value.trim() : "";
  if (!key) return;
  try {
    await postJSON("/v1/auth/codex/connect/api-key", {
      api_key: key,
      account_id: accountEl ? accountEl.value.trim() : "",
    });
    state.providerStatus.codex = { ...state.providerStatus.codex, connected: true };
    if (keyEl) keyEl.value = "";
    setProviderConfigOpen(false);
    renderProviderList();
  } catch (err) {
    pushToast("Failed to save API key: " + err.message, "err");
  }
}

export async function disconnectProvider() {
  try {
    await postJSON("/v1/auth/codex/disconnect", {});
    state.providerStatus.codex = { ...state.providerStatus.codex, connected: false };
    setProviderConfigOpen(false);
    renderProviderList();
  } catch (err) {
    pushToast("Failed to disconnect: " + err.message, "err");
  }
}
