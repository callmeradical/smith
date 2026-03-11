<script lang="ts">
	import { onMount } from 'svelte';
	import { appState, pushToast } from '$lib/stores';
	import { fetchJSON, deleteJSON, requestJSON, postJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import DocTile from '$lib/components/DocTile.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import { escapeHtml } from '$lib/utils';

	async function refreshDocuments() {
		try {
			const raw = await fetchJSON("/v1/documents");
			appState.update(s => ({ ...s, documents: Array.isArray(raw) ? raw : [] }));
		} catch (err) {
			console.error("Error fetching documents:", err);
		}
	}

	onMount(() => {
		refreshDocuments();
	});

	const filteredDocs = $derived(
		$appState.documents.filter((d: any) => {
			if (!d) return false;
			const query = $appState.docSearchQuery.toLowerCase();
			const matchesQuery = (d.title || "").toLowerCase().includes(query) || 
								(d.id || "").toLowerCase().includes(query) ||
								(d.project_id || "").toLowerCase().includes(query);
			const matchesProject = $appState.docFilterProject === "all" || $appState.docFilterProject === "" || d.project_id === $appState.docFilterProject;
			const matchesStatus = $appState.docFilterStatus === "all" || d.status === $appState.docFilterStatus;
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

	function setFilterStatus(s: string) {
		appState.update(s => ({ ...s, docFilterStatus: s }));
	}

	async function archiveDocument(id: string) {
		const doc = $appState.documents.find((d: any) => d.id === id);
		if (!doc) return;
		const nextStatus = doc.status === "active" ? "archived" : "active";
		try {
			await requestJSON("/v1/documents/" + id, "PUT", { status: nextStatus });
			refreshDocuments();
		} catch (err: any) {
			pushToast("Error archiving document: " + err.message, "err");
		}
	}

	async function deleteDocument(id: string) {
		if (!confirm("Delete this document?")) return;
		try {
			await deleteJSON("/v1/documents/" + id);
			refreshDocuments();
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
		const nextContent = prompt("Edit Document Content", doc.content);
		if (nextContent === null) return;
		requestJSON("/v1/documents/" + id, "PUT", { content: nextContent }).then(() => refreshDocuments());
	}
</script>

{#snippet controls()}
	<input 
		type="search" 
		placeholder="Filter documents" 
		value={$appState.docSearchQuery}
		oninput={(e) => appState.update(s => ({ ...s, docSearchQuery: e.currentTarget.value }))}
	/>
	<button class="primary">New Document</button>
{/snippet}

<TopBar title="Documents" {controls} />

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
		<div class="doc-sidebar-section">
			<div class="doc-sidebar-header">Status</div>
			<div class="doc-sidebar-list">
				{#each ["all", "active", "archived"] as s}
					<button 
						class="doc-sidebar-item" 
						class:active={$appState.docFilterStatus === s}
						onclick={() => setFilterStatus(s)}
					>{s.charAt(0).toUpperCase() + s.slice(1)}</button>
				{/each}
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
				buttonText="Create Document"
				buttonHref="#"
				icon="📄"
			/>
		{:else}
			<div class="pod-grid">
				{#each sortedProjectIDs as projectID}
					<section class="pod-group" style="width: 100%; grid-column: 1 / -1;">
						<div class="doc-sidebar-header" style="margin-bottom: 12px;">Project: {projectID}</div>
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
					</section>
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
	}
</style>
