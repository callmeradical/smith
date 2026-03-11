<script lang="ts">
	import { sidebarOpen, chatOpen } from '$lib/stores';
	import { page } from '$app/state';
  import { Sidebar, SidebarGroup, SidebarItem, SidebarWrapper, Drawer } from 'flowbite-svelte';
  import { GridOutline, FileLinesOutline, ArchiveOutline, UsersGroupOutline, CloseOutline, MessagesOutline } from 'flowbite-svelte-icons';
  import { sineIn } from 'svelte/easing';

	const navItems = [
		{ id: 'pods', label: 'Pods', href: '/pods', icon: GridOutline },
		{ id: 'documents', label: 'Documents', href: '/documents', icon: FileLinesOutline },
		{ id: 'projects', label: 'Projects', href: '/projects', icon: ArchiveOutline },
		{ id: 'providers', label: 'Providers', href: '/providers', icon: UsersGroupOutline }
	];

  let transitionParams = {
    x: -320,
    duration: 200,
    easing: sineIn
  };

  const currentPath = $derived(page.url.pathname);
</script>

<Drawer 
  transitionType="fly" 
  {transitionParams} 
  bind:open={$sidebarOpen} 
  id="sidebar-drawer" 
  width="w-64" 
  class="bg-black border-r border-gray-800 p-0 z-50"
>
  <SidebarWrapper class="bg-black h-full flex flex-col">
    <div class="px-6 py-8 flex items-center justify-between">
      <div class="brand-line flex items-center gap-3">
        <span id="api-dot" class="dot w-3 h-3 rounded-full bg-[#86BC25] shadow-[0_0_10px_rgba(134,188,37,0.8)]" aria-hidden="true"></span>
        <span class="text-2xl font-bold tracking-tighter text-white uppercase font-sans">SMITH</span>
      </div>
      <button 
        class="text-gray-500 hover:text-white transition-colors"
        onclick={() => sidebarOpen.set(false)}
        aria-label="Close Sidebar"
      >
        <CloseOutline size="md" />
      </button>
    </div>
    
    <div class="px-0 flex-1">
      <SidebarGroup>
        {#each navItems as item}
          {@const active = currentPath.startsWith(item.href)}
          <SidebarItem 
            href={item.href} 
            {active}
            onclick={() => sidebarOpen.set(false)}
            class="group text-gray-400 hover:text-[#86BC25] hover:bg-white/5 rounded-none transition-all py-4 px-6 border-l-2 border-transparent {active ? 'border-[#86BC25] text-white bg-white/5' : ''}"
          >
            {#snippet icon()}
              <div class="flex items-center gap-3">
                <item.icon size="sm" class="transition duration-75 group-hover:text-[#86BC25] {active ? 'text-[#86BC25]' : ''}" />
                <span class="font-bold uppercase tracking-tight text-sm">{item.label}</span>
              </div>
            {/snippet}
          </SidebarItem>
        {/each}

        <SidebarItem 
          onclick={() => { chatOpen.update(v => !v); sidebarOpen.set(false); }}
          class="group text-gray-400 hover:text-blue-500 hover:bg-white/5 rounded-none transition-all py-4 px-6 border-l-2 border-transparent"
        >
          {#snippet icon()}
            <div class="flex items-center gap-3">
              <MessagesOutline size="sm" class="transition duration-75 group-hover:text-blue-500" />
              <span class="font-bold uppercase tracking-tight text-sm">Operator Chat</span>
            </div>
          {/snippet}
        </SidebarItem>
      </SidebarGroup>
    </div>

    <div class="mt-auto p-6 border-t border-gray-900">
      <div class="text-[10px] font-bold text-gray-600 uppercase tracking-[0.2em]">
        System v1.0.4
      </div>
    </div>
  </SidebarWrapper>
</Drawer>

<style>
  :global(#sidebar-drawer) {
    background-color: #000000 !important;
  }
</style>
