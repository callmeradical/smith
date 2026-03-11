<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { onMount, onDestroy } from 'svelte';
  import { Modal, Button, Input, Badge } from 'flowbite-svelte';
  import { PaperPlaneOutline, CheckOutline } from 'flowbite-svelte-icons';

	interface Props {
		open: boolean;
		onClose: () => void;
		onDraftFinalized: (title: string, content: string) => void;
	}

	let { open = $bindable(), onClose, onDraftFinalized }: Props = $props();

	let chatMessages = $state<{ type: string, text?: string, error?: string, final_content?: string, final_title?: string }[]>([]);
	let chatSocket: WebSocket | null = $state(null);
	let chatInput = $state('');
	let busy = $state(false);
	let finalContent = $state<string | null>(null);
	let finalTitle = $state<string | null>(null);

	function connectChat() {
		if (chatSocket) chatSocket.close();
		
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = window.location.host;
		const url = `${protocol}//${host}/api/v1/chat/prd`;

		chatSocket = new WebSocket(url);
		chatMessages = [];
		finalContent = null;
		finalTitle = null;

		chatSocket.onopen = () => {
			chatSocket?.send(JSON.stringify({ type: 'user', text: "I want to draft a new document. Help me refine the requirements." }));
		};

		chatSocket.onmessage = (event) => {
			try {
				const msg = JSON.parse(event.data);
				if (msg.type === 'system' && msg.final_prd_path) {
					finalContent = msg.text;
					finalTitle = "Drafted Document";
				}
				chatMessages = [...chatMessages, msg];
			} catch (err) {
				console.error("Failed to parse chat message", err);
			}
		};

		chatSocket.onclose = () => {
			chatSocket = null;
		};
	}

	function sendChatMessage() {
		if (!chatInput || !chatSocket) return;
		chatSocket.send(JSON.stringify({ type: 'user', text: chatInput }));
		chatMessages = [...chatMessages, { type: 'user', text: chatInput }];
		chatInput = '';
	}

	function handleFinalize() {
		if (finalContent) {
			onDraftFinalized(finalTitle || "New Document", finalContent);
			onClose();
		}
	}

	$effect(() => {
		if (open) {
			connectChat();
		} else {
			if (chatSocket) chatSocket.close();
		}
	});

	onDestroy(() => {
		if (chatSocket) chatSocket.close();
	});
</script>

<Modal bind:open title="Draft Document with AI" size="lg" autoclose={false} class="bg-black border border-gray-800 rounded-none">
  <div class="flex flex-col h-[500px]">
    <div class="flex-1 overflow-y-auto p-4 space-y-4 bg-black rounded-none border border-gray-800 mb-4">
      {#each chatMessages as msg}
        {#if msg.type !== 'system' || msg.text}
          <div class="flex {msg.type === 'user' ? 'justify-end' : 'justify-start'}">
            <div class="max-w-[80%] px-4 py-2 rounded-none text-sm {msg.type === 'user' ? 'bg-[#86BC25] text-black font-bold' : 'bg-slate-900 text-gray-200 border border-gray-800'}">
              <div style="white-space: pre-wrap;">{msg.text || msg.error || ""}</div>
            </div>
          </div>
        {/if}
      {:else}
        <div class="flex justify-center items-center h-full text-gray-500 italic">
          Connecting to drafting agent...
        </div>
      {/each}
    </div>

    {#if finalContent}
      <div class="p-3 bg-[#86BC25]/10 border border-[#86BC25]/30 rounded-none mb-4 flex justify-between items-center">
        <Badge color="green" class="bg-[#86BC25] text-black font-bold">Document Ready</Badge>
        <Button size="xs" color="none" class="bg-[#86BC25] text-black font-bold px-4 py-1" onclick={handleFinalize}>Review & Save</Button>
      </div>
    {/if}

    <div class="flex gap-2">
      <Input 
        type="text" 
        placeholder="Describe your document needs..." 
        bind:value={chatInput}
        disabled={!chatSocket}
        onkeydown={(e) => e.key === 'Enter' && sendChatMessage()}
        class="bg-slate-900 border-gray-800 text-white rounded-none"
      />
      <Button color="none" class="bg-[#86BC25] text-black px-4 rounded-none" onclick={sendChatMessage} disabled={!chatSocket || !chatInput}>
        <PaperPlaneOutline size="sm" />
      </Button>
    </div>
  </div>

  <svelte:fragment slot="footer">
    <Button color="alternative" class="rounded-none border-gray-700" onclick={onClose}>Close</Button>
  </svelte:fragment>
</Modal>
