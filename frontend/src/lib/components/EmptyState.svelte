<script lang="ts">
  import { Button } from 'flowbite-svelte';
  import { ArrowRightOutline } from 'flowbite-svelte-icons';

	interface Props {
		title: string;
		description: string;
		buttonText?: string;
		buttonHref?: string;
		icon?: string;
	}

	let { 
		title, 
		description, 
		buttonText, 
		buttonHref,
		icon = "📁"
	}: Props = $props();
</script>

<div class="empty-state-container bg-slate-900/20 border border-dashed border-gray-800 rounded-none p-16 flex flex-col items-center justify-center text-center">
  <div class="text-6xl mb-6 filter drop-shadow-[0_0_15px_rgba(134,188,37,0.3)]">
    {icon}
  </div>
  <h2 class="text-2xl font-bold text-white mb-2 tracking-tight uppercase">{title}</h2>
  <p class="text-gray-500 max-w-sm mb-8 leading-relaxed font-medium">{description}</p>
  
  {#if buttonText && buttonHref}
    <Button color="none" class="bg-[#86BC25] text-black font-bold uppercase text-xs px-8 py-3 rounded-none transition-all" href={buttonHref}>
      {buttonText}
      <ArrowRightOutline size="sm" class="ml-2" />
    </Button>
  {/if}
</div>
