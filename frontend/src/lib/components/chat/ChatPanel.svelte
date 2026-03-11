<script lang="ts">
    import { onMount } from 'svelte';
    import { chatSession } from '$lib/chat/store.svelte';
    import MessageList from './MessageList.svelte';
    import Composer from './Composer.svelte';
    import ActionCard from './ActionCard.svelte';
    import { CloseOutline } from 'flowbite-svelte-icons';

    let { type = 'prd-refinement', context = {}, onClose } = $props();

    onMount(() => {
        if (!chatSession.sessionId) {
            chatSession.createSession(type, context);
        }
    });
</script>

<div class="flex flex-col h-full bg-gray-950 border-l border-gray-800 shadow-2xl">
    <div class="p-4 border-b border-gray-800 flex justify-between items-center bg-gray-900">
        <div>
            <h3 class="text-sm font-bold text-gray-100 uppercase tracking-wider">Smith Chat</h3>
            <div class="text-[10px] text-gray-500 font-mono">{type}</div>
        </div>
        {#if onClose}
            <button onclick={onClose} class="p-1 hover:bg-gray-800 rounded-full transition-colors text-gray-400 hover:text-gray-100">
                <CloseOutline class="w-5 h-5" />
            </button>
        {/if}
    </div>

    {#if chatSession.error}
        <div class="p-2 bg-red-900/20 border-b border-red-900/50 text-red-400 text-xs text-center">
            {chatSession.error}
        </div>
    {/if}

    <MessageList messages={chatSession.messages} />
    
    {#if chatSession.structuredResults.length > 0}
        <div class="px-4 pb-4 space-y-3">
            {#each chatSession.structuredResults as result}
                <ActionCard {result} />
            {/each}
        </div>
    {/if}

    <Composer />
</div>
