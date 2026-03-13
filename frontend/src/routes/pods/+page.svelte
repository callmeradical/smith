<script lang="ts">
	import { onMount } from 'svelte';
	import { appState } from '$lib/stores';
	import { fetchJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import PodTile from '$lib/components/PodTile.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
  import { Select, Input, Label, Button } from 'flowbite-svelte';
  import { GridOutline } from 'flowbite-svelte-icons';

	let stateFilter = $state('all');
	let searchQuery = $state('');

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

	onMount(() => {
		// Data is populated by the global stream in layout
	});

	const filteredLoops = $derived(
		$appState.loops.filter((loop: any) => {
			const matchesState =
				stateFilter === "all" ||
				(stateFilter === "active" && (loop.status === "unresolved" || loop.status === "running")) ||
				loop.status === stateFilter;
			const matchesSearch = !searchQuery || String(loop.loopID).toLowerCase().includes(searchQuery.toLowerCase());
			return matchesState && matchesSearch;
		})
	);

	const stats = $derived({
		total: $appState.loops.length,
		active: $appState.loops.filter((l: any) => l.status === "unresolved" || l.status === "running").length,
		flatline: $appState.loops.filter((l: any) => l.status === "flatline").length
	});

	function selectLoop(id: string) {
		appState.update(s => ({ ...s, selectedLoop: id }));
	}
</script>

<TopBar title="Pods" />

<div class="flex items-center justify-start gap-4 -mt-14 mb-8 relative z-50 px-4 ml-auto w-fit">
  <div class="flex items-center gap-2">
    <div class="w-32">
      <Select bind:value={stateFilter} size="sm" class="bg-black border-gray-800 text-gray-400 text-[10px] uppercase font-bold rounded-none h-7">
        <option value="all">All States</option>
        <option value="active">Active Only</option>
        <option value="unresolved">Unresolved</option>
        <option value="running">Running</option>
        <option value="synced">Synced</option>
        <option value="flatline">Flatline</option>
        <option value="cancelled">Cancelled</option>
      </Select>
    </div>
    
    <div class="w-48">
      <Input 
        type="search" 
        placeholder="Filter ID..." 
        bind:value={searchQuery} 
        size="sm" 
        class="bg-black border-gray-800 text-white text-[10px] uppercase font-bold rounded-none h-7 px-3"
      />
    </div>
  </div>
</div>

<div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8 px-4">
  <div class="bg-slate-900/20 border border-gray-900 rounded-none p-5 flex flex-col gap-1 backdrop-blur-sm">
    <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Total Pods</span>
    <span class="text-3xl font-bold text-white tracking-tighter">{stats.total}</span>
  </div>
  <div class="bg-slate-900/20 border border-gray-900 rounded-none p-5 flex flex-col gap-1 backdrop-blur-sm">
    <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Active Loops</span>
    <span class="text-3xl font-bold text-[#86BC25] tracking-tighter">{stats.active}</span>
  </div>
  <div class="bg-slate-900/20 border border-gray-900 rounded-none p-5 flex flex-col gap-1 backdrop-blur-sm">
    <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-600">Flatline</span>
    <span class="text-3xl font-bold text-rose-600 tracking-tighter">{stats.flatline}</span>
  </div>
</div>

<section class="tiles-shell px-4">
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4" role="list">
    {#if $appState.projects.length === 0}
      <div class="col-span-full py-12">
        <EmptyState 
          title="Welcome to SMITH" 
          description="To get started, you'll need to configure a project. Projects connect your repositories and enable autonomous development loops."
          buttonText="Configure Project"
          buttonHref="/projects"
          icon="🚀"
        />
      </div>
    {:else}
      {#each filteredLoops as loop (loop.loopID)}
        <PodTile 
          {loop} 
          selected={$appState.selectedLoop === loop.loopID} 
          onSelect={selectLoop} 
        />
      {:else}
        <div class="col-span-full py-20 bg-slate-900/10 border border-dashed border-gray-900 rounded-none flex flex-col items-center justify-center text-gray-600">
          <GridOutline size="xl" class="mb-4 opacity-20" />
          <p class="text-sm uppercase font-bold tracking-[0.2em]">No pods found matching filters.</p>
        </div>
      {/each}
    {/if}
  </div>
</section>
