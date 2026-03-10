<script lang="ts">
	import '../app.css';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Toast from '$lib/components/Toast.svelte';
	import { sidebarOpen, appState } from '$lib/stores';
	import { onMount } from 'svelte';
	import { fetchJSON } from '$lib/api';

	let { children } = $props();

	async function initApp() {
		try {
			const projects = await fetchJSON("/v1/projects");
			appState.update(s => ({ ...s, projects: Array.isArray(projects) ? projects : [] }));
		} catch (err) {
			console.error("Failed to load projects", err);
		}
	}

	onMount(() => {
		initApp();
	});
</script>

<div id="sidebar-overlay" class="sidebar-overlay" class:open={$sidebarOpen} aria-hidden="true" onclick={() => sidebarOpen.set(false)}></div>
<div id="provider-drawer-overlay" class="provider-drawer-overlay" aria-hidden="true"></div>

<Toast />

<div class="shell">
	<Sidebar />

	<main class="workspace">
		{@render children()}
	</main>
</div>

<style>
	.sidebar-overlay.open {
		display: block;
	}
</style>
