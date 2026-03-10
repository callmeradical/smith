<script lang="ts">
	import { onMount } from 'svelte';
	import { appState, pushToast } from '$lib/stores';
	import TopBar from '$lib/components/TopBar.svelte';
	import { escapeHtml } from '$lib/utils';

	function openProjectEditor() {
		// Logic to open editor drawer
		pushToast("Project editor coming soon", "muted");
	}
</script>

{#snippet controls()}
	<button class="icon-button" onclick={openProjectEditor}>+</button>
{/snippet}

<TopBar title="Projects" {controls} />

<section id="project-list-panel">
	<div class="project-list">
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
						<button type="button" class="project-action-icon">&#9998;</button>
					</div>
				</div>
			</details>
		{:else}
			<div class="status-note">No projects configured yet.</div>
		{/each}
	</div>
</section>
