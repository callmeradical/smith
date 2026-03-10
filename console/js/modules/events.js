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
  getSidebarToggleButtons,
  getSidebarOverlayEl,
  getPageLinks,
  getPodViewBackEl,
  getPodViewAttachEl,
  getPodViewCancelEl,
  getPodViewTerminateEl,
  getPodViewDeleteEl,
  getPodViewCommandEl,
  getPodViewRunEl
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
      // Toggle attach logic...
    });
  }

  const runBtn = getPodViewRunEl();
  if (runBtn) {
    runBtn.addEventListener("click", () => {
      import("./terminal.js").then(m => m.runSelectedLoopCommand());
    });
  }

  const commandInput = getPodViewCommandEl();
  if (commandInput) {
    commandInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        import("./terminal.js").then(m => m.runSelectedLoopCommand());
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
}
