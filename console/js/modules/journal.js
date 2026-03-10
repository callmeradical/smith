import { state } from "./state.js";
import { getTerminalEl, getJournalLatencyEl, getJournalStatusEl } from "./elements.js";
import { renderLatency, pushToast } from "./ui.js";
import { fetchJSON } from "./api.js";

export function clearJournal(reason) {
  state.journalEntries = [];
  state.journalLastSeq = 0;
  state.latencySamplesMs = [];
  renderLatency(state.latencySamplesMs);
  const terminal = getTerminalEl();
  if (terminal) terminal.textContent = reason ? "[journal] " + reason + "\n" : "";
}

export function disconnectStream() {
  if (state.journalSource) {
    state.journalSource.close();
    state.journalSource = null;
  }
  if (state.reconnectTimer) {
    clearTimeout(state.reconnectTimer);
    state.reconnectTimer = null;
  }
}

export function attachStream(loopID) {
  disconnectStream();
  if (!loopID) return;

  const url = `/api/v1/loops/${encodeURIComponent(loopID)}/journal/stream`;
  const source = new EventSource(url);
  state.journalSource = source;

  const statusEl = getJournalStatusEl();
  if (statusEl) statusEl.textContent = "connecting...";

  source.onopen = () => {
    if (statusEl) statusEl.textContent = "connected";
  };

  source.onmessage = (event) => {
    try {
      const entry = JSON.parse(event.data);
      appendJournal(entry);
    } catch (err) {
      console.error("Failed to parse journal entry", err);
    }
  };

  source.onerror = () => {
    if (statusEl) statusEl.textContent = "error (reconnecting...)";
    disconnectStream();
    state.reconnectTimer = setTimeout(() => attachStream(loopID), 3000);
  };
}

export function appendJournal(entry) {
  if (!entry || typeof entry !== "object") return;
  const seq = Number(entry.sequence || entry.Sequence || 0);
  if (seq <= state.journalLastSeq) return;
  state.journalLastSeq = seq;
  const tsRaw = String(entry.timestamp || entry.Timestamp || "");
  const tsMs = Date.parse(tsRaw);
  if (Number.isFinite(tsMs)) {
    const latencyMs = Date.now() - tsMs;
    if (latencyMs >= 0 && latencyMs < 10 * 60 * 1000) {
      state.latencySamplesMs.push(latencyMs);
      if (state.latencySamplesMs.length > 300) {
        state.latencySamplesMs.splice(0, state.latencySamplesMs.length - 300);
      }
      renderLatency(state.latencySamplesMs);
    }
  }
  state.journalEntries.push(entry);
  if (state.journalEntries.length > 500) {
    state.journalEntries.splice(0, state.journalEntries.length - 500);
  }
  renderJournal();
}

export function renderJournal() {
  const terminal = getTerminalEl();
  if (!terminal) return;
  if (state.journalEntries.length === 0) {
    terminal.textContent = "[journal] waiting for entries...\n";
    return;
  }
  const lines = state.journalEntries.map((entry) => {
    const ts = String(entry.timestamp || entry.Timestamp || "");
    const level = String(entry.level || entry.Level || "info").toLowerCase();
    const phase = String(entry.phase || entry.Phase || "-");
    const actor = String(entry.actor_id || entry.ActorID || "-");
    const msg = String(entry.message || entry.Message || "");
    return "[" + ts + "] [" + level + "] [" + phase + "] [" + actor + "] " + msg;
  });
  terminal.textContent = lines.join("\n") + "\n";
  terminal.scrollTop = terminal.scrollHeight;
}
