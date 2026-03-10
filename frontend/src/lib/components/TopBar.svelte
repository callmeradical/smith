<script lang="ts">
	import { sidebarOpen } from '$lib/stores';
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		controls?: Snippet;
	}

	let { title, controls }: Props = $props();
</script>

<header class="topbar">
	<div class="topbar-left">
		<button
			type="button"
			class="sidebar-toggle"
			aria-label="Toggle sidebar"
			aria-expanded={$sidebarOpen}
			onclick={() => sidebarOpen.update(v => !v)}
		>
			&#9776;
		</button>
		<div class="topbar-title">{title}</div>
	</div>
	<div class="controls">
		{#if controls}
			{@render controls()}
		{/if}
	</div>
</header>
