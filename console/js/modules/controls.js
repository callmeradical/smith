import { state } from "./state.js";
import { postJSON } from "./api.js";
import { pushToast } from "./ui.js";
import { getJournalStatusEl } from "./elements.js";

export async function applyOverride() {
  const loopID = state.selectedLoop;
  const stateEl = document.getElementById("override-state");
  const reasonEl = document.getElementById("override-reason");
  const actorEl = document.getElementById("override-actor");
  const confirmEl = document.getElementById("override-confirm");
  const statusEl = getJournalStatusEl();

  if (!loopID) {
    if (statusEl) statusEl.textContent = "select a loop first";
    return;
  }

  const targetState = stateEl?.value;
  const reason = reasonEl?.value.trim();
  const actor = actorEl?.value.trim() || "operator";
  const confirm = confirmEl?.value.trim();

  if (!reason) {
    if (statusEl) statusEl.textContent = "override reason required";
    return;
  }

  if (confirm !== "APPLY") {
    if (statusEl) statusEl.textContent = "type APPLY to confirm";
    return;
  }

  if (statusEl) statusEl.textContent = "applying...";
  try {
    await postJSON(`/v1/loops/${encodeURIComponent(loopID)}/control/override`, {
      target_state: targetState,
      reason,
      actor,
    });
    pushToast("Override applied successfully.", "ok");
    if (reasonEl) reasonEl.value = "";
    if (confirmEl) confirmEl.value = "";
    if (statusEl) statusEl.textContent = "override applied";
    import("./pods.js").then(m => m.refreshLoops());
  } catch (err) {
    if (statusEl) statusEl.textContent = "error: " + err.message;
    pushToast("Override failed: " + err.message, "err");
  }
}
