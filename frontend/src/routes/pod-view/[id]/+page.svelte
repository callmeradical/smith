<script lang="ts">
	import { page } from '$app/state';
	import { appState, pushToast } from '$lib/stores';
	import { onMount } from 'svelte';
	import { getJSON, postJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import Journal from '$lib/components/Journal.svelte';
	import { goto } from '$app/navigation';
  import { Button, Input } from 'flowbite-svelte';
  import { ArrowLeftOutline, TrashBinOutline, TerminalOutline } from 'flowbite-svelte-icons';

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
		if (!confirm("Force terminate this loop? (State will be set to flatline)")) return;
		busy = true;
		try {
			await postJSON('/v1/control/override', {
				loop_id: id,
				target_state: "flatline",
				reason: "terminated via console",
				actor: "operator",
			});
			pushToast("Termination requested", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		} finally {
			busy = false;
		}
	}

	async function cancel() {
		if (!confirm("Cancel this loop?")) return;
		busy = true;
		try {
			await postJSON('/v1/control/override', {
				loop_id: id,
				target_state: "cancelled",
				reason: "cancelled via console",
				actor: "operator",
			});
			pushToast("Cancellation requested", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		} finally {
			busy = false;
		}
	}
</script>

<TopBar title={`Pod: ${id}`} />

<!-- Inline Actions Header -->
<div class="flex justify-end gap-2 -mt-14 mb-8 relative z-50 px-4">
  <Button color="alternative" class="bg-black border-gray-800 text-gray-400 hover:text-white rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={() => goto('/pods')}>
    <ArrowLeftOutline size="xs" class="mr-1.5" />
    Back
  </Button>
  <Button color="alternative" class="bg-black border-gray-800 text-gray-400 hover:text-white rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={cancel} disabled={busy}>
    Cancel
  </Button>
  <Button color="red" class="rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7 border-none" onclick={terminate} disabled={busy}>
    <TrashBinOutline size="xs" class="mr-1.5" />
    Terminate
  </Button>
</div>

<div class="px-4">
  <Journal loopID={id || ""}>
    <div class="pod-command-row bg-black border border-gray-800 rounded-none p-2 flex items-center gap-3">
      <div class="flex items-center gap-2 pl-2 text-[#86BC25]">
        <TerminalOutline size="xs" />
        <span class="font-mono font-bold">$</span>
      </div>
      <Input
        type="text"
        placeholder="Run command (e.g. ls -la)"
        bind:value={command}
        onkeydown={(e) => e.key === 'Enter' && runCommand()}
        disabled={busy}
        size="sm"
        class="bg-transparent border-none text-white font-mono flex-1 focus:ring-0 rounded-none"
      />
      <Button size="xs" color="alternative" class="bg-[#86BC25] text-black font-bold uppercase px-4 rounded-none h-7 text-[10px]" onclick={runCommand} disabled={busy || !command}>
        {busy ? '...' : 'Execute'}
      </Button>
    </div>
  </Journal>
</div>

<style>
  :global(.pod-command-row input) {
    background: transparent !important;
  }
</style>
