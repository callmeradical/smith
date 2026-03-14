<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import TopBar from '$lib/components/TopBar.svelte';
	import ProviderEditorDrawer from '$lib/components/ProviderEditorDrawer.svelte';
  import { Button, Card, Badge } from 'flowbite-svelte';
  import { UsersGroupOutline, CheckCircleOutline, AdjustmentsHorizontalOutline, BrainSolid } from 'flowbite-svelte-icons';

	const providers = [
		{ 
      id: "codex", 
      label: "OpenAI Codex CLI", 
      subtitle: "Execute OpenAI models via Codex CLI.",
      status: "connected",
      description: "Direct integration with Codex API for code generation and analysis.",
      icon: BrainSolid
    }
	];

  let editorOpen = $state(false);
  let selectedProvider = $state<any>(null);

  function openConfig(provider: any) {
    selectedProvider = provider;
    editorOpen = true;
  }
</script>

<TopBar title="Providers" />

<ProviderEditorDrawer bind:open={editorOpen} onClose={() => editorOpen = false} provider={selectedProvider} />

<section class="providers-shell px-4">
	<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
		{#each providers as provider}
      <Card class="bg-black border-gray-800 hover:border-[#86BC25]/50 transition-all p-0 overflow-hidden rounded-none">
        <div class="p-6 flex flex-col h-full gap-4">
          <div class="flex justify-between items-start">
            <div class="w-12 h-12 bg-[#86BC25]/10 flex items-center justify-center text-[#86BC25]">
              <provider.icon size="lg" />
            </div>
            <Badge color="green" class="uppercase text-[10px] font-bold px-2 py-0.5 rounded-none bg-[#86BC25] text-black">
              {provider.status}
            </Badge>
          </div>

          <div>
            <h3 class="text-xl font-bold text-white tracking-tight uppercase">{provider.label}</h3>
            <p class="text-sm text-gray-400 mt-1">{provider.subtitle}</p>
          </div>

          <div class="text-sm text-gray-500 leading-relaxed font-medium">
            {provider.description}
          </div>

          <div class="mt-auto pt-6 border-t border-gray-900">
            <div class="flex items-center justify-between">
              <Button 
                color="alternative" 
                class="flex items-center bg-[#86BC25] text-black hover:bg-[#6b961d] font-bold uppercase text-[9px] tracking-widest px-6 py-2 rounded-none transition-all h-7"
                onclick={() => openConfig(provider)}
              >
                <AdjustmentsHorizontalOutline size="xs" class="mr-2" />
                <span>Configure</span>
              </Button>
              <div class="text-[#86BC25]">
                <CheckCircleOutline size="lg" />
              </div>
            </div>
          </div>
        </div>
      </Card>
		{/each}
	</div>
</section>
