import { state } from "./state.js";
import { postJSON, fetchJSON, deleteJSON, requestJSON } from "./api.js";
import { pushToast, escapeHtml, setActivePage } from "./ui.js";
import {
  getDocCreateStepChips,
  getDocCreateStepPanels,
  getDocCreateMethodButtons,
  getDocCreateMethodPanels,
  getDocCreateBackEl,
  getDocCreateNextEl,
  getDocCreateSubmitEl,
  getDocCreateStatusEl,
  getDocListEl,
  getDocCreateProjectEl,
  getDocCreateIssueEl,
  getDocCreateIssueFilterEl,
  getDocCreateTitleEl,
  getDocCreateContentEl,
  getDocCreatePromptEl,
  getDocCreateFileEl,
  getDocCreatePanelEl,
  getDocBreadcrumbEl,
  getDocProjectListEl,
  getDocStatusListEl
} from "./elements.js";

export function setDocCreateMethod(method) {
  const next = String(method || "").trim().toLowerCase();
  if (next === "issue" || next === "generate" || next === "upload") {
    state.docCreateMethod = next;
  } else {
    state.docCreateMethod = "issue";
  }
  syncDocCreateWorkflow();
}

export function setDocCreateStep(step) {
  state.docCreateStep = Math.max(1, Math.min(2, Number(step || 1)));
  syncDocCreateWorkflow();
}

export function syncDocCreateWorkflow() {
  const step = state.docCreateStep;
  const method = state.docCreateMethod;

  const panels = getDocCreateStepPanels();
  if (!panels.length) return;

  panels.forEach(panel => {
    const panelStep = Number(panel.dataset.docCreateStep || 0);
    panel.classList.toggle("hidden", panelStep !== step);
  });
  
  const chips = getDocCreateStepChips();
  Object.entries(chips).forEach(([rawStep, chip]) => {
    if (!chip) return;
    const chipStep = Number(rawStep);
    chip.classList.toggle("active", chipStep === step);
    chip.classList.toggle("complete", chipStep < step);
  });
  
  getDocCreateMethodButtons().forEach(button => {
    const btnMethod = button.getAttribute("data-doc-create-method");
    button.classList.toggle("selected", btnMethod === method);
  });
  
  getDocCreateMethodPanels().forEach(panel => {
    const panelMethod = panel.dataset.docCreateMethodPanel;
    panel.classList.toggle("hidden", panelMethod !== method || step !== 2);
  });
  
  const back = getDocCreateBackEl();
  if (back) back.classList.toggle("hidden", step <= 1);
  const isAtFinalStep = step === 2;
  const next = getDocCreateNextEl();
  if (next) next.classList.toggle("hidden", isAtFinalStep);
  const submit = getDocCreateSubmitEl();
  if (submit) submit.classList.toggle("hidden", !isAtFinalStep);
}

export function setDocCreateStatus(message, level) {
  const el = getDocCreateStatusEl();
  if (!el) return;
  el.textContent = message;
  el.style.color =
    level === "ok" ? "var(--ok)" : level === "err" ? "var(--err)" : "var(--muted)";
}

export function setDocCreateOpen(open) {
  const isOpen = Boolean(open);
  document.body.classList.toggle("pod-drawer-open", isOpen);
  const panel = getDocCreatePanelEl();
  if (panel) {
    panel.classList.toggle("open", isOpen);
    panel.setAttribute("aria-hidden", isOpen ? "false" : "true");
  }
  if (isOpen) {
    setDocCreateStep(1);
    setDocCreateMethod("issue");
    setDocCreateStatus("", "muted");
  }
}

export async function submitDocCreate() {
  const projectID = getDocCreateProjectEl()?.value;
  if (!projectID) {
    setDocCreateStatus("Select a project first.", "err");
    return;
  }
  const method = state.docCreateMethod;
  let title = getDocCreateTitleEl()?.value.trim() || "";
  let content = "";
  let sourceRef = "";
  let sourceType = "direct";

  if (method === "issue") {
    const issueID = getDocCreateIssueEl()?.value;
    if (!issueID) {
      setDocCreateStatus("Select a GitHub issue first.", "err");
      return;
    }
    const issues = state.podProjectIssues[projectID] || [];
    const issue = issues.find(i => String(i.id) === issueID || String(i.number) === issueID);
    if (!issue) {
      setDocCreateStatus("Issue not found.", "err");
      return;
    }
    content = issue.body || "";
    title = title || issue.title || "Issue #" + issue.number;
    sourceType = "github_issue";
    sourceRef = projectID + "#" + issue.number;
  } else if (method === "generate") {
    const prompt = getDocCreatePromptEl()?.value.trim();
    if (!prompt) {
      setDocCreateStatus("Provide a prompt for generation.", "err");
      return;
    }
    content = prompt;
    title = title || "Generated PRD";
    sourceType = "interactive_prompt";
  } else if (method === "upload") {
    content = getDocCreateContentEl()?.value.trim();
    if (!content) {
      setDocCreateStatus("Provide document content.", "err");
      return;
    }
    title = title || "Uploaded Document";
    sourceType = "upload";
  }

  state.docCreateBusy = true;
  setDocCreateStatus("Creating document...", "muted");
  try {
    const payload = {
      project_id: projectID,
      title: title,
      content: content,
      format: "markdown",
      source_type: sourceType,
      source_ref: sourceRef,
      status: "active",
      metadata: {}
    };
    await postJSON("/v1/documents", payload);
    setDocCreateStatus("Document created.", "ok");
    pushToast("Document created successfully.", "ok");
    setDocCreateOpen(false);
    void refreshDocuments();
  } catch (err) {
    setDocCreateStatus(err.message || "Failed to create document", "err");
  } finally {
    state.docCreateBusy = false;
  }
}

export async function refreshDocuments() {
  try {
    const raw = await fetchJSON("/v1/documents");
    state.documents = Array.isArray(raw) ? raw : [];
    renderDocuments();
  } catch (err) {
    console.error("Error fetching documents:", err);
  }
}

export async function loadDocProjectIssues() {
  const el = getDocCreateProjectEl();
  const projectID = el?.value;
  if (!projectID) {
    const issueEl = getDocCreateIssueEl();
    if (issueEl) {
      issueEl.innerHTML = '<option value="">Select project</option>';
      issueEl.disabled = true;
    }
    return;
  }
  setDocCreateStatus("Loading issues...", "muted");
  try {
    const issues = await fetchJSON("/v1/projects/" + projectID + "/issues");
    state.podProjectIssues[projectID] = Array.isArray(issues) ? issues : [];
    renderDocIssueOptions(projectID);
    setDocCreateStatus("Issues loaded.", "ok");
  } catch (err) {
    setDocCreateStatus("Error loading issues: " + err.message, "err");
  }
}

export function renderDocIssueOptions(projectID) {
  const el = getDocCreateIssueEl();
  if (!el) return;
  const issues = state.podProjectIssues[projectID] || [];
  const filter = (getDocCreateIssueFilterEl()?.value || "").toLowerCase();
  const filtered = issues.filter(i => 
    String(i.number).includes(filter) || 
    (i.title || "").toLowerCase().includes(filter)
  );
  el.innerHTML = '<option value="">Select issue (' + filtered.length + ')</option>';
  filtered.forEach(i => {
    const opt = document.createElement("option");
    opt.value = String(i.number);
    opt.textContent = "#" + i.number + " " + i.title;
    el.appendChild(opt);
  });
  el.disabled = filtered.length === 0;
}

let lastFilteredDocsJson = "";

export function renderDocuments() {
  const docListEl = getDocListEl();
  if (!docListEl) return;

  const query = (state.docSearchQuery || "").toLowerCase();
  const docs = Array.isArray(state.documents) ? state.documents : [];

  const filtered = docs.filter(d => {
    if (!d) return false;
    const matchesQuery = (d.title || "").toLowerCase().includes(query) || 
                        (d.id || "").toLowerCase().includes(query) ||
                        (d.project_id || "").toLowerCase().includes(query);
    const matchesProject = state.docFilterProject === "all" || d.project_id === state.docFilterProject;
    const matchesStatus = state.docFilterStatus === "all" || d.status === state.docFilterStatus;
    return matchesQuery && matchesProject && matchesStatus;
  });

  const currentJson = JSON.stringify(filtered.map(d => d.id + d.updated_at + d.status));
  if (currentJson === lastFilteredDocsJson) {
    updateDocumentMeta(docs);
    return;
  }
  lastFilteredDocsJson = currentJson;

  updateDocumentMeta(docs);

  docListEl.innerHTML = "";
  if (state.docFilterProject === "") {
    // Show nothing if nothing selected
    return;
  }

  if (filtered.length === 0) {
    docListEl.innerHTML = '<div class="status-note">No documents found.</div>';
    return;
  }

  if (state.docFilterProject === "all") {
    const grouped = {};
    filtered.forEach(doc => {
      if (!doc.project_id) return;
      if (!grouped[doc.project_id]) grouped[doc.project_id] = [];
      grouped[doc.project_id].push(doc);
    });

    Object.keys(grouped).sort().forEach(projectID => {
      const section = document.createElement("section");
      section.className = "pod-group";
      section.style.width = "100%";
      section.style.gridColumn = "1 / -1";
      
      const header = document.createElement("div");
      header.className = "doc-sidebar-header";
      header.style.marginBottom = "12px";
      header.textContent = "Project: " + projectID;
      section.appendChild(header);

      const grid = document.createElement("div");
      grid.className = "pod-grid";
      grouped[projectID].forEach(doc => renderDocTile(doc, grid));
      
      section.appendChild(grid);
      docListEl.appendChild(section);
    });
  } else {
    filtered.forEach(doc => renderDocTile(doc, docListEl));
  }
}

function updateDocumentMeta(docs) {
  const projectListEl = getDocProjectListEl();
  if (projectListEl) {
    const projectIDs = docs.map(d => d.project_id).filter(pid => !!pid);
    const uniqueProjects = Array.from(new Set(projectIDs)).sort();
    
    projectListEl.innerHTML = '<div class="doc-sidebar-item' + (state.docFilterProject === "all" ? " active" : "") + '" data-doc-filter-project="all">All Projects</div>';
    
    if (uniqueProjects.length === 0) {
      projectListEl.innerHTML += '<div class="doc-sidebar-item muted">📁 (Empty)</div>';
    } else {
      uniqueProjects.forEach(p => {
        const item = document.createElement("div");
        item.className = "doc-sidebar-item" + (state.docFilterProject === p ? " active" : "");
        item.dataset.docFilterProject = p;
        item.textContent = p;
        projectListEl.appendChild(item);
      });
    }
  }

  const statusListEl = getDocStatusListEl();
  if (statusListEl) {
    statusListEl.querySelectorAll(".doc-sidebar-item").forEach(item => {
      if (item instanceof HTMLElement) {
        item.classList.toggle("active", item.dataset.docFilterStatus === state.docFilterStatus);
      }
    });
  }

  const breadcrumbEl = getDocBreadcrumbEl();
  if (breadcrumbEl) {
    let projectLabel = state.docFilterProject === "all" ? "All Projects" : state.docFilterProject;
    if (!projectLabel && state.docFilterProject === "") projectLabel = "None";
    const statusLabel = state.docFilterStatus === "all" ? "" : " / " + state.docFilterStatus.charAt(0).toUpperCase() + state.docFilterStatus.slice(1);
    breadcrumbEl.innerHTML = `<span>${escapeHtml(projectLabel || "Select Project")}</span>${escapeHtml(statusLabel)}`;
  }
}

function renderDocTile(doc, container) {
  const tile = document.createElement("div");
  tile.className = "pod-tile";
  if (doc.status === "archived") tile.style.opacity = "0.6";
  
  const updatedAtLabel = doc.updated_at ? new Date(doc.updated_at).toLocaleString() : "unknown date";

  tile.innerHTML = `
    <div class="tile-head">
      <div class="tile-title loop-id">${escapeHtml(doc.title || "Untitled")}</div>
      <div class="badge ${doc.status === "active" ? "state-synced" : "state-cancelled"}">${escapeHtml(doc.status || "active")}</div>
    </div>
    <div class="tile-meta">
      <span class="muted">${escapeHtml(doc.source_type || "unknown")}: ${escapeHtml(doc.source_ref || "direct")}</span>
      <span class="muted">${escapeHtml(updatedAtLabel)}</span>
    </div>
    <div class="tile-footer" style="margin-top: 12px; display: flex; gap: 8px; justify-content: flex-start;">
      <button class="tile-action-button doc-edit" data-doc-id="${escapeHtml(doc.id)}">Edit</button>
      <button class="tile-action-button doc-build primary" data-doc-id="${escapeHtml(doc.id)}">Build</button>
      <button class="tile-action-button doc-archive" data-doc-id="${escapeHtml(doc.id)}">${doc.status === "active" ? "Archive" : "Unarchive"}</button>
      <button class="tile-action-button doc-delete danger" data-doc-id="${escapeHtml(doc.id)}">Delete</button>
    </div>
  `;
  
  tile.querySelector(".doc-edit").onclick = (e) => { e.stopPropagation(); editDocument(doc.id); };
  tile.querySelector(".doc-build").onclick = (e) => { e.stopPropagation(); buildDocument(doc.id); };
  tile.querySelector(".doc-archive").onclick = (e) => { e.stopPropagation(); archiveDocument(doc.id); };
  tile.querySelector(".doc-delete").onclick = (e) => { e.stopPropagation(); deleteDocument(doc.id); };
  
  container.appendChild(tile);
}

export async function archiveDocument(id) {
  const doc = state.documents.find(d => d.id === id);
  if (!doc) return;
  const nextStatus = doc.status === "active" ? "archived" : "active";
  try {
    await requestJSON("/v1/documents/" + id, "PUT", { status: nextStatus });
    void refreshDocuments();
  } catch (err) {
    pushToast("Error archiving document: " + err.message, "err");
  }
}

export async function deleteDocument(id) {
  if (!confirm("Delete this document?")) return;
  try {
    await deleteJSON("/v1/documents/" + id);
    void refreshDocuments();
  } catch (err) {
    pushToast("Error deleting document: " + err.message, "err");
  }
}

export async function buildDocument(id) {
  try {
    await postJSON("/v1/documents/" + id + "/build", {});
    pushToast("Build loop instantiated for document: " + id, "ok");
    setActivePage("pods", true);
  } catch (err) {
    pushToast("Error building document: " + err.message, "err");
  }
}

export function editDocument(id) {
  const doc = state.documents.find(d => d.id === id);
  if (!doc) return;
  const nextContent = prompt("Edit Document Content", doc.content);
  if (nextContent === null) return;
  void requestJSON("/v1/documents/" + id, "PUT", { content: nextContent }).then(() => refreshDocuments());
}
