<script lang="ts">
	import { state, pushToast } from '$lib/stores';
	import TopBar from '$lib/components/TopBar.svelte';
	import { providerCatalog } from '$lib/constants'; // I'll port this later

	const providers = [
		{ id: "codex", label: "OpenAI Codex CLI", subtitle: "Execute OpenAI models via Codex CLI." },
		// Add others as needed for simulation
	];
</script>

<TopBar title="Providers" />

<section id="provider-list-panel">
	<div class="provider-card-grid">
		{#each providers as provider}
			<article class="provider-card">
				<div class="provider-card-head">
					<span class="provider-card-name">{provider.label}</span>
				</div>
				<div class="provider-card-desc">{provider.subtitle}</div>
				<div class="provider-card-actions">
					<button type="button" class={provider.id === 'codex' ? 'primary' : ''}>configure</button>
					<span class="provider-card-status">available</span>
				</div>
			</article>
		{/each}
	</div>
</section>
