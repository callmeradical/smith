<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import TopBar from '$lib/components/TopBar.svelte';

	let overrideState = $state('unresolved');
	let reason = $state('');
	let actor = $state('operator');
	let confirmText = $state('');

	function applyOverride() {
		if (confirmText !== 'APPLY') {
			pushToast("Type APPLY to confirm", "err");
			return;
		}
		pushToast("Override logic coming soon", "muted");
	}
</script>

<TopBar title="Manual Controls" />

<section class="panel">
	<div class="panel-title">
		<span>Loop Override</span>
	</div>
	<div class="status-note">selected loop: {$appState.selectedLoop || "--"}</div>
	<select bind:value={overrideState}>
		<option value="unresolved">unresolved</option>
		<option value="overwriting">overwriting</option>
		<option value="synced">synced</option>
		<option value="flatline">flatline</option>
		<option value="cancelled">cancelled</option>
	</select>
	<input type="text" placeholder="override reason (required)" bind:value={reason} />
	<input type="text" placeholder="actor (default: operator)" bind:value={actor} />
	<input type="text" placeholder="type APPLY to confirm" bind:value={confirmText} />
	<div class="override-controls">
		<button class="danger" onclick={applyOverride}>apply override</button>
	</div>
</section>
