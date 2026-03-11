<script lang="ts">
    import { chatSession } from '$lib/chat/store.svelte';
    import { PaperPlaneOutline } from 'flowbite-svelte-icons';

    let content = $state('');

    async function handleSubmit(e: Event) {
        e.preventDefault();
        if (!content.trim() || chatSession.isStreaming) return;

        const msg = content;
        content = '';
        await chatSession.sendMessage(msg);
    }

    function handleKeydown(e: KeyboardEvent) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleSubmit(e);
        }
    }
</script>

<form onsubmit={handleSubmit} class="p-4 border-t border-gray-800 bg-gray-900">
    <div class="relative flex items-center">
        <textarea
            bind:value={content}
            onkeydown={handleKeydown}
            placeholder="Type your message..."
            class="w-full bg-gray-800 text-gray-100 rounded-lg pl-4 pr-12 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500 border-none resize-none"
            rows="1"
            disabled={chatSession.isStreaming}
        ></textarea>
        <button
            type="submit"
            disabled={!content.trim() || chatSession.isStreaming}
            class="absolute right-2 p-2 text-blue-500 hover:text-blue-400 disabled:text-gray-600 transition-colors"
        >
            <PaperPlaneOutline class="w-6 h-6" />
        </button>
    </div>
</form>
