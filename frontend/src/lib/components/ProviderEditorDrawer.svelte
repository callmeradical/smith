<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { postJSON, requestJSON, fetchJSON } from '$lib/api';
  import { Drawer, Button, Label, Input, Helper } from 'flowbite-svelte';
  import { BrainOutline, CheckOutline, CloseOutline, AdjustmentsHorizontalOutline } from 'flowbite-svelte-icons';
  import { sineIn } from 'svelte/easing';

	interface Props {
		open: boolean;
		onClose: () => void;
		provider?: any;
	}

	let { open = $bindable(), onClose, provider = null }: Props = $props();

	// Form fields
	let apiKey = $state('');
	let accountId = $state('');
	let busy = $state(false);

  // Use bind:hidden for drawer control
  let isHidden = $state(true);
  $effect(() => { isHidden = !open; });
  $effect(() => { if (open && isHidden) onClose(); });

  let transitionParams = {
    x: 450,
    duration: 300,
    easing: sineIn
  };

	$effect(() => {
		if (open) {
			apiKey = '';
      accountId = '';
		}
	});

	async function saveProvider() {
		if (!apiKey) {
			pushToast("API Key is required", "err");
			return;
		}
		busy = true;
		try {
			await postJSON(`/v1/auth/codex/connect/api-key`, {
				actor: "operator", api_key: apiKey, account_id: accountId || 'default'
			});
			pushToast(`Provider ${provider?.label || 'Codex'} configured successfully`, "ok");
			onClose();
		} catch (err: any) {
			pushToast(err.message || "Failed to configure provider", "err");
		} finally {
			busy = false;
		}
	}
</script>

<Drawer 
  placement="right" 
  bind:hidden={isHidden} 
  outsideclose={true}
  id="provider-editor-drawer" 
  width="default" 
  class="fixed top-0 right-0 bg-black border-l border-gray-800 z-50 overflow-y-auto h-full m-0 shadow-2xl p-0 w-[450px]"
>
  <div class="flex flex-col h-full relative">
    <!-- Header -->
    <div class="px-8 py-10 bg-slate-900/20 border-b border-gray-900 flex items-center justify-between">
      <div class="flex items-center gap-4">
        <div class="w-12 h-12 bg-[#86BC25]/10 flex items-center justify-center text-[#86BC25]">
          <AdjustmentsHorizontalOutline size="lg" />
        </div>
        <div>
          <h2 class="text-2xl font-bold text-white uppercase tracking-tighter">Configure Provider</h2>
          <p class="text-[10px] font-bold text-gray-500 uppercase tracking-[0.2em] mt-1">{provider?.label || 'OpenAI Codex'}</p>
        </div>
      </div>
      <button 
        class="text-white hover:text-[#86BC25] transition-colors p-2"
        onclick={onClose}
        aria-label="Close Drawer"
      >
        <CloseOutline size="md" />
      </button>
    </div>

    <!-- Body -->
    <form class="flex-1 p-8 space-y-8" onsubmit={(e) => { e.preventDefault(); saveProvider(); }}>
      <div class="space-y-6">
        <div class="bg-slate-900/30 p-4 border border-gray-800 rounded-none">
          <div class="flex items-center gap-3 mb-2 text-[#86BC25]">
            <BrainOutline size="sm" />
            <span class="text-[10px] font-bold uppercase tracking-widest">OpenAI Integration</span>
          </div>
          <p class="text-[11px] text-gray-400 leading-relaxed">
            Configure your OpenAI API key to enable autonomous code generation and analysis. Keys are securely stored and never exposed in the UI.
          </p>
        </div>

        <div>
          <Label for="api-key" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">OpenAI API Key</Label>
          <Input type="password" id="api-key" placeholder="sk-..." bind:value={apiKey} disabled={busy} required class="bg-black border-gray-800 text-white rounded-none focus:border-[#86BC25]" />
          <Helper class="mt-2 text-gray-600 text-[10px] uppercase font-bold">Paste your secret key from the OpenAI dashboard.</Helper>
        </div>

        <div>
          <Label for="account-id" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Account ID (Optional)</Label>
          <Input type="text" id="account-id" placeholder="Optional Organization ID" bind:value={accountId} disabled={busy} class="bg-black border-gray-800 text-white rounded-none focus:border-[#86BC25]" />
        </div>
      </div>

      <!-- Footer Actions -->
      <div class="pt-10 pb-20 border-t border-gray-900 flex justify-end gap-4 mt-auto">
        <Button color="alternative" size="sm" class="rounded-none font-bold uppercase text-[10px] tracking-widest border-gray-700 bg-slate-900 text-gray-300 hover:bg-slate-700 px-6" onclick={onClose} disabled={busy}>Cancel</Button>
        <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest rounded-none px-8 py-2 hover:bg-[#a1e02c]" type="submit" disabled={busy}>
          <CheckOutline size="xs" class="mr-2" />
          Update Credentials
        </Button>
      </div>
    </form>
  </div>
</Drawer>

<style>
  :global(#provider-editor-drawer) {
    background-color: #000000 !important;
    left: auto !important;
    right: 0 !important;
  }
  /* Force hide default absolute close button */
  :global(#provider-editor-drawer button[aria-label="Close"]) {
    display: none !important;
  }
  /* Preserve custom close button visibility */
  :global(#provider-editor-drawer .relative > button[aria-label="Close Drawer"]) {
    display: flex !important;
  }
</style>
