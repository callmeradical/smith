<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { onMount, onDestroy } from 'svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
		onDraftFinalized: (title: string, content: string) => void;
	}

	let { open, onClose, onDraftFinalized }: Props = $props();

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
		// Reusing the PRD chat endpoint or similar for document drafting
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
				// Standard protocol for finalizing
				if (msg.type === 'system' && msg.final_prd_path) {
					// In this context, msg.text is the finalized content
					finalContent = msg.text;
					finalTitle = "Drafted Document"; // Fallback or extracted from metadata
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

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="provider-drawer-overlay" style="opacity: 1; visibility: visible; pointer-events: auto; backdrop-filter: blur(8px);" onclick={onClose}></div>
	<aside class="doc-chat-modal open" aria-hidden="false">
		<div class="provider-drawer-head">
			<div class="provider-drawer-title">Draft Document with Agent</div>
			<button type="button" class="provider-drawer-close" onclick={onClose}>&times;</button>
		</div>
		<section class="panel chat-section" style="flex: 1; display: flex; flex-direction: column; overflow: hidden;">
			<div class="chat-panel" style="flex: 1; overflow-y: auto; padding: 12px; display: flex; flex-direction: column; gap: 8px;">
				{#each chatMessages as msg}
					{#if msg.type !== 'system' || msg.text}
						<div class="chat-bubble {msg.type === 'user' ? 'user' : 'agent'}">
							<div style="white-space: pre-wrap;">{msg.text || msg.error || ""}</div>
						</div>
					{/if}
				{/each}
			</div>
			
			<div class="chat-composer" style="padding: 12px; border-top: 1px solid var(--border); display: flex; gap: 8px;">
				<input 
					type="text" 
					placeholder="Describe your document needs..." 
					bind:value={chatInput}
					onkeydown={(e) => e.key === 'Enter' && sendChatMessage()}
					style="flex: 1;"
				/>
				<button class="primary" onclick={sendChatMessage} disabled={!chatSocket}>send</button>
			</div>

			{#if finalContent}
				<div class="finalize-banner" style="padding: 12px; background: rgba(16, 185, 129, 0.1); border-top: 1px solid var(--ok); display: flex; justify-content: space-between; align-items: center;">
					<span class="status-note ok">Document ready!</span>
					<button class="primary" onclick={handleFinalize}>Review & Save</button>
				</div>
			{/if}
		</section>
	</aside>
{/if}

<style>
	.doc-chat-modal {
		position: fixed;
		left: 50%;
		top: 50%;
		width: min(600px, calc(100vw - 28px));
		height: min(700px, calc(100vh - 28px));
		background: rgba(7, 9, 14, 0.98);
		border-radius: 14px;
		box-shadow: 0 16px 30px rgba(0, 0, 0, 0.45);
		padding: 12px;
		display: flex;
		flex-direction: column;
		transform: translate(-50%, -50%) scale(1);
		opacity: 1;
		visibility: visible;
		pointer-events: auto;
		z-index: 34;
	}
	.chat-section {
		background: var(--panel);
		border-radius: 8px;
		border: 1px solid var(--border);
	}
</style>
