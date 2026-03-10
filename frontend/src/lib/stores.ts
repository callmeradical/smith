import { writable } from 'svelte/store';

export const appState = writable({
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
});

export const sidebarOpen = writable(false);
export const toastMessages = writable([]);

export function pushToast(message: string, level: 'ok' | 'err' | 'muted' = 'muted') {
  const id = Math.random().toString(36).substring(2);
  toastMessages.update(messages => [...messages, { id, message, level, show: true }]);
  
  setTimeout(() => {
    toastMessages.update(messages => messages.map(m => m.id === id ? { ...m, show: false } : m));
    setTimeout(() => {
      toastMessages.update(messages => messages.filter(m => m.id !== id));
    }, 160);
  }, 3200);
}
