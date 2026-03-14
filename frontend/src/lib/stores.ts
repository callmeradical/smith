import { writable } from 'svelte/store';

export interface AppState {
  loops: any[];
  selectedLoop: string;
  journalEntries: any[];
  journalLastSeq: number;
  latencySamplesMs: number[];
  journalSource: EventSource | null;
  reconnectTimer: any;
  authBusy: boolean;
  authFormDirty: boolean;
  activeProvider: string;
  providerCredentialRevealed: boolean;
  projects: any[];
  projectCredentials: Record<string, any>;
  projectFormBusy: boolean;
  loopWorkflow: Record<string, any>;
  editingProjectID: string;
  podProjectIssues: Record<string, any[]>;
  podCreateBusy: boolean;
  podCreateStep: number;
  podCreateMethod: string;
  podCreateFinalPRD: any;
  podCreateChatSocket: WebSocket | null;
  podCreateChatMessages: any[];
  attachBusyLoopID: string;
  loopDeleteBusy: boolean;
  loopControlBusy: boolean;
  loopControlAction: string;
  loopControlBusyLoopID: string;
  pendingPodViewLoopID: string;
  terminalUIState: string;
  terminalMessage: string;
  terminalAttachedLoopID: string;
  runtimeByLoopID: Record<string, any>;
  runtimeBusyLoopID: string;
  providerStatus: Record<string, any>;
  documents: any[];
  docFilterProject: string;
  docFilterStatus: string;
  docCreateBusy: boolean;
  docCreateStep: number;
  docCreateMethod: string;
  docSearchQuery: string;
}

export const appState = writable<AppState>({
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
export const chatOpen = writable(false);
export const chatType = writable('prd-refinement');

export interface ToastMessage {
  id: string;
  message: string;
  level: 'ok' | 'err' | 'muted';
  show: boolean;
}

export const toastMessages = writable<ToastMessage[]>([]);

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
