<script lang="ts">
	import { escapeHtml } from '$lib/utils';
	import { goto } from '$app/navigation';

	interface Props {
		loop: any;
		selected: boolean;
		onSelect: (id: string) => void;
	}

	let { loop, selected, onSelect }: Props = $props();
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<article 
	class="pod-tile" 
	class:selected 
	role="listitem" 
	onclick={() => goto(`/pod-view/${loop.loopID}`)}
>
	<div class="tile-head">
		<div class="tile-title loop-id">{loop.loopID}</div>
	</div>
	<div class="tile-loop">{loop.project}</div>
	<div class="tile-reason">{loop.reason || "no recent update"}</div>
	<div class="tile-footer">
		<span class="badge state-{loop.status}">{loop.status}</span>
		<div class="tile-meta">
			<span>ATT {loop.attempt}</span>
			<span>REV {loop.revision}</span>
		</div>
	</div>
</article>
