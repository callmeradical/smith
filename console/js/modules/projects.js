import { state } from "./state.js";
import { fetchJSON, postJSON, deleteJSON } from "./api.js";
import { pushToast, escapeHtml } from "./ui.js";
import { projectStorageKey } from "./constants.js";
import {
  getProjectListEl,
  getProjectEmptyEl,
  getProjectConfigPanelEl,
  getProjectAddEl,
  getProjectFormStatusEl,
  getProjectActionStatusEl,
  getProjectBackEl
} from "./elements.js";

export function loadProjectsFromStorage() {
  try {
    const raw = window.localStorage.getItem(projectStorageKey);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .map((item) => ({
        id: String(item.id || "").trim(),
        name: String(item.name || "").trim(),
        repo_url: String(item.repo_url || "").trim(),
        github_user: String(item.github_user || "").trim(),
        credential_set: Boolean(item.credential_set),
        runtime_image: String(item.runtime_image || "").trim(),
        runtime_pull_policy:
          String(item.runtime_pull_policy || "").trim() || "IfNotPresent",
        skills_image: String(item.skills_image || "").trim(),
        skills_pull_policy:
          String(item.skills_pull_policy || "").trim() || "IfNotPresent",
        updated_at: String(item.updated_at || ""),
        workflow_status: String(item.workflow_status || "new").trim() || "new",
        last_action_at: String(item.last_action_at || ""),
      }))
      .filter((item) => item.id && item.name && item.repo_url);
  } catch (_) {
    return [];
  }
}

export function persistProjects() {
  const payload = state.projects.map((project) => ({
    id: project.id,
    name: project.name,
    repo_url: project.repo_url,
    github_user: project.github_user,
    credential_set: Boolean(project.credential_set),
    runtime_image: project.runtime_image || "",
    runtime_pull_policy: project.runtime_pull_policy || "IfNotPresent",
    skills_image: project.skills_image || "",
    skills_pull_policy: project.skills_pull_policy || "IfNotPresent",
    updated_at: project.updated_at,
    workflow_status: project.workflow_status || "new",
    last_action_at: project.last_action_at || "",
  }));
  try {
    window.localStorage.setItem(projectStorageKey, JSON.stringify(payload));
  } catch (_) {}
}

export function setProjectEditorOpen(open) {
  const isOpen = Boolean(open);
  document.body.classList.toggle("project-drawer-open", isOpen);
  const panel = getProjectConfigPanelEl();
  if (panel) {
    panel.classList.toggle("open", isOpen);
    panel.setAttribute("aria-hidden", isOpen ? "false" : "true");
  }
  const addBtn = getProjectAddEl();
  if (addBtn) addBtn.classList.toggle("hidden", isOpen);
}

export function renderProjects() {
  const listEl = getProjectListEl();
  if (!listEl) return;
  listEl.innerHTML = "";
  const emptyEl = getProjectEmptyEl();
  if (emptyEl) emptyEl.classList.toggle("hidden", state.projects.length > 0);
  
  if (state.projects.length === 0) return;

  const sorted = state.projects.slice().sort((a, b) => a.name.localeCompare(b.name));
  for (const project of sorted) {
    const section = document.createElement("details");
    section.className = "project-tile";
    section.open = true;
    section.innerHTML = `
      <summary class="collapsible-summary">
        <span class="collapsible-label"><span class="collapsible-caret">&gt;</span>
        <span class="project-name">${escapeHtml(project.name)}</span></span>
      </summary>
      <div class="collapsible-body">
        <div class="project-card-actions">
          <button type="button" class="project-action-icon" data-project-action="edit" data-project-id="${escapeHtml(project.id)}">&#9998;</button>
        </div>
      </div>
    `;
    listEl.appendChild(section);
  }
}
