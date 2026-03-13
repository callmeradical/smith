<script lang="ts">
    import { chatSession } from '$lib/chat/store.svelte';

    let { messages = [] } = $props();
</script>

<div class="flex-1 overflow-y-auto p-4 space-y-4">
    {#each messages as message}
        <div class="flex {message.role === 'user' ? 'justify-end' : 'justify-start'}">
            <div class="max-w-[80%] rounded-lg p-3 {message.role === 'user' ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-100'}">
                <div class="prose prose-invert prose-sm whitespace-pre-wrap">
                    {message.content}
                </div>
                <div class="text-[10px] mt-1 opacity-50">
                    {message.timestamp.toLocaleTimeString()}
                </div>
            </div>
        </div>
    {/each}
    
    {#if chatSession.activeTool}
        <div class="flex justify-start">
            <div class="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 flex items-center space-x-2">
                <div class="animate-spin h-3 w-3 border-2 border-blue-500 border-t-transparent rounded-full"></div>
                <span class="text-xs text-gray-400">Tool: {chatSession.activeTool}...</span>
            </div>
        </div>
    {/if}
</div>
