<script lang="ts">
	import { onMount } from 'svelte';
	import { appState, pushToast } from '$lib/stores';
	import { fetchJSON, deleteJSON, requestJSON, postJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import DocTile from '$lib/components/DocTile.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import DocEditorModal from '$lib/components/DocEditorModal.svelte';
	import DocChatModal from '$lib/components/DocChatModal.svelte';
	import { escapeHtml } from '$lib/utils';

	let showAll = $state(false);
	
	let editorOpen = $state(false);
	let editingDocId = $state<string | null>(null);
	let editTitle = $state("");
	let editContent = $state("");

	let chatOpen = $state(false);

	const filteredDocs = $derived(
		$appState.documents.filter((d: any) => {
			if (!d) return false;
			const query = $appState.docSearchQuery.toLowerCase();
			const matchesQuery = (d.title || "").toLowerCase().includes(query) || 
								(d.id || "").toLowerCase().includes(query) ||
								(d.project_id || "").toLowerCase().includes(query);
			const matchesProject = $appState.docFilterProject === "all" || $appState.docFilterProject === "" || d.project_id === $appState.docFilterProject;
			// If not showing all, only show documents that are "active" (yet to be built)
			const matchesStatus = showAll || d.status === "active" || d.status === "unresolved";
			return matchesQuery && matchesProject && matchesStatus;
		})
	);

	const projects = $derived(
		Array.from(new Set($appState.documents.map((d: any) => d.project_id).filter(Boolean))).sort()
	);

	const groupedDocs = $derived.by(() => {
		if ($appState.docFilterProject !== "all" && $appState.docFilterProject !== "") {
			return { [$appState.docFilterProject]: filteredDocs };
		}
		const grouped: any = {};
		filteredDocs.forEach((doc: any) => {
			if (!doc.project_id) return;
			if (!grouped[doc.project_id]) grouped[doc.project_id] = [];
			grouped[doc.project_id].push(doc);
		});
		return grouped;
	});

	const sortedProjectIDs = $derived(Object.keys(groupedDocs).sort());

	function setFilterProject(p: string) {
		appState.update(s => ({ ...s, docFilterProject: p }));
	}

	async function archiveDocument(id: string) {
		const doc = $appState.documents.find((d: any) => d.id === id);
		if (!doc) return;
		const nextStatus = doc.status === "active" ? "archived" : "active";
		try {
			await requestJSON("/v1/documents/" + id, "PUT", { status: nextStatus });
		} catch (err: any) {
			pushToast("Error archiving document: " + err.message, "err");
		}
	}

	async function deleteDocument(id: string) {
		if (!confirm("Delete this document?")) return;
		try {
			await deleteJSON("/v1/documents/" + id);
		} catch (err: any) {
			pushToast("Error deleting document: " + err.message, "err");
		}
	}

	async function buildDocument(id: string) {
		try {
			await postJSON("/v1/documents/" + id + "/build", {});
			pushToast("Build loop instantiated for document: " + id, "ok");
		} catch (err: any) {
			pushToast("Error building document: " + err.message, "err");
		}
	}

	function editDocument(id: string) {
		const doc = $appState.documents.find((d: any) => d.id === id);
		if (!doc) return;
		editingDocId = id;
		editTitle = doc.title || "";
		editContent = doc.content || "";
		editorOpen = true;
	}

	function createNewDocument() {
		editingDocId = null;
		editTitle = "";
		editContent = "";
		editorOpen = true;
	}

	function handleEditorSave(title: string, content: string) {
		if (editingDocId) {
			requestJSON("/v1/documents/" + editingDocId, "PUT", { title, content });
		} else {
			// New document creation
			const projectID = $appState.docFilterProject === "all" || $appState.docFilterProject === "" 
				? ($appState.projects[0]?.id || "default") 
				: $appState.docFilterProject;
			
			postJSON("/v1/documents", {
				project_id: projectID,
				title: title || "New Document",
				content: content,
				format: "markdown",
				status: "active",
				source_type: "direct"
			}).then(() => {
				pushToast("Document created", "ok");
			}).catch(err => {
				pushToast(err.message, "err");
			});
		}
		editorOpen = false;
	}

	function handleDraftFinalized(title: string, content: string) {
		editTitle = title;
		editContent = content;
		editingDocId = null;
		editorOpen = true; // Open editor for final review
	}
</script>

{#snippet controls()}
	<input 
		type="search" 
		placeholder="Filter documents" 
		value={$appState.docSearchQuery}
		oninput={(e) => appState.update(s => ({ ...s, docSearchQuery: e.currentTarget.value }))}
	/>
	<label class="muted" style="display: flex; align-items: center; gap: 4px; cursor: pointer;">
		<input type="checkbox" bind:checked={showAll} /> Show All
	</label>
	<button class="primary" onclick={() => chatOpen = true} style="margin-left: 8px;">Draft with AI</button>
	<button onclick={createNewDocument}>New Doc</button>
{/snippet}

<TopBar title="Documents" {controls} />

<DocEditorModal 
	bind:open={editorOpen} 
	bind:title={editTitle}
	bind:content={editContent}
	onClose={() => editorOpen = false} 
	onSave={handleEditorSave} 
/>

<DocChatModal
	open={chatOpen}
	onClose={() => chatOpen = false}
	onDraftFinalized={handleDraftFinalized}
/>

<div class="doc-container">
	<aside id="doc-sidebar" class="doc-sidebar">
		<div class="doc-sidebar-section">
			<div class="doc-sidebar-header">Projects</div>
			<div class="doc-sidebar-list">
				<button 
					class="doc-sidebar-item" 
					class:active={$appState.docFilterProject === "all"}
					onclick={() => setFilterProject("all")}
				>All Projects</button>
				{#if projects.length === 0}
					<div class="doc-sidebar-item muted">📁 (Empty)</div>
				{:else}
					{#each projects as p}
						<button 
							class="doc-sidebar-item" 
							class:active={$appState.docFilterProject === p}
							onclick={() => setFilterProject(p)}
						>{p}</button>
					{/each}
				{/if}
			</div>
		</div>
	</aside>
	<main class="doc-main">
		{#if $appState.docFilterProject === ""}
			<EmptyState 
				title="Document Explorer" 
				description="Select a project from the sidebar to view associated Product Requirement Documents (PRDs) and technical specs."
				icon="🔍"
			/>
		{:else if sortedProjectIDs.length === 0}
			<EmptyState 
				title="No Documents Found" 
				description="There are no documents matching your current filters for this project."
				buttonText="Draft with AI"
				onclick={() => chatOpen = true}
				icon="📄"
			/>
		{:else}
			<div class="project-loop-list">
				{#each sortedProjectIDs as projectID}
					<details class="project-tile" open>
						<summary class="collapsible-summary">
							<span class="collapsible-label">
								<span class="collapsible-caret">&gt;</span>
								<span class="project-name">{projectID}</span>
							</span>
						</summary>
						<div class="collapsible-body">
							<div class="pod-grid">
								{#each groupedDocs[projectID] as doc (doc.id)}
									<DocTile 
										{doc}
										onEdit={editDocument}
										onBuild={buildDocument}
										onArchive={archiveDocument}
										onDelete={deleteDocument}
									/>
								{/each}
							</div>
						</div>
					</details>
				{/each}
			</div>
		{/if}
	</main>
</div>

<style>
	.doc-sidebar-item {
		background: transparent;
		border: none;
		text-align: left;
		width: 100%;
		display: flex;
		align-items: center;
		padding: 8px 12px;
		border-radius: 6px;
		color: var(--muted);
		cursor: pointer;
		transition: all 0.2s;
	}
	.doc-sidebar-item:hover {
		background: rgba(255, 255, 255, 0.05);
		color: var(--text);
	}
	.doc-sidebar-item.active {
		background: var(--panel-strong);
		color: var(--accent);
		font-weight: 600;
	}
	.project-loop-list {
		display: grid;
		gap: 12px;
	}
</style>
