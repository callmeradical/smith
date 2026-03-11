<script lang="ts">
	import { onMount } from 'svelte';
	import { appState, pushToast } from '$lib/stores';
	import TopBar from '$lib/components/TopBar.svelte';
	import ProjectEditorDrawer from '$lib/components/ProjectEditorDrawer.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';

	let editorOpen = $state(false);
	let projectToEdit = $state<any>(null);

	function openNewProject() {
		projectToEdit = null;
		editorOpen = true;
	}

	function openEditProject(project: any) {
		projectToEdit = project;
		editorOpen = true;
	}
</script>

{#snippet controls()}
	<button class="icon-button" onclick={openNewProject}>+</button>
{/snippet}

<TopBar title="Projects" {controls} />

<ProjectEditorDrawer open={editorOpen} onClose={() => editorOpen = false} {projectToEdit} />

<section id="project-list-panel">
	<div class="project-list">
		{#if $appState.projects.length === 0}
			<EmptyState 
				title="No Projects Configured" 
				description="Projects define the repositories and environments for your autonomous loops. Create one to get started."
				buttonText="Create Project"
				buttonHref="#"
				icon="🏗️"
			/>
		{:else}
			{#each $appState.projects as project (project.id)}
				<details class="project-tile" open>
					<summary class="collapsible-summary">
						<span class="collapsible-label">
							<span class="collapsible-caret">&gt;</span>
							<span class="project-name">{project.name}</span>
						</span>
					</summary>
					<div class="collapsible-body">
						<div class="project-repo">{project.repo_url}</div>
						<div class="project-card-actions">
							<button type="button" class="project-action-icon" onclick={() => openEditProject(project)}>&#9998;</button>
						</div>
					</div>
				</details>
			{/each}
		{/if}
	</div>
</section>
