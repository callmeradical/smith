import { 
  setSidebarOpen, 
  setActivePage, 
  pushToast 
} from "./ui.js";
import { 
  refreshLoops, 
  setPodCreateOpen, 
  goToNextPodCreateStep, 
  setPodCreateStep,
  setPodCreateMethod,
  startPRDChat,
  startLoopFromIssue,
  loadProjectIssues,
  renderPodIssueOptions,
  renderPodIssuePreview,
  renderGrid
} from "./pods.js";
import {
  setDocCreateOpen,
  setDocCreateStep,
  setDocCreateMethod,
  submitDocCreate,
  renderDocuments
} from "./docs.js";
import {
  getPodCreateToggleEl,
  getPodCreateCloseEl,
  getPodCreateCancelEl,
  getPodCreateBackEl,
  getPodCreateNextEl,
  getPodCreateSubmitEl,
  getPodCreateMethodButtons,
  getPodCreateProjectEl,
  getPodCreateIssueEl,
  getDocCreateToggleEl,
  getDocCreateCloseEl,
  getDocCreateCancelEl,
  getDocCreateBackEl,
  getDocCreateNextEl,
  getDocCreateSubmitEl,
  getDocSearchEl,
  getDocCreateProjectEl,
  getSidebarToggleButtons,
  getSidebarOverlayEl,
  getPageLinks,
  getPodViewBackEl,
  getPodViewAttachEl,
  getPodViewCancelEl,
  getPodViewTerminateEl,
  getPodViewDeleteEl,
  getPodViewCommandEl,
  getPodViewRunEl,
  getDocViewChatInputEl,
  getDocViewChatSendEl,
  getDocViewTabs,
  getProviderListEl,
  getProviderBackEl,
  getProjectAddEl,
  getProjectBackEl
} from "./elements.js";
import { state } from "./state.js";

export function initEventListeners() {
  // Sidebar
  getSidebarToggleButtons().forEach((btn) => {
    btn.addEventListener("click", () => {
      setSidebarOpen(!document.body.classList.contains("sidebar-open"));
    });
  });
  const sidebarOverlay = getSidebarOverlayEl();
  if (sidebarOverlay) sidebarOverlay.addEventListener("click", () => setSidebarOpen(false));

  // Page Links
  getPageLinks().forEach((link) => {
    link.addEventListener("click", (ev) => {
      ev.preventDefault();
      setActivePage(link.getAttribute("data-page-link"), true);
      setSidebarOpen(false);
    });
  });

  // Pod Create
  const podCreateToggle = getPodCreateToggleEl();
  if (podCreateToggle) podCreateToggle.addEventListener("click", () => setPodCreateOpen(true));
  const podCreateClose = getPodCreateCloseEl();
  if (podCreateClose) podCreateClose.addEventListener("click", () => setPodCreateOpen(false));
  const podCreateCancel = getPodCreateCancelEl();
  if (podCreateCancel) podCreateCancel.addEventListener("click", () => setPodCreateOpen(false));
  const podCreateBack = getPodCreateBackEl();
  if (podCreateBack) podCreateBack.addEventListener("click", () => setPodCreateStep(state.podCreateStep - 1));
  const podCreateNext = getPodCreateNextEl();
  if (podCreateNext) podCreateNext.addEventListener("click", goToNextPodCreateStep);
  const podCreateSubmit = getPodCreateSubmitEl();
  if (podCreateSubmit) podCreateSubmit.addEventListener("click", startLoopFromIssue);

  getPodCreateMethodButtons().forEach(btn => {
    btn.addEventListener("click", () => {
      setPodCreateMethod(btn.dataset.podCreateMethod);
    });
  });

  const podCreateProject = getPodCreateProjectEl();
  if (podCreateProject) {
    podCreateProject.addEventListener("change", () => {
      loadProjectIssues(true);
    });
  }

  const podCreateIssue = getPodCreateIssueEl();
  if (podCreateIssue) {
    podCreateIssue.addEventListener("change", () => {
      renderPodIssuePreview();
    });
  }

  // Doc Create
  const docCreateToggle = getDocCreateToggleEl();
  if (docCreateToggle) docCreateToggle.addEventListener("click", () => setDocCreateOpen(true));
  const docCreateClose = getDocCreateCloseEl();
  if (docCreateClose) docCreateClose.addEventListener("click", () => setDocCreateOpen(false));
  const docCreateCancel = getDocCreateCancelEl();
  if (docCreateCancel) docCreateCancel.addEventListener("click", () => setDocCreateOpen(false));
  const docCreateBack = getDocCreateBackEl();
  if (docCreateBack) docCreateBack.addEventListener("click", () => setDocCreateStep(state.docCreateStep - 1));
  const docCreateNext = getDocCreateNextEl();
  if (docCreateNext) docCreateNext.addEventListener("click", () => setDocCreateStep(state.docCreateStep + 1));
  const docCreateSubmit = getDocCreateSubmitEl();
  if (docCreateSubmit) docCreateSubmit.addEventListener("click", submitDocCreate);

  const docSearch = getDocSearchEl();
  if (docSearch) {
    docSearch.addEventListener("input", () => {
      state.docSearchQuery = docSearch.value;
      renderDocuments();
    });
  }

  document.body.addEventListener("click", (e) => {
    const projectItem = e.target.closest("[data-doc-filter-project]");
    if (projectItem) {
      state.docFilterProject = projectItem.dataset.docFilterProject;
      renderDocuments();
    }
    const statusItem = e.target.closest("[data-doc-filter-status]");
    if (statusItem) {
      state.docFilterStatus = statusItem.dataset.docFilterStatus;
      renderDocuments();
    }
  });

  const docCreateProject = getDocCreateProjectEl();
  if (docCreateProject) {
    docCreateProject.addEventListener("change", () => {
      import("./docs.js").then(m => m.loadDocProjectIssues());
    });
  }

  const podViewBack = getPodViewBackEl();
  if (podViewBack) {
    podViewBack.addEventListener("click", () => {
      setActivePage("pods", true);
    });
  }

  // Pod View Actions
  const attachBtn = getPodViewAttachEl();
  if (attachBtn) {
    attachBtn.addEventListener("click", () => {
      import("./terminal.js").then((m) => {
        const selected = state.selectedLoop;
        if (!selected) return;
        if (state.terminalAttachedLoopID === selected) {
          m.setTerminalUIState("detaching", "");
          // Actual detach logic would go here
          state.terminalAttachedLoopID = "";
          m.setTerminalUIState("idle", "");
        } else {
          m.setTerminalUIState("attaching", "");
          // Actual attach logic would go here
          state.terminalAttachedLoopID = selected;
          m.setTerminalUIState("idle", "");
        }
      });
    });
  }

  const cancelBtn = getPodViewCancelEl();
  if (cancelBtn) {
    cancelBtn.addEventListener("click", () => {
      import("./terminal.js").then((m) => m.cancelSelectedLoop());
    });
  }

  const terminateBtn = getPodViewTerminateEl();
  if (terminateBtn) {
    terminateBtn.addEventListener("click", () => {
      import("./terminal.js").then((m) => m.terminateSelectedLoop());
    });
  }

  const deleteBtn = getPodViewDeleteEl();
  if (deleteBtn) {
    deleteBtn.addEventListener("click", () => {
      import("./terminal.js").then((m) => m.deleteSelectedLoop());
    });
  }

  const runBtn = getPodViewRunEl();
  if (runBtn) {
    runBtn.addEventListener("click", () => {
      import("./terminal.js").then(m => m.runSelectedLoopCommand());
    });
  }

  const commandEl = getPodViewCommandEl();
  if (commandEl) {
    commandEl.addEventListener("input", () => {
      import("./terminal.js").then((m) => m.syncPodViewActions());
    });
    commandEl.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        import("./terminal.js").then((m) => m.runSelectedLoopCommand());
      }
    });
  }


  // Doc View Actions
  getDocViewTabs().forEach(btn => {
    btn.addEventListener("click", () => {
      import("./docs.js").then(m => m.setDocViewTab(btn.dataset.docTab));
    });
  });

  const docSendBtn = getDocViewChatSendEl();
  if (docSendBtn) {
    docSendBtn.addEventListener("click", () => {
      import("./docs.js").then(m => m.sendDocChatMessage());
    });
  }

  const docChatInput = getDocViewChatInputEl();
  if (docChatInput) {
    docChatInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        import("./docs.js").then(m => m.sendDocChatMessage());
      }
    });
  }

  // Regressions for Pods page
  const stateFilter = document.getElementById("state-filter");
  if (stateFilter) stateFilter.addEventListener("change", renderGrid);
  const searchInput = document.getElementById("search");
  if (searchInput) searchInput.addEventListener("input", renderGrid);
  const refreshBtn = document.getElementById("refresh");
  if (refreshBtn) refreshBtn.addEventListener("click", () => refreshLoops());

  const overrideApply = document.getElementById("override-apply");
  if (overrideApply) {
    overrideApply.addEventListener("click", () => {
      import("./controls.js").then((m) => m.applyOverride());
    });
  }

  // Provider List Delegation
  const providerList = getProviderListEl();
  if (providerList) {
    providerList.addEventListener("click", (e) => {
      const btn = e.target.closest("[data-provider-config]");
      if (btn) {
        import("./providers.js").then((m) => m.configureProvider(btn.dataset.providerConfig));
      }
    });
  }

  // Provider Drawer Close
  const providerBack = getProviderBackEl();
  if (providerBack) {
    providerBack.addEventListener("click", () => {
      import("./providers.js").then((m) => m.setProviderConfigOpen(false));
    });
  }

  // Provider Auth Actions
  const authSaveBtn = document.getElementById("auth-save-api-key");
  if (authSaveBtn) {
    authSaveBtn.addEventListener("click", () => {
      import("./providers.js").then((m) => m.saveAPIKey());
    });
  }

  const authDisconnectBtn = document.getElementById("auth-disconnect");
  if (authDisconnectBtn) {
    authDisconnectBtn.addEventListener("click", () => {
      import("./providers.js").then((m) => m.disconnectProvider());
    });
  }

  // Project Drawer
  const projectAdd = getProjectAddEl();
  if (projectAdd) {
    projectAdd.addEventListener("click", () => {
      import("./projects.js").then((m) => m.openAddProjectForm());
    });
  }

  const projectBack = getProjectBackEl();
  if (projectBack) {
    projectBack.addEventListener("click", () => {
      import("./projects.js").then((m) => m.setProjectEditorOpen(false));
    });
  }

  const projectSaveBtn = document.getElementById("project-save");
  if (projectSaveBtn) {
    projectSaveBtn.addEventListener("click", () => {
      import("./projects.js").then((m) => m.saveProject());
    });
  }
}
