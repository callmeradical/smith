<script lang="ts">
	import { onMount } from 'svelte';
	import { appState } from '$lib/stores';
	import { fetchJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import PodTile from '$lib/components/PodTile.svelte';
	import PodCreateModal from '$lib/components/PodCreateModal.svelte';

	let stateFilter = $state('all');
	let searchQuery = $state('');
	let autoRefresh = $state(true);
	let modalOpen = $state(false);

	function normalizeLoop(item: any) {
		const record = item.record || item.Record || {};
		const loopID = record.loop_id || record.LoopID || item.loop_id || "unknown-loop";
		const status = (record.state || record.State || "unknown").toLowerCase();
		const attempt = Number(record.attempt || record.Attempt || 0);
		const reason = record.reason || record.Reason || "";
		const revision = Number(item.revision || item.Revision || record.observed_revision || 0);
		return {
			loopID,
			project: record.project_id || record.project || record.project_name || "default",
			status,
			attempt,
			reason,
			revision,
		};
	}

	async function refreshLoops() {
		try {
			const raw = await fetchJSON("/v1/loops");
			const loops = Array.isArray(raw) ? raw.map(normalizeLoop) : [];
			appState.update(s => ({ ...s, loops }));
		} catch (err) {
			console.error("Failed to refresh loops", err);
		}
	}

	onMount(() => {
		refreshLoops();
		const interval = setInterval(() => {
			if (autoRefresh) refreshLoops();
		}, 5000);
		return () => clearInterval(interval);
	});

	const filteredLoops = $derived(
		$appState.loops.filter((loop: any) => {
			const matchesState =
				stateFilter === "all" ||
				(stateFilter === "active" && (loop.status === "unresolved" || loop.status === "overwriting")) ||
				loop.status === stateFilter;
			const matchesSearch = !searchQuery || String(loop.loopID).toLowerCase().includes(searchQuery.toLowerCase());
			return matchesState && matchesSearch;
		})
	);

	const stats = $derived({
		total: $appState.loops.length,
		active: $appState.loops.filter((l: any) => l.status === "unresolved" || l.status === "overwriting").length,
		flatline: $appState.loops.filter((l: any) => l.status === "flatline").length
	});

	function selectLoop(id: string) {
		appState.update(s => ({ ...s, selectedLoop: id }));
	}
</script>

{#snippet controls()}
	<select bind:value={stateFilter} aria-label="State filter">
		<option value="all">All States</option>
		<option value="active">Active Only</option>
		<option value="unresolved">Unresolved</option>
		<option value="overwriting">Overwriting</option>
		<option value="synced">Synced</option>
		<option value="flatline">Flatline</option>
		<option value="cancelled">Cancelled</option>
	</select>
	<input type="search" placeholder="Filter loop id" bind:value={searchQuery} />
	<label class="muted">
		<input type="checkbox" bind:checked={autoRefresh} /> auto-refresh
	</label>
	<button onclick={refreshLoops}>refresh</button>
{/snippet}

<TopBar title="Pods" {controls} />

<section class="stats">
	<div class="stat"><small>Total</small><strong>{stats.total}</strong></div>
	<div class="stat"><small>Active</small><strong>{stats.active}</strong></div>
	<div class="stat"><small>Flatline</small><strong>{stats.flatline}</strong></div>
	<div class="stat stat-action">
		<small>New Loop</small>
		<button type="button" class="stat-add-button" aria-label="Start loop" onclick={() => modalOpen = true}>+</button>
	</div>
</section>

<PodCreateModal open={modalOpen} onClose={() => modalOpen = false} />

<section class="board">
	<section class="tiles-shell">
		<div class="pod-grid" role="list">
			{#if $appState.projects.length === 0}
				<div class="empty">
					<p>No projects configured yet.</p>
					<p><a href="/projects" class="nav-link" style="display: inline; padding: 0; color: var(--accent); text-decoration: underline;">Configure a project</a> to start creating loops.</p>
				</div>
			{:else}
				{#each filteredLoops as loop (loop.loopID)}
					<PodTile 
						{loop} 
						selected={$appState.selectedLoop === loop.loopID} 
						onSelect={selectLoop} 
					/>
				{:else}
					<div class="empty">No pods found.</div>
				{/each}
			{/if}
		</div>
	</section>
</section>
