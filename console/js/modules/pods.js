import { state, requestTimeoutMs } from "./state.js";
import { postJSON, fetchJSON, getJSON, deleteJSON, fetchWithTimeout } from "./api.js";
import { pushToast, escapeHtml, setActivePage, syncRightDrawerOverlay } from "./ui.js";
import { podViewTerminalStates } from "./constants.js";
import {
  getGridEl,
  getPodCreateStepChips,
  getPodCreateStepPanels,
  getPodCreateMethodButtons,
  getPodCreateMethodPanels,
  getPodCreateProjectEl,
  getPodCreateIssueFilterEl,
  getPodCreateIssueEl,
  getPodCreateIssuePreviewEl,
  getPodCreateBranchEl,
  getPodCreateSourceBranchEl,
  getPodCreateProviderEl,
  getPodCreatePromptEl,
  getPodCreatePRDFileEl,
  getPodCreateBackEl,
  getPodCreateNextEl,
  getPodCreateSubmitEl,
  getPodCreateStatusEl,
  getPodCreateChatPanelEl,
  getPodCreateChatInputEl,
  getPodCreateChatStatusEl,
  getPodCreatePanelEl,
  getApiDotEl
} from "./elements.js";

export function normalizeLoop(item) {
  const record = item.record || item.Record || {};
  const loopID = record.loop_id || record.LoopID || item.loop_id || "unknown-loop";
  const status = (record.state || record.State || "unknown").toLowerCase();
  const attempt = Number(record.attempt || record.Attempt || 0);
  const reason = record.reason || record.Reason || "";
  const revision = Number(item.revision || item.Revision || record.observed_revision || 0);
  return {
    loopID,
    project: record.project_id || record.project || record.project_name || "default",
    status,
    attempt,
    reason,
    revision,
  };
}

export function renderGrid() {
  const gridEl = getGridEl();
  if (!gridEl) return;
  
  const stateFilter = document.getElementById("state-filter")?.value || "all";
  const searchQuery = document.getElementById("search")?.value.trim().toLowerCase() || "";

  const filtered = state.loops.filter(loop => {
    const matchesState =
      stateFilter === "all" ||
      (stateFilter === "active" && (loop.status === "unresolved" || loop.status === "overwriting")) ||
      loop.status === stateFilter;
    const matchesSearch = !searchQuery || String(loop.loopID).toLowerCase().includes(searchQuery);
    return matchesState && matchesSearch;
  });

  gridEl.innerHTML = "";
  if (filtered.length === 0) {
    gridEl.innerHTML = '<div class="empty">No pods found matching filters.</div>';
    return;
  }

  filtered.forEach(loop => {
    const tile = document.createElement("article");
    tile.className = "pod-tile" + (state.selectedLoop === loop.loopID ? " selected" : "");
    tile.innerHTML = `
      <div class="tile-head"><div class="tile-title loop-id">${escapeHtml(loop.loopID)}</div></div>
      <div class="tile-loop">${escapeHtml(loop.project)}</div>
      <div class="tile-reason">${escapeHtml(loop.reason || "no recent update")}</div>
      <div class="tile-footer">
        <span class="badge state-${escapeHtml(loop.status)}">${escapeHtml(loop.status)}</span>
        <div class="tile-meta"><span>ATT ${loop.attempt}</span><span>REV ${loop.revision}</span></div>
      </div>
    `;
    tile.onclick = () => {
      state.selectedLoop = loop.loopID;
      setActivePage("podView", true);
    };
    gridEl.appendChild(tile);
  });
  
  renderStats();
}

export function renderStats() {
  const total = state.loops.length;
  const active = state.loops.filter(l => l.status === "unresolved" || l.status === "overwriting").length;
  const flatline = state.loops.filter(l => l.status === "flatline").length;

  const totalEl = document.getElementById("stat-total");
  const activeEl = document.getElementById("stat-active");
  const flatlineEl = document.getElementById("stat-flatline");
  const refreshEl = document.getElementById("stat-refresh");

  if (totalEl) totalEl.textContent = String(total);
  if (activeEl) activeEl.textContent = String(active);
  if (flatlineEl) flatlineEl.textContent = String(flatline);
  if (refreshEl) refreshEl.textContent = new Date().toLocaleTimeString();
}

export function selectLoop(loopID) {
  state.selectedLoop = loopID;
  renderGrid();
  renderSelectedLoop();
  setActivePage("podView", true);
}

export function renderSelectedLoop() {
  const titleEl = getPodViewTitleEl();
  if (titleEl) titleEl.textContent = state.selectedLoop ? "Pod: " + state.selectedLoop : "Pod Detail";
  
  import("./journal.js").then(m => {
    m.clearJournal("attaching to stream...");
    if (state.selectedLoop) {
      m.attachStream(state.selectedLoop);
    }
  });
  
  if (state.selectedLoop) {
    void loadLoopRuntime(state.selectedLoop, true);
  }
}

export async function loadLoopRuntime(loopID, silent) {
  if (!loopID) return;
  state.runtimeBusyLoopID = loopID;
  try {
    const res = await getJSON(`/v1/loops/${encodeURIComponent(loopID)}/runtime`);
    state.runtimeByLoopID[loopID] = res;
  } catch (err) {
    if (!silent) pushToast("Error loading runtime: " + err.message, "err");
  } finally {
    state.runtimeBusyLoopID = "";
    import("./terminal.js").then(m => m.syncPodViewActions());
  }
}

export function setPodCreateOpen(open) {
  const isOpen = Boolean(open);
  document.body.classList.toggle("pod-drawer-open", isOpen);
  const panel = getPodCreatePanelEl();
  if (panel) {
    panel.classList.toggle("open", isOpen);
    panel.setAttribute("aria-hidden", isOpen ? "false" : "true");
  }
  syncRightDrawerOverlay();
  if (isOpen) {
    renderPodProjectOptions();
    resetPodCreateWorkflow();
    void loadProjectIssues(false);
  } else {
    setPodCreateStatus("", "muted");
  }
}

export function resetPodCreateWorkflow() {
  state.podCreateMethod = "issue";
  state.podCreateStep = 1;
  state.podCreateFinalPRD = null;
  disconnectPodCreateChat();
  const els = [
    getPodCreateIssueFilterEl(),
    getPodCreateIssueEl(),
    getPodCreateLoopNameEl(),
    getPodCreateBranchEl(),
    getPodCreateSourceBranchEl(),
    getPodCreateProviderEl(),
    getPodCreatePromptEl(),
    getPodCreatePRDFileEl()
  ];
  els.forEach(el => { if (el) el.value = ""; });
  const preview = getPodCreateIssuePreviewEl();
  if (preview) preview.textContent = "Select a project to load issues.";
  setPodCreateStatus("", "muted");
  syncPodCreateWorkflow();
}

export function setPodCreateMethod(method) {
  state.podCreateMethod = method;
  syncPodCreateWorkflow();
}

export function setPodCreateStep(step) {
  const isInteractive = state.podCreateMethod === "issue" || state.podCreateMethod === "generate_prd";
  const maxStep = isInteractive ? 4 : 3;
  state.podCreateStep = Math.max(1, Math.min(maxStep, Number(step || 1)));
  syncPodCreateWorkflow();
}

export function syncPodCreateWorkflow() {
  const isInteractive = state.podCreateMethod === "issue" || state.podCreateMethod === "generate_prd";
  const maxStep = isInteractive ? 4 : 3;
  const step = Math.max(1, Math.min(maxStep, state.podCreateStep));
  state.podCreateStep = step;

  const chips = getPodCreateStepChips();
  if (chips[4]) chips[4].classList.toggle("hidden", !isInteractive);

  getPodCreateStepPanels().forEach(panel => {
    const panelStep = Number(panel.getAttribute("data-pod-create-step"));
    panel.classList.toggle("hidden", panelStep !== step);
  });

  Object.entries(chips).forEach(([s, chip]) => {
    if (!chip) return;
    const chipStep = Number(s);
    chip.classList.toggle("active", chipStep === step);
    chip.classList.toggle("complete", chipStep < step);
  });

  getPodCreateMethodButtons().forEach(button => {
    const method = button.getAttribute("data-pod-create-method");
    button.classList.toggle("selected", method === state.podCreateMethod);
  });
  
  getPodCreateMethodPanels().forEach(panel => {
    const panelMethod = panel.getAttribute("data-pod-create-method-panel");
    if (step === 2 && panelMethod === state.podCreateMethod) {
      panel.classList.remove("hidden");
    } else {
      panel.classList.add("hidden");
    }
  });

  const back = getPodCreateBackEl();
  if (back) back.classList.toggle("hidden", step <= 1);
  const isAtFinalStep = step === maxStep;
  const next = getPodCreateNextEl();
  if (next) next.classList.toggle("hidden", isAtFinalStep);
  const submit = getPodCreateSubmitEl();
  if (submit) {
    submit.classList.toggle("hidden", !isAtFinalStep);
    if (isAtFinalStep) {
      if (isInteractive) {
        submit.disabled = !state.podCreateFinalPRD;
      } else {
        submit.disabled = false;
      }
    }
  }

  if (step === 3) {
    preparePodCreateDetails();
  }
  if (step === 4 && isInteractive) {
    if (!state.podCreateChatSocket) {
      startPRDChat();
    }
  } else {
    disconnectPodCreateChat();
  }

  setPodCreateBusy(state.podCreateBusy);
}

export function goToNextPodCreateStep() {
  const error = validatePodCreateStep(state.podCreateStep);
  if (error) {
    setPodCreateStatus(error, "err");
    return;
  }
  setPodCreateStatus("", "muted");
  setPodCreateStep(state.podCreateStep + 1);
}

export function validatePodCreateStep(step) {
  const method = state.podCreateMethod;
  if (step === 1) return "";
  if (step === 2) {
    if (!selectedPodProject()) return "Select a project first.";
    return "";
  }
  if (step === 3) {
    if (!selectedPodProject()) return "Select a project first.";
    const promptEl = getPodCreatePromptEl();
    const prompt = String(promptEl?.value || "").trim();
    const hasIssue = Boolean(selectedPodIssue());
    const hasProvidedPRD = Boolean(parsePromptAsPRDJSON(prompt));
    if (method === "load_prd" && !hasProvidedPRD) {
      return "Provide PRD JSON (paste or upload) before creating a loop.";
    }
    if (!hasIssue && !prompt) {
      return "Select an issue or provide a prompt.";
    }
    const configuredProviders = configuredLoopProviders();
    if (configuredProviders.length === 0) {
      return "Configure at least one provider before starting a loop.";
    }
    const providerEl = getPodCreateProviderEl();
    const provider = String(providerEl?.value || "").trim().toLowerCase();
    if (!configuredProviders.some((item) => item.id === provider)) {
      return "Select a configured provider.";
    }
  }
  return "";
}

export function startPRDChat() {
  const project = selectedPodProject();
  if (!project) return;
  const promptEl = getPodCreatePromptEl();
  const prompt = promptEl?.value.trim() || "";
  const issue = selectedPodIssue();

  let initialMsg = prompt;
  if (!initialMsg && issue) {
    initialMsg = `Build a PRD from GitHub issue #${issue.number}: ${issue.title}\n\n${issue.body || ""}`;
  }

  if (!initialMsg) initialMsg = "Let's build a new PRD.";

  const statusEl = getPodCreateChatStatusEl();
  if (statusEl) {
    statusEl.innerHTML = '<span class="spinner"></span> Working on PRD...';
    statusEl.className = "status-note muted";
  }

  connectPodCreateChat(initialMsg);
}

export function connectPodCreateChat(initialMessage) {
  disconnectPodCreateChat();
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = window.location.host;
  const url = `${protocol}//${host}/v1/chat/prd`;

  const socket = new WebSocket(url);
  state.podCreateChatSocket = socket;
  state.podCreateChatMessages = [];
  state.podCreateFinalPRD = null;
  renderPodCreateChat();

  socket.onopen = () => {
    if (initialMessage) {
      socket.send(JSON.stringify({ type: "user", text: initialMessage }));
    }
  };

  socket.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      const statusEl = getPodCreateChatStatusEl();
      if (msg.type === "system" && msg.final_prd_path) {
        state.podCreateFinalPRD = msg.text;
        if (statusEl) {
          statusEl.textContent = "PRD finalized! Click 'create loop' to start.";
          statusEl.className = "status-note ok";
        }
        const submit = getPodCreateSubmitEl();
        if (submit) submit.disabled = false;
      } else if (msg.type === "error") {
        if (statusEl) {
          statusEl.textContent = "Error: " + (msg.error || "unknown");
          statusEl.className = "status-note err";
        }
      }
      state.podCreateChatMessages.push(msg);
      renderPodCreateChat();
    } catch (err) {
      console.error("Failed to parse pod create chat message", err);
    }
  };

  socket.onclose = () => {
    state.podCreateChatSocket = null;
  };
}

export function disconnectPodCreateChat() {
  if (state.podCreateChatSocket) {
    state.podCreateChatSocket.close();
    state.podCreateChatSocket = null;
  }
}

export function renderPodCreateChat() {
  const panel = getPodCreateChatPanelEl();
  if (!panel) return;
  panel.innerHTML = "";
  state.podCreateChatMessages.forEach((msg) => {
    if (msg.type === "system" && !msg.text) return;
    const bubble = document.createElement("div");
    bubble.className = "chat-bubble " + (msg.type === "user" ? "user" : "agent");
    if (msg.type === "system") bubble.className = "chat-bubble agent system-msg";
    const content = document.createElement("div");
    content.textContent = msg.text;
    bubble.appendChild(content);
    panel.appendChild(bubble);
  });
  panel.scrollTop = panel.scrollHeight;
}

export async function startLoopFromIssue() {
  const project = selectedPodProject();
  if (!project) return;
  const issue = selectedPodIssue();
  const repository = parseGitHubRepository(project.repo_url);
  if (!repository) return;
  
  const promptEl = getPodCreatePromptEl();
  const prompt = String(promptEl?.value || "").trim();
  const providedPRD = parsePromptAsPRDJSON(prompt);
  const hasProvidedPRD = Boolean(providedPRD);
  const providerEl = getPodCreateProviderEl();
  const provider = String(providerEl?.value || "").trim().toLowerCase();
  
  const loopNameEl = getPodCreateLoopNameEl();
  const loopName = loopNameEl?.value.trim() || (issue ? defaultPodLoopName(issue) : "prompt-" + Date.now());
  const loopNameSlug = slugifySegment(loopName);
  
  const branchEl = getPodCreateBranchEl();
  const branch = normalizeBranchName(branchEl?.value || "") || defaultPodBranchName(project, issue) || loopNameSlug;
  
  const sourceBranchEl = getPodCreateSourceBranchEl();
  const sourceBranch = normalizeBranchName(sourceBranchEl?.value || "") || "main";
  const loopID = slugifySegment(project.name || project.id || "project") + "-" + loopNameSlug;
  
  const sourceType = state.podCreateFinalPRD ? "prompt" : hasProvidedPRD ? "prompt" : issue ? "github_issue" : "prompt";
  const sourceRef = state.podCreateFinalPRD ? "prompt:console-chat" : issue ? repository + "#" + issue.number : "prompt:" + loopNameSlug;

  setPodCreateBusy(true);
  setPodCreateStatus("Creating loop...", "muted");
  try {
    const payload = {
      loop_id: loopID,
      title: issue ? "Loop " + loopName + ": " + issue.title : "Loop " + loopName + ": prompt request",
      provider_id: provider,
      source_type: sourceType,
      source_ref: sourceRef,
      metadata: {
        project_id: project.id,
        project_name: project.name,
        workspace_branch: branch,
        workspace_source_branch: sourceBranch,
        workspace_prd_json: state.podCreateFinalPRD || (hasProvidedPRD ? JSON.stringify(providedPRD) : ""),
      }
    };
    await postJSON("/v1/loops", payload);
    setPodCreateOpen(false);
    pushToast("Loop started successfully.", "ok");
    void refreshLoops();
  } catch (err) {
    setPodCreateStatus(err.message || "failed to create loop", "err");
  } finally {
    setPodCreateBusy(false);
  }
}
export async function refreshLoops() {
  console.log('PODS: Refreshing loops...');
  try {
    const raw = await fetchJSON("/v1/loops");
    console.log('PODS: Received loops:', raw?.length);
    state.loops = Array.isArray(raw) ? raw.map(normalizeLoop) : [];
    renderGrid();

    const dot = getApiDotEl();
    if (dot) dot.className = "dot ok";
  } catch (_) {
    const dot = getApiDotEl();
    if (dot) dot.className = "dot err";
  }
}

export function setPodCreateBusy(busy) {
  state.podCreateBusy = Boolean(busy);
  const els = [
    getPodCreateProjectEl(),
    getPodCreateNextEl(),
    getPodCreateSubmitEl()
  ];
  els.forEach(el => { if (el) el.disabled = state.podCreateBusy; });
}

export function setPodCreateStatus(message, level) {
  const el = getPodCreateStatusEl();
  if (!el) return;
  el.textContent = message;
  el.style.color = level === "ok" ? "var(--ok)" : level === "err" ? "var(--err)" : "var(--muted)";
}

export function renderPodProjectOptions() {
  const el = getPodCreateProjectEl();
  if (!el) return;
  const selected = el.value;
  el.innerHTML = '<option value="">Select project</option>';
  state.projects.sort((a,b) => a.name.localeCompare(b.name)).forEach(p => {
    const opt = document.createElement("option");
    opt.value = p.id;
    opt.textContent = p.name;
    el.appendChild(opt);
  });
  if (selected) el.value = selected;
}

export async function loadProjectIssues(refresh) {
  const project = selectedPodProject();
  if (!project) return;
  if (!refresh && state.podProjectIssues[project.id]) {
    renderPodIssueOptions(project.id);
    return;
  }
  
  setPodCreateStatus("Loading issues...", "muted");
  try {
    const issues = await fetchJSON("/v1/projects/" + project.id + "/issues");
    state.podProjectIssues[project.id] = Array.isArray(issues) ? issues : [];
    renderPodIssueOptions(project.id);
    setPodCreateStatus("Loaded issues.", "ok");
  } catch (err) {
    setPodCreateStatus("Failed to load issues.", "err");
  }
}

export function renderPodIssueOptions(projectID) {
  const el = getPodCreateIssueEl();
  if (!el) return;
  el.innerHTML = '<option value="">Select issue</option>';
  const issues = state.podProjectIssues[projectID] || [];
  issues.forEach(i => {
    const opt = document.createElement("option");
    opt.value = String(i.number);
    opt.textContent = "#" + i.number + " " + i.title;
    el.appendChild(opt);
  });
  el.disabled = issues.length === 0;
}

export function renderPodIssuePreview() {
  const project = selectedPodProject();
  const preview = getPodCreateIssuePreviewEl();
  if (!project || !preview) return;
  const issue = selectedPodIssue();
  if (!issue) {
    preview.textContent = "Select an issue for this loop.";
    return;
  }
  preview.textContent = "#" + issue.number + " " + (issue.title || "").trim();
  const branchEl = getPodCreateBranchEl();
  if (branchEl && !branchEl.value.trim()) {
    branchEl.value = defaultPodBranchName(project, issue);
  }
}

export function selectedPodProject() {
  const el = getPodCreateProjectEl();
  return state.projects.find(p => p.id === el?.value) || null;
}

export function selectedPodIssue() {
  const project = selectedPodProject();
  if (!project) return null;
  const el = getPodCreateIssueEl();
  return (state.podProjectIssues[project.id] || []).find(i => String(i.number) === el?.value) || null;
}

export function parseGitHubRepository(url) {
  if (!url) return "";
  const m = url.match(/github\.com[\/:]([^\/]+)\/([^\/.]+)/);
  return m ? m[1] + "/" + m[2] : "";
}

export function slugifySegment(v) {
  return String(v).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

export function normalizeBranchName(v) {
  return String(v).replace(/[^a-zA-Z0-9._/-]/g, "-");
}

export function defaultPodLoopName(issue) {
  return "issue-" + issue.number + "-" + slugifySegment(issue.title);
}

export function defaultPodBranchName(project, issue) {
  return "smith-" + issue.number;
}

export function preparePodCreateDetails() {
  const project = selectedPodProject();
  const issue = selectedPodIssue();
  const branchEl = getPodCreateBranchEl();
  if (branchEl && !branchEl.value) branchEl.value = defaultPodBranchName(project, issue);
  renderPodProviderOptions();
}

export function configuredLoopProviders() {
  return Object.keys(state.providerStatus).filter(id => state.providerStatus[id].connected);
}

export function renderPodProviderOptions() {
  const el = getPodCreateProviderEl();
  if (!el) return;
  el.innerHTML = '<option value="">Select provider</option>';
  configuredLoopProviders().forEach(id => {
    const opt = document.createElement("option");
    opt.value = id;
    opt.textContent = id;
    el.appendChild(opt);
  });
}

export function parsePromptAsPRDJSON(v) {
  try { return JSON.parse(v); } catch(_) { return null; }
}
