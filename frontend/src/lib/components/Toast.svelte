<script lang="ts">
	import { toastMessages } from '$lib/stores';
  import { Toast } from 'flowbite-svelte';
  import { CheckCircleOutline, ExclamationCircleOutline, BellOutline } from 'flowbite-svelte-icons';
  import { fly } from 'svelte/transition';
</script>

<div class="fixed bottom-5 right-5 z-[1000] flex flex-col gap-3 pointer-events-none">
	{#each $toastMessages as toast (toast.id)}
    {#if toast.show}
      <div transition:fly={{ x: 200, duration: 200 }} class="pointer-events-auto">
        <Toast color={toast.level === 'ok' ? 'green' : toast.level === 'err' ? 'red' : 'blue'} class="bg-black border border-gray-800 shadow-2xl rounded-none">
          {#snippet icon()}
            {#if toast.level === 'ok'}
              <CheckCircleOutline size="md" class="text-[#86BC25]" />
            {:else}
              <ExclamationCircleOutline size="md" class="text-rose-500" />
            {/if}
          {/snippet}
          <div class="ml-3 text-sm font-bold text-gray-200 uppercase tracking-widest">
            {toast.message}
          </div>
        </Toast>
      </div>
    {/if}
	{/each}
</div>
