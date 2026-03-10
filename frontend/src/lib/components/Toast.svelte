<script lang="ts">
	import { toastMessages } from '$lib/stores';
</script>

<div id="toast-region" class="toast-region" aria-live="polite" aria-atomic="false">
	{#each $toastMessages as toast (toast.id)}
		<div class="toast {toast.level}" class:show={toast.show}>
			{toast.message}
		</div>
	{/each}
</div>
