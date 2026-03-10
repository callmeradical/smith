<script lang="ts">
	import { page } from '$app/state';
	import { appState, pushToast } from '$lib/stores';
	import { onMount } from 'svelte';
	import { getJSON, postJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import Journal from '$lib/components/Journal.svelte';
	import { goto } from '$app/navigation';

	const id = $derived(page.params.id);
	
	let command = $state('');
	let busy = $state(false);

	async function runCommand() {
		if (!command || !id) return;
		busy = true;
		try {
			await postJSON(`/v1/loops/${encodeURIComponent(id)}/control/command`, {
				actor: "operator",
				command,
			});
			command = '';
			pushToast("Command sent", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		} finally {
			busy = false;
		}
	}

	async function terminate() {
		if (!confirm("Force terminate this loop?")) return;
		try {
			await postJSON(`/v1/loops/${encodeURIComponent(id)}/control/terminate`, { actor: "operator" });
			pushToast("Termination requested", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}
</script>

{#snippet controls()}
	<button type="button" onclick={() => goto('/pods')}>&larr; back</button>
	<button type="button" class="danger" onclick={terminate}>terminate</button>
{/snippet}

<TopBar title={`Pod: ${id}`} {controls} />

<Journal loopID={id} />

<section class="pod-control-panel">
	<div class="pod-command-row journal-prompt-row">
		<span class="journal-prompt-glyph" aria-hidden="true">$</span>
		<input
			type="text"
			placeholder="Run command (e.g. pwd)"
			bind:value={command}
			onkeydown={(e) => e.key === 'Enter' && runCommand()}
		/>
		<button type="button" onclick={runCommand} disabled={busy}>
			{busy ? 'running...' : 'run'}
		</button>
	</div>
</section>
