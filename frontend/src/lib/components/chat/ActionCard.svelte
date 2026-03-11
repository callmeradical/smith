<script lang="ts">
    import { chatSession } from '$lib/chat/store.svelte';
    import { CheckOutline, CloseOutline } from 'flowbite-svelte-icons';

    let { result } = $props();
    let busy = $state(false);

    async function handleApprove() {
        busy = true;
        await chatSession.commitAction(result.type, result.payload);
        busy = false;
    }

    function handleReject() {
        chatSession.structuredResults = chatSession.structuredResults.filter(r => r !== result);
    }
</script>

<div class="bg-blue-900/20 border border-blue-500/50 rounded-lg p-4 space-y-3 shadow-lg animate-in fade-in slide-in-from-bottom-2">
    <div class="flex items-center justify-between">
        <span class="text-[10px] font-bold uppercase tracking-tighter text-blue-400">Suggested Action</span>
        <span class="text-xs font-mono text-blue-300">{result.type}</span>
    </div>

    <div class="space-y-2">
        <h4 class="text-sm font-bold text-white">{result.payload.title || 'Untitled Action'}</h4>
        {#if result.payload.description}
            <p class="text-xs text-gray-400 leading-relaxed">{result.payload.description}</p>
        {/if}
        
        {#if result.payload.tasks}
            <ul class="space-y-1">
                {#each result.payload.tasks as task}
                    <li class="text-[10px] text-gray-500 flex items-center gap-2">
                        <div class="w-1 h-1 bg-blue-500 rounded-full"></div>
                        {task}
                    </li>
                {/each}
            </ul>
        {/if}
    </div>

    <div class="flex gap-2 pt-2">
        <button 
            onclick={handleApprove}
            disabled={busy}
            class="flex-1 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold py-2 rounded flex items-center justify-center gap-2 transition-colors disabled:opacity-50"
        >
            <CheckOutline class="w-4 h-4" />
            Approve
        </button>
        <button 
            onclick={handleReject}
            disabled={busy}
            class="px-3 border border-gray-700 hover:bg-gray-800 text-gray-400 hover:text-white rounded transition-colors"
        >
            <CloseOutline class="w-4 h-4" />
        </button>
    </div>
</div>
