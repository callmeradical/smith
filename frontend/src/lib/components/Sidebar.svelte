<script lang="ts">
	import { sidebarOpen } from '$lib/stores';
	import { page } from '$app/state';

	const navItems = [
		{ id: 'pods', label: 'Pods', href: '/pods' },
		{ id: 'documents', label: 'Documents', href: '/documents' },
		{ id: 'projects', label: 'Projects', href: '/projects' },
		{ id: 'providers', label: 'Providers', href: '/providers' },
		{ id: 'controls', label: 'Manual Controls', href: '/controls' }
	];
</script>

<aside class="sidebar" class:open={$sidebarOpen} aria-label="Console sidebar">
	<section class="brand">
		<div class="brand-line">
			<span id="api-dot" class="dot" aria-hidden="true"></span>
			<span>SMITH</span>
		</div>
		<div class="api-url" id="api-url">API URL</div>
	</section>

	<nav class="quick-nav" aria-label="Configuration pages">
		{#each navItems as item}
			<a
				class="nav-link"
				class:active={page.url.pathname.startsWith(item.href)}
				href={item.href}
				onclick={() => sidebarOpen.set(false)}
			>
				{item.label}
			</a>
		{/each}
	</nav>
</aside>

<style>
	.sidebar {
		transform: translateX(-120%);
		transition: transform 180ms ease;
	}

	.sidebar.open {
		transform: translateX(0);
	}
</style>
