<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { fetchJSON, postJSON } from '$lib/api';
	import { onMount, onDestroy } from 'svelte';
	import { slugifySegment, normalizeBranchName } from '$lib/utils';
  import { Modal, Button, Label, Input, Select, Textarea, Badge } from 'flowbite-svelte';
  import { ArrowLeftOutline, ArrowRightOutline, RocketOutline, PaperPlaneOutline } from 'flowbite-svelte-icons';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open = $bindable(), onClose }: Props = $props();

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
		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}
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

  $effect(() => {
    if (!open) {
      step = 1;
      if (chatSocket) {
				chatSocket.close();
				chatSocket = null;
			}
    }
  });

	onDestroy(() => {
		if (chatSocket) {
			chatSocket.close();
			chatSocket = null;
		}
	});
</script>

<Modal bind:open title="Start New Loop" size="md" autoclose={false} class="bg-black border border-gray-800 rounded-none">
  <div class="space-y-6">
    <div class="flex justify-between items-center mb-4">
      <div class="flex gap-2">
        {#each Array.from({ length: maxStep }) as _, i}
          <div class="w-8 h-1 rounded-full {step > i ? 'bg-[#86BC25]' : 'bg-slate-800'}"></div>
        {/each}
      </div>
      <Badge color="dark" class="text-[10px] uppercase font-bold tracking-wider rounded-none bg-slate-800 text-gray-400">Step {step} of {maxStep}</Badge>
    </div>

    {#if step === 1}
      <div class="space-y-4">
        <p class="text-gray-400 text-sm">Choose how you want to start this development loop.</p>
        <div class="grid grid-cols-1 gap-3">
          <button 
            class="p-4 rounded-none border text-left transition-all {method === 'issue' ? 'bg-[#86BC25]/10 border-[#86BC25] text-[#86BC25]' : 'bg-slate-900 border-gray-800 text-gray-400 hover:border-gray-600'}"
            onclick={() => method = 'issue'}
          >
            <div class="font-bold uppercase text-xs tracking-widest">Create from Issue</div>
            <div class="text-[10px] opacity-70 mt-1 uppercase">Import requirements from a GitHub issue.</div>
          </button>
          <button 
            class="p-4 rounded-none border text-left transition-all {method === 'generate_prd' ? 'bg-[#86BC25]/10 border-[#86BC25] text-[#86BC25]' : 'bg-slate-900 border-gray-800 text-gray-400 hover:border-gray-600'}"
            onclick={() => method = 'generate_prd'}
          >
            <div class="font-bold uppercase text-xs tracking-widest">Generate a PRD</div>
            <div class="text-[10px] opacity-70 mt-1 uppercase">Chat with an agent to build a new document.</div>
          </button>
          <button 
            class="p-4 rounded-none border text-left transition-all {method === 'load_prd' ? 'bg-[#86BC25]/10 border-[#86BC25] text-[#86BC25]' : 'bg-slate-900 border-gray-800 text-gray-400 hover:border-gray-600'}"
            onclick={() => method = 'load_prd'}
          >
            <div class="font-bold uppercase text-xs tracking-widest">Load a PRD</div>
            <div class="text-[10px] opacity-70 mt-1 uppercase">Paste raw JSON or a manual prompt.</div>
          </button>
        </div>
      </div>
    {:else if step === 2}
      <div class="space-y-4">
        <div>
          <Label for="project" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Target Project</Label>
          <Select id="project" bind:value={projectID} onchange={loadIssues} class="bg-slate-900 border-gray-800 text-white rounded-none">
            <option value="">Select a project</option>
            {#each projects as p}
              <option value={p.id}>{p.name}</option>
            {/each}
          </Select>
        </div>
        {#if method === 'issue'}
          <div>
            <Label for="issue" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">GitHub Issue</Label>
            <Select id="issue" bind:value={issueNumber} disabled={issues.length === 0} class="bg-slate-900 border-gray-800 text-white rounded-none">
              <option value="">{issues.length === 0 ? 'No issues found' : 'Select an issue'}</option>
              {#each issues as issue}
                <option value={issue.number}>#{issue.number} {issue.title}</option>
              {/each}
            </Select>
          </div>
        {/if}
      </div>
    {:else if step === 3}
      <div class="space-y-4">
        <div>
          <Label for="loop-name" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Loop Identifier</Label>
          <Input type="text" id="loop-name" placeholder="fix-authentication-bug" bind:value={loopName} class="bg-slate-900 border-gray-800 text-white rounded-none" />
        </div>
        <div>
          <Label for="branch" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Branch Name</Label>
          <Input type="text" id="branch" placeholder="feature/auth-fix" bind:value={branch} class="bg-slate-900 border-gray-800 text-white rounded-none" />
        </div>
        {#if method === 'load_prd'}
          <div>
            <Label for="prompt" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">PRD Content / Prompt</Label>
            <Textarea id="prompt" rows={6} placeholder="Paste JSON or instructions..." bind:value={prompt} class="bg-slate-900 border-gray-800 text-white rounded-none" />
          </div>
        {/if}
      </div>
    {:else if step === 4}
      <div class="flex flex-col h-[400px]">
        <div class="flex-1 overflow-y-auto p-4 space-y-4 bg-black rounded-none border border-gray-800 mb-4">
          {#each chatMessages as msg}
            {#if msg.type !== 'system' || msg.text}
              <div class="flex {msg.type === 'user' ? 'justify-end' : 'justify-start'}">
                <div class="max-w-[85%] px-3 py-2 rounded-none text-xs {msg.type === 'user' ? 'bg-[#86BC25] text-black font-bold' : 'bg-slate-900 text-gray-200 border border-gray-800'}">
                  <div style="white-space: pre-wrap;">{msg.text || msg.error || ""}</div>
                </div>
              </div>
            {/if}
          {:else}
            <div class="flex justify-center items-center h-full text-gray-500 italic text-sm">
              Initializing PRD chat...
            </div>
          {/each}
        </div>

        {#if finalPRD}
          <Badge color="green" class="mb-4 py-2 rounded-none bg-[#86BC25] text-black font-bold uppercase text-[10px]">PRD Finalized</Badge>
        {/if}

        <div class="flex gap-2">
          <Input 
            type="text" 
            placeholder="Refine requirements..." 
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
    {/if}
  </div>

  <svelte:fragment slot="footer">
    <Button color="alternative" class="rounded-none border-gray-700" onclick={onClose}>Cancel</Button>
    <div class="flex gap-2 ml-auto">
      <Button color="alternative" class="rounded-none border-gray-700" onclick={prevStep} disabled={step === 1}>
        <ArrowLeftOutline size="sm" class="mr-2" /> Back
      </Button>
      {#if step < maxStep}
        <Button color="none" class="bg-[#86BC25] text-black font-bold uppercase text-xs px-6 py-2 rounded-none transition-all" onclick={nextStep}>
          Next <ArrowRightOutline size="sm" class="ml-2" />
        </Button>
      {:else}
        <Button color="none" class="bg-[#86BC25] text-black font-bold uppercase text-xs px-6 py-2 rounded-none transition-all" onclick={submit} disabled={busy || (isInteractive && !finalPRD)}>
          <RocketOutline size="sm" class="mr-2" /> Create Loop
        </Button>
      {/if}
    </div>
  </svelte:fragment>
</Modal>
