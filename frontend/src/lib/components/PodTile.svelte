<script lang="ts">
	import { escapeHtml } from '$lib/utils';
	import { goto } from '$app/navigation';
  import { Card, Badge } from 'flowbite-svelte';

	interface Props {
		loop: any;
		selected: boolean;
		onSelect: (id: string) => void;
	}

	let { loop, selected, onSelect }: Props = $props();
  
  const statusColor = $derived.by(() => {
    switch(loop.status) {
      case 'synced': return 'green';
      case 'overwriting':
      case 'unresolved': return 'yellow';
      case 'flatline':
      case 'cancelled': return 'red';
      default: return 'dark';
    }
  });
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div 
	class="pod-card-container group" 
  role="button"
  tabindex="0"
	onclick={() => goto(`/pod-view/${loop.loopID}`)}
>
  <Card class="bg-slate-900/40 border-gray-800 hover:border-cyan-500/50 transition-all cursor-pointer h-full backdrop-blur-sm p-4 {selected ? 'ring-1 ring-cyan-500 border-cyan-500' : ''}">
    <div class="flex flex-col h-full gap-3">
      <div class="flex justify-between items-start gap-2">
        <div class="truncate text-sm font-mono font-bold text-gray-200 group-hover:text-cyan-400 transition-colors" title={loop.loopID}>
          {loop.loopID}
        </div>
        <Badge color={statusColor} rounded class="uppercase text-[10px] px-2 py-0.5 font-bold">
          {loop.status}
        </Badge>
      </div>

      <div class="flex items-center gap-2 text-xs font-semibold text-gray-500 uppercase tracking-wider">
        <span class="w-2 h-2 rounded-full bg-gray-700"></span>
        {loop.project}
      </div>

      <div class="text-xs text-gray-400 line-clamp-2 min-h-[32px] leading-relaxed">
        {loop.reason || "No recent updates available"}
      </div>

      <div class="mt-auto pt-3 border-t border-gray-800/50 flex justify-between items-center text-[10px] font-mono text-gray-500">
        <div class="flex gap-3">
          <span>ATT <span class="text-gray-300 font-bold">{loop.attempt}</span></span>
          <span>REV <span class="text-gray-300 font-bold">{loop.revision}</span></span>
        </div>
        <div class="opacity-0 group-hover:opacity-100 transition-opacity text-cyan-500 font-bold">
          VIEW &rarr;
        </div>
      </div>
    </div>
  </Card>
</div>

<style>
  :global(.pod-card-container .p-4) {
    padding: 1rem !important;
  }
</style>
