<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { appState } from '$lib/stores';

	interface Props {
		loopID: string;
	}

	let { loopID }: Props = $props();

	let terminalEl: HTMLPreElement | null = $state(null);

	function appendJournal(entry: any) {
		if (!entry || typeof entry !== 'object') return;
		appState.update(s => {
			const seq = Number(entry.sequence || entry.Sequence || 0);
			if (seq <= s.journalLastSeq) return s;
			
			const journalEntries = [...s.journalEntries, entry].slice(-500);
			return { ...s, journalEntries, journalLastSeq: seq };
		});
	}

	function renderJournal() {
		if (!terminalEl) return;
		if ($appState.journalEntries.length === 0) {
			terminalEl.textContent = "[journal] waiting for entries...\n";
			return;
		}
		const lines = $appState.journalEntries.map((entry: any) => {
			const ts = String(entry.timestamp || entry.Timestamp || "");
			const level = String(entry.level || entry.Level || "info").toLowerCase();
			const phase = String(entry.phase || entry.Phase || "-");
			const actor = String(entry.actor_id || entry.ActorID || "-");
			const msg = String(entry.message || entry.Message || "");
			return `[${ts}] [${level}] [${phase}] [${actor}] ${msg}`;
		});
		terminalEl.textContent = lines.join("\n") + "\n";
		terminalEl.scrollTop = terminalEl.scrollHeight;
	}

	let source: EventSource | null = null;

	function connect() {
		if (source) source.close();
		if (!loopID) return;

		const url = `/api/v1/loops/${encodeURIComponent(loopID)}/journal/stream`;
		source = new EventSource(url);

		source.onmessage = (event) => {
			try {
				const entry = JSON.parse(event.data);
				appendJournal(entry);
				renderJournal();
			} catch (err) {
				console.error("Failed to parse journal entry", err);
			}
		};

		source.onerror = () => {
			source?.close();
			source = null;
			setTimeout(connect, 3000);
		};
	}

	onMount(() => {
		appState.update(s => ({ ...s, journalEntries: [], journalLastSeq: 0 }));
		connect();
	});

	onDestroy(() => {
		if (source) source.close();
	});
</script>

<div id="pod-view-journal-shell" class="journal">
	<div class="journal-header">
		<strong>Live Journal: {loopID}</strong>
	</div>
	<pre bind:this={terminalEl} class="terminal">[journal] attaching to stream...
</pre>
</div>
