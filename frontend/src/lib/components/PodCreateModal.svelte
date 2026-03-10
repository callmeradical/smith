<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { fetchJSON, postJSON } from '$lib/api';
	import { onMount } from 'svelte';
	import { slugifySegment, normalizeBranchName } from '$lib/utils';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	let step = $state(1);
	let method = $state('issue');
	let projectID = $state('');
	let issueNumber = $state('');
	let loopName = $state('');
	let branch = $state('');
	let sourceBranch = $state('main');
	let provider = $state('');
	let prompt = $state('');
	let busy = $state(false);
	
	let chatMessages = $state<{ type: string, text?: string, error?: string, final_prd_path?: string }[]>([]);
	let chatSocket: WebSocket | null = $state(null);
	let chatInput = $state('');
	let finalPRD = $state<string | null>(null);

	const isInteractive = $derived(method === 'issue' || method === 'generate_prd');
	const maxStep = $derived(isInteractive ? 4 : 3);

	const projects = $derived($appState.projects);
	const issues = $derived(projectID ? ($appState.podProjectIssues[projectID] || []) : []);

	async function loadIssues() {
		if (!projectID) return;
		busy = true;
		try {
			const raw = await fetchJSON("/v1/projects/" + projectID + "/issues");
			appState.update(s => ({
				...s,
				podProjectIssues: { ...s.podProjectIssues, [projectID]: Array.isArray(raw) ? raw : [] }
			}));
		} catch (err) {
			console.error("Failed to load issues", err);
		} finally {
			busy = false;
		}
	}

	function nextStep() {
		if (step === 2 && !projectID) {
			pushToast("Select a project first.", "err");
			return;
		}
		if (step === 3 && isInteractive) {
			startPRDChat();
		}
		step = Math.min(maxStep, step + 1);
	}

	function startPRDChat() {
		if (chatSocket) chatSocket.close();
		
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = window.location.host;
		const url = `${protocol}//${host}/api/v1/chat/prd`;

		chatSocket = new WebSocket(url);
		chatMessages = [];
		finalPRD = null;

		chatSocket.onopen = () => {
			let initialMsg = prompt;
			if (!initialMsg && issueNumber) {
				const issue = issues.find(i => String(i.number) === issueNumber);
				if (issue) {
					initialMsg = `Build a PRD from GitHub issue #${issue.number}: ${issue.title}\n\n${issue.body || ""}`;
				}
			}
			if (!initialMsg) initialMsg = "Let's build a new PRD.";
			chatSocket?.send(JSON.stringify({ type: 'user', text: initialMsg }));
		};

		chatSocket.onmessage = (event) => {
			try {
				const msg = JSON.parse(event.data);
				if (msg.type === 'system' && msg.final_prd_path) {
					finalPRD = msg.text;
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

	function prevStep() {
		step = Math.max(1, step - 1);
		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}
	}

	async function submit() {
		busy = true;
		try {
			const payload = {
				loop_id: slugifySegment(projectID + "-" + (loopName || "loop")),
				title: "Loop " + (loopName || "request"),
				provider_id: provider,
				source_type: method === 'issue' ? 'github_issue' : 'prompt',
				source_ref: method === 'issue' ? `${projectID}#${issueNumber}` : 'prompt',
				metadata: {
					project_id: projectID,
					workspace_branch: branch,
					workspace_source_branch: sourceBranch,
					workspace_prd_json: finalPRD || (method === 'load_prd' ? prompt : ""),
				}
			};
			await postJSON("/v1/loops", payload);
			pushToast("Loop created successfully.", "ok");
			onClose();
		} catch (err: any) {
			pushToast(err.message, "err");
		} finally {
			busy = false;
		}
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="provider-drawer-overlay" style="opacity: 1; visibility: visible; pointer-events: auto; backdrop-filter: blur(8px);" onclick={onClose}></div>
	<aside class="pod-create-modal open" aria-hidden="false">
		<div class="provider-drawer-head">
			<div class="provider-drawer-title">New Loop</div>
			<button type="button" class="provider-drawer-close" onclick={onClose}>&times;</button>
		</div>
		<section class="panel">
			<div class="pod-create-step-indicators">
				{#each Array.from({ length: maxStep }) as _, i}
					<div class="pod-create-step-chip" class:active={step === i + 1} class:complete={step > i + 1}>
						<span class="pod-create-step-node">{i + 1}</span>
					</div>
				{/each}
			</div>

			{#if step === 1}
				<div class="pod-create-step-panel">
					<div class="status-note">How should this loop start?</div>
					<div class="pod-create-method-options">
						<button class="pod-create-method-button" class:selected={method === 'issue'} onclick={() => method = 'issue'}>
							<strong>Create from Issue</strong>
						</button>
						<button class="pod-create-method-button" class:selected={method === 'generate_prd'} onclick={() => method = 'generate_prd'}>
							<strong>Generate a PRD</strong>
						</button>
						<button class="pod-create-method-button" class:selected={method === 'load_prd'} onclick={() => method = 'load_prd'}>
							<strong>Load a PRD</strong>
						</button>
					</div>
				</div>
			{:else if step === 2}
				<div class="pod-create-step-panel">
					<select bind:value={projectID} onchange={loadIssues}>
						<option value="">Select project</option>
						{#each projects as p}
							<option value={p.id}>{p.name}</option>
						{/each}
					</select>
					{#if method === 'issue'}
						<select bind:value={issueNumber} disabled={issues.length === 0}>
							<option value="">Select issue</option>
							{#each issues as issue}
								<option value={issue.number}>#{issue.number} {issue.title}</option>
							{/each}
						</select>
					{/if}
				</div>
			{:else if step === 3}
				<div class="pod-create-step-panel">
					<input type="text" placeholder="loop name" bind:value={loopName} />
					<input type="text" placeholder="branch name" bind:value={branch} />
					<textarea placeholder="prompt or PRD JSON" bind:value={prompt}></textarea>
				</div>
			{:else if step === 4}
				<div class="pod-create-step-panel">
					<div class="chat-shell" style="height: 300px; margin-top: 12px;">
						<div class="chat-panel">
							{#each chatMessages as msg}
								{#if msg.type !== 'system' || msg.text}
									<div class="chat-bubble {msg.type === 'user' ? 'user' : 'agent'}">
										<div>{msg.text || msg.error || ""}</div>
									</div>
								{/if}
							{/each}
						</div>
						<div class="chat-composer">
							<input 
								type="text" 
								placeholder="Type to refine PRD..." 
								bind:value={chatInput}
								onkeydown={(e) => e.key === 'Enter' && sendChatMessage()}
							/>
							<button class="primary" onclick={sendChatMessage}>send</button>
						</div>
					</div>
					{#if finalPRD}
						<div class="status-note ok">PRD finalized! Click 'create loop' to start.</div>
					{:else}
						<div class="status-note">Chat with agent to finalize PRD.</div>
					{/if}
				</div>
			{/if}

			<div class="pod-create-footer">
				<button onclick={prevStep} disabled={step === 1}>back</button>
				{#if step < maxStep}
					<button class="primary" onclick={nextStep}>next</button>
				{:else}
					<button class="primary" onclick={submit} disabled={busy}>create loop</button>
				{/if}
				<button onclick={onClose}>cancel</button>
			</div>
		</section>
	</aside>
{/if}

<style>
	.pod-create-modal {
		transform: translate(-50%, -50%) scale(1);
		opacity: 1;
		visibility: visible;
		pointer-events: auto;
	}
	.pod-create-step-chip {
		display: flex;
		flex-direction: column;
		align-items: center;
	}
</style>
