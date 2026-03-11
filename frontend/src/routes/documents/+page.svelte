<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { fetchJSON, deleteJSON, requestJSON, postJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import DocChatModal from '$lib/components/DocChatModal.svelte';
	import { onMount } from 'svelte';
  import { Checkbox, Button, Textarea, Toolbar, ToolbarButton, ToolbarGroup, Select, Input, Label, Badge } from 'flowbite-svelte';
  import { PlusOutline, MessagesOutline, FileLinesOutline, CloudArrowUpOutline, TrashBinOutline, RocketOutline, ArchiveOutline, EditOutline, CloseOutline } from 'flowbite-svelte-icons';
  import { marked } from 'marked';

	let showAll = $state(false);
	let chatOpen = $state(false);
	let isEditing = $state(false);
	
	let selectedDocId = $state<string | null>(null);
	let editTitle = $state("");
	let editContent = $state("");
	let editProjectID = $state("");

	const selectedDoc = $derived(
		$appState.documents.find((d: any) => d.id === selectedDocId) || null
	);

  const renderedContent = $derived.by(() => {
    return marked.parse(editContent || '');
  });

	const projectsWithDocs = $derived.by(() => {
		const grouped: Record<string, any[]> = {};
		$appState.documents.forEach((d: any) => {
			if (!showAll && d.status === 'archived') return;
			if (!grouped[d.project_id]) grouped[d.project_id] = [];
			grouped[d.project_id].push(d);
		});
		return grouped;
	});

	const projectIDs = $derived(Object.keys(projectsWithDocs).sort());

	function selectDocument(doc: any) {
		selectedDocId = doc.id;
		editTitle = doc.title;
		editContent = doc.content;
		editProjectID = doc.project_id;
		isEditing = false;
	}

	function startEdit() {
		isEditing = true;
	}

	async function saveDocument() {
		if (!selectedDocId) {
			try {
				await postJSON("/v1/documents", {
					project_id: editProjectID,
					title: editTitle,
					content: editContent,
					format: "markdown",
					status: "active"
				});
				pushToast("Document created", "ok");
				isEditing = false;
			} catch (err: any) {
				pushToast(err.message, "err");
			}
			return;
		}

		try {
			await requestJSON(`/v1/documents/${selectedDocId}`, "PUT", {
				title: editTitle,
				content: editContent,
				project_id: editProjectID
			});
			pushToast("Document saved", "ok");
			isEditing = false;
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function buildDoc() {
		if (!selectedDocId) return;
		try {
			await postJSON(`/v1/documents/${selectedDocId}/build`, {});
			pushToast("Build loop started", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function archiveDoc() {
		if (!selectedDocId || !selectedDoc) return;
		const nextStatus = selectedDoc.status === 'active' ? 'archived' : 'active';
		try {
			await requestJSON(`/v1/documents/${selectedDocId}`, "PUT", { status: nextStatus });
			pushToast(`Document ${nextStatus}`, "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	async function deleteDoc() {
		if (!selectedDocId || !confirm("Delete document?")) return;
		try {
			await deleteJSON(`/v1/documents/${selectedDocId}`);
			selectedDocId = null;
			pushToast("Document deleted", "ok");
		} catch (err: any) {
			pushToast(err.message, "err");
		}
	}

	function createNew() {
		selectedDocId = null;
		editTitle = "Untitled Document";
		editContent = "";
		editProjectID = $appState.projects[0]?.id || "";
		isEditing = true;
	}

	function handleDraftFinalized(title: string, content: string) {
		editTitle = title;
		editContent = content;
		editProjectID = $appState.projects[0]?.id || "";
		selectedDocId = null;
		isEditing = true;
		chatOpen = false;
	}
</script>

<TopBar title="Documents">
  {#snippet controls()}
    <div class="flex items-center gap-4">
      <Checkbox bind:checked={showAll} class="text-gray-400 font-medium uppercase text-[10px] tracking-widest">Show Archived</Checkbox>
    </div>
  {/snippet}
</TopBar>

<!-- Inline Actions Header -->
<div class="flex justify-end gap-2 -mt-14 mb-8 relative z-50 px-4">
  <Button color="alternative" class="bg-black border-gray-800 text-[#86BC25] hover:bg-white/5 rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={() => chatOpen = true}>
    <MessagesOutline size="xs" class="mr-1.5" />
    Draft with AI
  </Button>
  
  <Button color="none" class="bg-[#86BC25] text-black rounded-none font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7" onclick={createNew}>
    <PlusOutline size="xs" class="mr-1.5" />
    New Doc
  </Button>
</div>

<DocChatModal
	open={chatOpen}
	onClose={() => chatOpen = false}
	onDraftFinalized={handleDraftFinalized}
/>

<div class="doc-layout">
	<!-- Left Sidebar: Document List -->
	<aside class="doc-list-sidebar">
		{#each projectIDs as pid}
			<div class="project-group">
				<div class="project-header">
					{pid}
				</div>
				<div class="project-docs">
					{#each projectsWithDocs[pid] as doc}
						<button 
							class="doc-item" 
							class:active={selectedDocId === doc.id}
							onclick={() => selectDocument(doc)}
						>
							<span class="doc-label">{doc.title || "Untitled"}</span>
						</button>
					{/each}
				</div>
			</div>
		{:else}
			<div class="empty-sidebar flex flex-col items-center justify-center gap-3 py-10 opacity-40">
        <FileLinesOutline size="lg" />
        <span class="uppercase text-[10px] font-bold tracking-widest">No documents found</span>
      </div>
		{/each}
	</aside>

	<!-- Right Pane: Editor / Preview -->
	<main class="doc-editor-pane">
		{#if isEditing || selectedDocId || editTitle}
			<div class="editor-frame">
				<div class="editor-top-bar px-8 py-6 border-b border-gray-900">
					{#if isEditing}
            <div class="flex items-center justify-between w-full gap-4">
              <div class="flex items-center gap-4 flex-1">
                <Input type="text" bind:value={editTitle} placeholder="Document Title" class="bg-black border-gray-800 text-white font-bold text-xl flex-1 rounded-none focus:border-[#86BC25] transition-all" />
                <Select bind:value={editProjectID} class="bg-black border-gray-800 text-white w-48 rounded-none">
                  {#each $appState.projects as p}
                    <option value={p.id}>{p.name}</option>
                  {/each}
                </Select>
              </div>
              <div class="flex gap-2">
                <Button color="none" class="bg-[#86BC25] text-black font-bold uppercase text-[9px] px-4 py-1 h-7 rounded-none" onclick={saveDocument}>
                  <CloudArrowUpOutline size="xs" class="mr-1.5" />
                  Save
                </Button>
                <Button color="alternative" class="border-gray-800 text-gray-400 font-bold uppercase text-[9px] px-4 py-1 h-7 rounded-none" onclick={() => isEditing = false}>
                  <CloseOutline size="xs" class="mr-1.5" />
                  Cancel
                </Button>
              </div>
            </div>
					{:else}
            <div class="flex items-center justify-between w-full">
              <div class="flex flex-col">
                <h1 class="title-display uppercase">{editTitle}</h1>
                <div class="mt-1 text-[10px] font-bold text-gray-600 tracking-[0.2em]">{editProjectID}</div>
              </div>
              <div class="flex gap-2">
                <Button color="alternative" class="border-gray-800 text-gray-400 hover:text-white rounded-none font-bold text-[9px] tracking-widest px-3 h-7" onclick={startEdit} title="Edit">
                  <EditOutline size="xs" class="mr-1.5" />
                  EDIT
                </Button>
                <Button color="none" class="bg-[#86BC25] text-black font-bold rounded-none text-[9px] tracking-widest px-3 h-7" onclick={buildDoc} title="Build">
                  <RocketOutline size="xs" class="mr-1.5" />
                  BUILD
                </Button>
                <Button color="alternative" class="border-gray-800 text-gray-400 hover:text-white rounded-none px-2 h-7" onclick={archiveDoc} title="Archive">
                  <ArchiveOutline size="xs" />
                </Button>
                <Button color="red" class="rounded-none border-none px-2 h-7" onclick={deleteDoc} title="Delete">
                  <TrashBinOutline size="xs" />
                </Button>
              </div>
            </div>
					{/if}
				</div>

				<div class="editor-viewport flex-1 flex">
          {#if isEditing}
            <!-- Side-by-Side Editor -->
            <div class="flex w-full h-full divide-x divide-gray-900 overflow-hidden">
              <div class="flex-1 flex flex-col bg-black">
                <div class="px-8 py-2 border-b border-gray-900 bg-slate-900/20 text-[10px] font-bold text-gray-500 uppercase tracking-widest">Editor</div>
                <Textarea 
                  bind:value={editContent} 
                  rows={20} 
                  placeholder="Start writing in Markdown..." 
                  class="bg-black border-none text-gray-300 font-mono text-lg focus:ring-0 resize-none h-full w-full rounded-none px-8 py-6"
                >
                  <Toolbar slot="header" embedded class="bg-black border-b border-gray-900 rounded-none px-8">
                    <ToolbarGroup>
                      <ToolbarButton name="Bold" class="text-gray-400 hover:text-white" onclick={() => editContent += '**bold**'}><span class="font-bold text-xs">B</span></ToolbarButton>
                      <ToolbarButton name="Italic" class="text-gray-400 hover:text-white" onclick={() => editContent += '_italic_'}><span class="italic text-xs">I</span></ToolbarButton>
                      <ToolbarButton name="List" class="text-gray-400 hover:text-white" onclick={() => editContent += '\n- '}><span class="text-xs">L</span></ToolbarButton>
                    </ToolbarGroup>
                  </Toolbar>
                </Textarea>
              </div>
              <div class="flex-1 flex flex-col overflow-hidden bg-black">
                <div class="px-8 py-2 border-b border-gray-900 bg-slate-900/20 text-[10px] font-bold text-gray-500 uppercase tracking-widest">Live Preview</div>
                <div class="markdown-preview prose prose-invert max-w-none h-full overflow-y-auto px-8 py-8">
                  {@html renderedContent}
                </div>
              </div>
            </div>
          {:else}
            <!-- Full Width Preview -->
            <div class="flex-1 flex flex-col bg-black">
              <div class="markdown-preview prose prose-invert max-w-none h-full overflow-y-auto px-8 py-8">
                {@html renderedContent}
              </div>
            </div>
          {/if}
				</div>
			</div>
		{:else}
			<EmptyState 
				title="" 
				description="Select a document from the left to view or edit."
				icon="📄"
			/>
		{/if}
	</main>
</div>

<style>
	.doc-layout {
		display: grid;
		grid-template-columns: 260px 1fr;
		height: calc(100vh - 160px);
		background: #000000;
		overflow: hidden;
    border: 1px solid rgba(255, 255, 255, 0.05);
	}

	.doc-list-sidebar {
		background: #000000;
		overflow-y: auto;
		padding: 20px 0;
		display: flex;
		flex-direction: column;
		gap: 24px;
    border-right: 1px solid rgba(255, 255, 255, 0.05);
	}

	.project-header {
		font-size: 0.6rem;
		text-transform: uppercase;
		letter-spacing: 0.2em;
		color: #5c6b7a;
		font-weight: 800;
		margin-bottom: 12px;
		padding-left: 24px;
	}

	.project-docs {
		display: flex;
		flex-direction: column;
	}

	.doc-item {
		background: transparent;
		border: none;
		display: flex;
		align-items: center;
		padding: 12px 24px;
		color: #90a1b7;
		font-size: 0.8rem;
    font-weight: bold;
    text-transform: uppercase;
    letter-spacing: 0.05em;
		text-align: left;
		cursor: pointer;
		transition: all 0.1s;
    @apply border-l-2 border-transparent;
	}

	.doc-item:hover {
		background: rgba(255, 255, 255, 0.03);
		color: #fff;
	}

	.doc-item.active {
		background: rgba(134, 188, 37, 0.05);
		color: #86BC25;
    @apply border-l-2 border-[#86BC25];
	}

	.doc-editor-pane {
		background: #000000;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		position: relative;
	}

	.editor-frame {
		display: flex;
		flex-direction: column;
		height: 100%;
	}

	.title-display {
		margin: 0;
		font-size: 1.5rem;
		font-weight: 800;
		color: #ffffff;
		letter-spacing: -0.02em;
	}

	.editor-viewport {
		flex: 1;
		overflow: hidden;
		display: flex;
	}

  .markdown-preview {
    color: #d1d7e0;
    line-height: 1.8;
    font-size: 1.1rem;
  }

  :global(.markdown-preview h1) { font-size: 2rem; border-bottom: 1px solid #30363d; padding-bottom: 0.3em; margin-top: 24px; margin-bottom: 16px; font-weight: 600; color: #fff; }
  :global(.markdown-preview h2) { font-size: 1.5rem; border-bottom: 1px solid #30363d; padding-bottom: 0.3em; margin-top: 24px; margin-bottom: 16px; font-weight: 600; color: #fff; }
  :global(.markdown-preview p) { margin-top: 0; margin-bottom: 16px; }
  :global(.markdown-preview ul) { padding-left: 2em; margin-bottom: 16px; list-style-type: disc; }
  :global(.markdown-preview code) { padding: 0.2em 0.4em; margin: 0; font-size: 85%; background-color: rgba(110, 118, 129, 0.4); border-radius: 0px; font-family: var(--mono); }
  :global(.markdown-preview pre) { padding: 16px; overflow: auto; font-size: 85%; line-height: 1.45; background-color: #111; border-radius: 0px; border: 1px solid #222; margin-bottom: 16px; }
  :global(.markdown-preview pre code) { background-color: transparent; padding: 0; font-size: 100%; }

	.empty-sidebar {
		padding: 40px 20px;
		text-align: center;
		color: #5c6b7a;
		font-size: 0.85rem;
	}
</style>
