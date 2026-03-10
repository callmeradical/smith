export const getGridEl = () => document.getElementById("grid");
export const getTerminalEl = () => document.getElementById("terminal");
export const getPodViewBackEl = () => document.getElementById("pod-view-back");
export const getPodViewAttachEl = () => document.getElementById("pod-view-attach");
export const getPodViewCancelEl = () => document.getElementById("pod-view-cancel");
export const getPodViewTerminateEl = () => document.getElementById("pod-view-terminate");
export const getPodViewDeleteEl = () => document.getElementById("pod-view-delete");
export const getPodViewCommandEl = () => document.getElementById("pod-view-command");
export const getPodViewRunEl = () => document.getElementById("pod-view-run");
export const getPodViewTerminalStateEl = () => document.getElementById("pod-view-terminal-state");
export const getPodViewJournalShellEl = () => document.getElementById("pod-view-journal-shell");
export const getPodViewRuntimeTargetEl = () => document.getElementById("pod-view-runtime-target");
export const getPodViewControlMessageEl = () => document.getElementById("pod-view-control-message");
export const getPodViewTitleEl = () => document.getElementById("pod-view-title");
export const getPodsNoticeEl = () => document.getElementById("pods-notice");
export const getPodCreateToggleEl = () => document.getElementById("pod-create-toggle");
export const getPodCreatePanelEl = () => document.getElementById("pod-create-panel");
export const getPodCreateProjectEl = () => document.getElementById("pod-create-project");
export const getPodCreateIssueFilterEl = () => document.getElementById("pod-create-issue-filter");
export const getPodCreateIssueEl = () => document.getElementById("pod-create-issue");
export const getPodCreateLoopNameEl = () => document.getElementById("pod-create-loop-name");
export const getPodCreateBranchEl = () => document.getElementById("pod-create-branch");
export const getPodCreateSourceBranchEl = () => document.getElementById("pod-create-source-branch");
export const getPodCreateProviderEl = () => document.getElementById("pod-create-provider");
export const getPodCreatePromptEl = () => document.getElementById("pod-create-prompt");
export const getPodCreatePRDFileEl = () => document.getElementById("pod-create-prd-file");
export const getPodCreateIssuePreviewEl = () => document.getElementById("pod-create-issue-preview");
export const getPodCreateBackEl = () => document.getElementById("pod-create-back");
export const getPodCreateNextEl = () => document.getElementById("pod-create-next");
export const getPodCreateSubmitEl = () => document.getElementById("pod-create-submit");
export const getPodCreateCancelEl = () => document.getElementById("pod-create-cancel");
export const getPodCreateCloseEl = () => document.getElementById("pod-create-close");
export const getPodCreateStatusEl = () => document.getElementById("pod-create-status");
export const getPodCreateChatPanelEl = () => document.getElementById("pod-create-chat-panel");
export const getPodCreateChatInputEl = () => document.getElementById("pod-create-chat-input");
export const getPodCreateChatSendEl = () => document.getElementById("pod-create-chat-send");
export const getPodCreateChatStatusEl = () => document.getElementById("pod-create-chat-status");

export const getPodCreateStepChips = () => ({
  1: document.getElementById("pod-create-step-chip-1"),
  2: document.getElementById("pod-create-step-chip-2"),
  3: document.getElementById("pod-create-step-chip-3"),
  4: document.getElementById("pod-create-step-chip-4"),
});

export const getPodCreateStepPanels = () => Array.from(document.querySelectorAll("[data-pod-create-step]"));
export const getPodCreateMethodButtons = () => Array.from(document.querySelectorAll("[data-pod-create-method]"));
export const getPodCreateMethodPanels = () => Array.from(document.querySelectorAll("[data-pod-create-method-panel]"));

export const getApiDotEl = () => document.getElementById("api-dot");
export const getJournalLatencyEl = () => document.getElementById("journal-latency");
export const getJournalStatusEl = () => document.getElementById("journal-status");

export const getProviderBackEl = () => document.getElementById("provider-back");
export const getProviderConfigPanelEl = () => document.getElementById("provider-config-panel");
export const getProviderConfigTitleEl = () => document.getElementById("provider-config-title");
export const getProviderListEl = () => document.getElementById("provider-list");
export const getProviderCatalogStatusEl = () => document.getElementById("provider-catalog-status");
export const getProviderEmptyEl = () => document.getElementById("provider-empty");
export const getProviderCodexPanelEl = () => document.getElementById("provider-codex-panel");

export const getAuthAPIKeyEl = () => document.getElementById("auth-api-key");
export const getAuthAccountIDEl = () => document.getElementById("auth-account-id");
export const getAuthRevealAPIKeyEl = () => document.getElementById("auth-reveal-api-key");

export const getProjectBackEl = () => document.getElementById("project-back");
export const getProjectAddEl = () => document.getElementById("project-add");
export const getProjectListPanelEl = () => document.getElementById("project-list-panel");
export const getProjectConfigPanelEl = () => document.getElementById("project-config-panel");
export const getProjectListEl = () => document.getElementById("project-list");
export const getProjectActionStatusEl = () => document.getElementById("project-action-status");
export const getProjectEmptyEl = () => document.getElementById("project-empty");
export const getProjectFormStatusEl = () => document.getElementById("project-form-status");

export const getOverrideLoopEl = () => document.getElementById("override-loop");

export const getDocSearchEl = () => document.getElementById("doc-search");
export const getDocCreateToggleEl = () => document.getElementById("doc-create-toggle");
export const getDocCreateCloseEl = () => document.getElementById("doc-create-close");
export const getDocCreateCancelEl = () => document.getElementById("doc-create-cancel");
export const getDocCreatePanelEl = () => document.getElementById("doc-create-panel");
export const getDocCreateProjectEl = () => document.getElementById("doc-create-project");
export const getDocCreateIssueFilterEl = () => document.getElementById("doc-create-issue-filter");
export const getDocCreateIssueEl = () => document.getElementById("doc-create-issue");
export const getDocCreateTitleEl = () => document.getElementById("doc-create-title");
export const getDocCreateContentEl = () => document.getElementById("doc-create-content");
export const getDocCreatePromptEl = () => document.getElementById("doc-create-prompt");
export const getDocCreateFileEl = () => document.getElementById("doc-create-file");
export const getDocCreateBackEl = () => document.getElementById("doc-create-back");
export const getDocCreateNextEl = () => document.getElementById("doc-create-next");
export const getDocCreateSubmitEl = () => document.getElementById("doc-create-submit");
export const getDocCreateStatusEl = () => document.getElementById("doc-create-status");
export const getDocListEl = () => document.getElementById("doc-list");

export const getDocCreateStepChips = () => ({
  1: document.getElementById("doc-create-step-chip-1"),
  2: document.getElementById("doc-create-step-chip-2"),
});

export const getDocCreateStepPanels = () => Array.from(document.querySelectorAll("[data-doc-create-step]"));
export const getDocCreateMethodButtons = () => Array.from(document.querySelectorAll("[data-doc-create-method]"));
export const getDocCreateMethodPanels = () => Array.from(document.querySelectorAll("[data-doc-create-method-panel]"));

export const getDocBreadcrumbEl = () => document.getElementById("doc-breadcrumb");
export const getDocProjectListEl = () => document.getElementById("doc-project-list");
export const getDocStatusListEl = () => document.getElementById("doc-status-list");

export const getSidebarEl = () => document.querySelector(".sidebar");
export const getSidebarOverlayEl = () => document.getElementById("sidebar-overlay");
export const getProviderDrawerOverlayEl = () => document.getElementById("provider-drawer-overlay");
export const getToastRegionEl = () => document.getElementById("toast-region");
export const getSidebarToggleButtons = () => Array.from(document.querySelectorAll(".sidebar-toggle"));
export const getPageLinks = () => Array.from(document.querySelectorAll("[data-page-link]"));

export const getPages = () => ({
  pods: document.getElementById("page-pods"),
  podView: document.getElementById("page-pod-view"),
  documents: document.getElementById("page-documents"),
  projects: document.getElementById("page-projects"),
  providers: document.getElementById("page-providers"),
  controls: document.getElementById("page-controls"),
});
