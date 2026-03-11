export const config = window.__SMITH_CONFIG__ || {};
export const apiBase = String(config.apiBaseUrl || "").replace(/\/+$/, "");
export const operatorToken = String(config.operatorToken || "").trim();
export const requestTimeoutMs = (() => {
  const parsed = Number(config.requestTimeoutMs || 20000);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 20000;
})();
export const showComingSoonProviders = /^(1|true|yes|on)$/i.test(
  String(config.showComingSoonProviders || ""),
);

export const state = {
  loops: [],
  selectedLoop: "",
  journalEntries: [],
  journalLastSeq: 0,
  latencySamplesMs: [],
  journalSource: null,
  reconnectTimer: null,
  authBusy: false,
  authFormDirty: false,
  activeProvider: "",
  providerCredentialRevealed: false,
  projects: [],
  projectCredentials: {},
  projectFormBusy: false,
  loopWorkflow: {},
  editingProjectID: "",
  podProjectIssues: {},
  podCreateBusy: false,
  podCreateStep: 1,
  podCreateMethod: "issue",
  podCreateFinalPRD: null,
  podCreateChatSocket: null,
  podCreateChatMessages: [],
  attachBusyLoopID: "",
  loopDeleteBusy: false,
  loopControlBusy: false,
  loopControlAction: "",
  loopControlBusyLoopID: "",
  pendingPodViewLoopID: "",
  terminalUIState: "idle",
  terminalMessage: "",
  terminalAttachedLoopID: "",
  runtimeByLoopID: {},
  runtimeBusyLoopID: "",
  providerStatus: {
    codex: {
      connected: false,
      account_id: "",
      expires_at: "",
      last_refresh_at: "",
      auth_method: "",
      api_key_masked: "",
      api_key_revealed: "",
    },
  },
  documents: [],
  docFilterProject: "",
  docFilterStatus: "active",
  docCreateBusy: false,
  docCreateStep: 1,
  docCreateMethod: "issue",
  docSearchQuery: "",
};
