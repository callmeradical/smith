import { state } from "./state.js";
import { postJSON, getJSON, deleteJSON } from "./api.js";
import { pushToast } from "./ui.js";
import { 
  getPodViewTerminalStateEl, 
  getPodViewRuntimeTargetEl, 
  getPodViewControlMessageEl,
  getPodViewCommandEl,
  getPodViewRunEl,
  getPodViewAttachEl,
  getPodViewCancelEl,
  getPodViewTerminateEl,
  getPodViewDeleteEl,
  getPages
} from "./elements.js";

function isActive(status) {
  return status === "unresolved" || status === "running";
}

function selectedLoopRecord() {
  const loopID = state.selectedLoop;
  if (!loopID) return null;
  return state.loops.find((item) => item.loopID === loopID) || null;
}

function selectedLoopRuntime() {
  const selected = selectedLoopRecord();
  if (!selected) return null;
  return state.runtimeByLoopID[selected.loopID] || null;
}

function runtimeSummaryText(runtime) {
  if (!runtime || typeof runtime !== "object") {
    return "runtime target: -- / -- / -- (phase --)";
  }
  const namespace = String(runtime.namespace || "--");
  const podName = String(runtime.pod_name || "--");
  const containerName = String(runtime.container_name || "--");
  const phase = String(runtime.pod_phase || "--");
  return (
    "runtime target: " +
    namespace +
    " / " +
    podName +
    " / " +
    containerName +
    " (phase " +
    phase +
    ")"
  );
}

function runtimeReason(runtime) {
  if (!runtime || runtime.attachable) return "";
  return String(runtime.reason || "runtime target not attachable").trim();
}

export function setTerminalUIState(nextState, message) {
  state.terminalUIState = nextState;
  state.terminalMessage = String(message || "").trim();
  syncPodViewActions();
}

export function syncPodViewActions() {
  const selected = selectedLoopRecord();
  const hasSelection = Boolean(selected);
  const selectedLoopID = hasSelection ? String(selected.loopID || "").trim() : "";
  const runtime = selectedLoopRuntime();
  const loadingRuntime = hasSelection && state.runtimeBusyLoopID === selectedLoopID;
  const runtimeAttachable = Boolean(runtime && runtime.attachable);
  const runtimePhase = String((runtime && runtime.pod_phase) || "").trim().toLowerCase();
  const runtimeUnhealthy =
    !runtimeAttachable ||
    runtimePhase === "pending" ||
    runtimePhase === "unknown" ||
    runtimePhase === "failed";
  const attachedToSelected = hasSelection && state.terminalAttachedLoopID === selectedLoopID;
  const terminalState = state.terminalUIState || "idle";
  const attaching = terminalState === "attaching";
  const executing = terminalState === "executing";
  const detaching = terminalState === "detaching";
  const loopControlBusy = Boolean(state.loopControlBusy);
  const controlBusyOnSelected =
    loopControlBusy && state.loopControlBusyLoopID === selectedLoopID;
  const cancelling = controlBusyOnSelected && state.loopControlAction === "cancel";
  const terminating = controlBusyOnSelected && state.loopControlAction === "terminate";
  const controlsLocked =
    state.loopDeleteBusy || loopControlBusy || loadingRuntime || attaching || executing || detaching;

  const attachEl = getPodViewAttachEl();
  if (attachEl) {
    const canToggleAttach = hasSelection && !controlsLocked && (attachedToSelected || runtimeAttachable);
    attachEl.disabled = !canToggleAttach;
    if (loadingRuntime && !attachedToSelected) {
      attachEl.textContent = "loading...";
    } else if (attaching) {
      attachEl.textContent = "attaching...";
    } else if (detaching) {
      attachEl.textContent = "detaching...";
    } else if (attachedToSelected) {
      attachEl.textContent = "detach";
    } else {
      attachEl.textContent = "attach";
    }
  }

  const cancelEl = getPodViewCancelEl();
  if (cancelEl) {
    const canCancel = hasSelection && isActive(selected.status) && !controlsLocked;
    cancelEl.disabled = !canCancel;
    cancelEl.textContent = cancelling ? "cancelling..." : "cancel";
  }

  const terminateEl = getPodViewTerminateEl();
  if (terminateEl) {
    const canTerminate = hasSelection && selected.status === "running" && runtimeUnhealthy && !controlsLocked;
    terminateEl.disabled = !canTerminate;
    terminateEl.textContent = terminating ? "terminating..." : "terminate";
  }

  const deleteEl = getPodViewDeleteEl();
  if (deleteEl) {
    const canDelete = hasSelection && !isActive(selected.status) && !controlsLocked;
    deleteEl.disabled = !canDelete;
    deleteEl.textContent = state.loopDeleteBusy ? "deleting..." : "delete loop";
  }

  const commandEl = getPodViewCommandEl();
  const runEl = getPodViewRunEl();
  if (commandEl && runEl) {
    const commandEnabled = hasSelection && attachedToSelected && !controlsLocked;
    const commandText = String(commandEl.value || "").trim();
    commandEl.disabled = !commandEnabled;
    commandEl.placeholder = commandEnabled ? "Run command (for example: pwd)" : "Attach to run commands";
    runEl.disabled = !commandEnabled || commandText === "";
    runEl.textContent = executing ? "running..." : "run";
  }

  const terminalStateEl = getPodViewTerminalStateEl();
  if (terminalStateEl) {
    const status = attachedToSelected && terminalState === "idle" ? "attached" : terminalState;
    terminalStateEl.textContent = status;
    terminalStateEl.className = "pod-terminal-state state-" + status;
  }

  const runtimeTargetEl = getPodViewRuntimeTargetEl();
  if (runtimeTargetEl) {
    runtimeTargetEl.textContent = runtimeSummaryText(runtime);
  }

  const controlMessageEl = getPodViewControlMessageEl();
  if (controlMessageEl) {
    const resolvedRuntimeReason = runtimeReason(runtime);
    let controlMessage = "";
    const status = attachedToSelected && terminalState === "idle" ? "attached" : terminalState;
    if (!hasSelection) {
      controlMessage = "Select a loop to resolve runtime target.";
    } else if (loadingRuntime) {
      controlMessage = "Loading runtime target...";
    } else if (resolvedRuntimeReason && !attachedToSelected) {
      controlMessage = resolvedRuntimeReason;
    } else if (state.terminalMessage) {
      controlMessage = state.terminalMessage;
    } else if (status === "attached") {
      controlMessage = "Attached. Press Enter or Run to execute a command.";
    } else if (status === "idle" && runtimeAttachable) {
      controlMessage = "Attach to enable command execution.";
    } else if (status === "idle") {
      controlMessage = "Runtime target not attachable.";
    } else if (status === "executing") {
      controlMessage = "Command execution in progress...";
    } else if (status === "attaching") {
      controlMessage = "Creating terminal session...";
    } else if (status === "detaching") {
      controlMessage = "Closing terminal session...";
    }
    controlMessageEl.textContent = controlMessage;
  }
}

export async function runSelectedLoopCommand() {
  const id = state.selectedLoop;
  if (!id) return;
  const commandEl = getPodViewCommandEl();
  const command = commandEl?.value.trim();
  if (!command) return;

  setTerminalUIState("executing", "");
  try {
    await postJSON(`/v1/loops/${encodeURIComponent(id)}/control/command`, {
      actor: "operator",
      command,
    });
    if (commandEl) commandEl.value = "";
    setTerminalUIState("attached", "");
  } catch (err) {
    setTerminalUIState("error", err.message);
  }
}
