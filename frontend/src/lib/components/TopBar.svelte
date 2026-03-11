<script lang="ts">
	import { sidebarOpen } from '$lib/stores';
  import { page } from '$app/state';
  import { Button, Navbar, NavBrand, NavUl, NavLi } from 'flowbite-svelte';
  import { BarsOutline, GridOutline, FileLinesOutline, ArchiveOutline, UsersGroupOutline } from 'flowbite-svelte-icons';

	interface Props {
		title: string;
	}

	let { title }: Props = $props();

  const navItems = [
		{ id: 'pods', label: 'Pods', href: '/pods', icon: GridOutline },
		{ id: 'documents', label: 'Documents', href: '/documents', icon: FileLinesOutline },
		{ id: 'projects', label: 'Projects', href: '/projects', icon: ArchiveOutline },
		{ id: 'providers', label: 'Providers', href: '/providers', icon: UsersGroupOutline }
	];

  const currentPath = $derived(page.url.pathname);
</script>

<Navbar fluid class="bg-black border-b border-gray-800 px-4 py-2 sticky top-0 z-40">
  <NavBrand href="/">
    <div class="flex items-center gap-3">
      <span id="api-dot" class="dot w-3 h-3 rounded-full bg-[#86BC25] shadow-[0_0_10px_rgba(134,188,37,0.8)]" aria-hidden="true"></span>
      <span class="text-xl font-bold tracking-tighter text-white uppercase font-sans">SMITH</span>
    </div>
  </NavBrand>

  <div class="flex items-center gap-2 lg:order-2">
    <Button
      color="none"
      class="p-2 text-[#86BC25] hover:bg-white/5 transition-colors lg:hidden"
      onclick={() => sidebarOpen.update(v => !v)}
      aria-label="Toggle Sidebar"
    >
      <BarsOutline size="md" />
    </Button>
  </div>

  <NavUl class="hidden lg:flex lg:gap-1" ulClass="flex flex-row space-x-1 mt-0 bg-transparent border-0">
    {#each navItems as item}
      {@const active = currentPath.startsWith(item.href)}
      <NavLi 
        href={item.href} 
        {active}
        activeClass="text-white border-b-2 border-[#86BC25] bg-transparent"
        nonActiveClass="text-gray-400 hover:text-[#86BC25] bg-transparent"
        class="px-4 py-3 transition-all hover:bg-transparent"
      >
        <div class="flex items-center gap-2 uppercase tracking-widest text-[10px] font-bold">
          <item.icon size="sm" class={active ? 'text-[#86BC25]' : 'text-gray-500'} />
          {item.label}
        </div>
      </NavLi>
    {:else}
      <!-- no items -->
    {/each}
  </NavUl>
</Navbar>

<div class="page-header py-6 flex items-center justify-between px-4">
  <h1 class="text-2xl font-bold text-white tracking-tight uppercase border-l-4 border-[#86BC25] pl-4">{title}</h1>
  <div id="page-actions" class="flex items-center gap-2">
    <!-- Buttons will be injected here via portal or component logic -->
  </div>
</div>

<style>
  :global(.navbar-ul) {
    background: transparent !important;
  }
  :global(.navbar-ul li a:hover) {
    background-color: transparent !important;
  }
</style>
