<script lang="ts">
  import { Modal, Button, Label, Input, Select, Textarea, Toolbar, ToolbarButton, ToolbarGroup, Badge } from 'flowbite-svelte';
  import { CloudArrowUpOutline, CloseOutline, BoldOutline, ItalicOutline, ListOutline } from 'flowbite-svelte-icons';
  import { marked } from 'marked';

	interface Props {
		open: boolean;
		title: string;
		content: string;
		projectID?: string;
		projects?: any[];
		isNew?: boolean;
		onClose: () => void;
		onSave: (title: string, content: string, projectID: string) => void;
	}

	let { 
		open = $bindable(), 
		title = $bindable(), 
		content = $bindable(), 
		projectID = $bindable(),
		projects = [],
		isNew = false,
		onClose, 
		onSave 
	}: Props = $props();

  let editTab = $state('write');

  const renderedContent = $derived.by(() => {
    return marked.parse(content || '');
  });

	function handleSave() {
		if (isNew && !projectID) {
			alert("Please select a project");
			return;
		}
		onSave(title, content, projectID || "");
	}
</script>

<Modal bind:open title={isNew ? 'New Document' : 'Edit Document'} size="xl" autoclose={false} class="bg-black border border-gray-800 rounded-none">
  <div class="space-y-6">
    <div class="flex gap-4 items-center">
      <div class="flex-1">
        <Label for="doc-title" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Document Title</Label>
        <Input type="text" id="doc-title" placeholder="Implementation Plan" bind:value={title} class="bg-black border-gray-800 text-white font-bold rounded-none focus:border-[#86BC25]" />
      </div>
      <div class="w-64">
        <Label for="doc-project" class="mb-2 text-gray-400 uppercase font-bold text-xs tracking-widest">Project</Label>
        {#if isNew}
          <Select id="doc-project" bind:value={projectID} class="bg-black border-gray-800 text-white rounded-none focus:border-[#86BC25]">
            <option value="">Select Project</option>
            {#each projects as p}
              <option value={p.id}>{p.name}</option>
            {/each}
          </Select>
        {:else}
          <div class="bg-black border border-gray-800 rounded-none px-4 py-2 text-gray-400 font-mono text-sm uppercase font-bold tracking-widest">
            {projectID}
          </div>
        {/if}
      </div>
    </div>

    <div class="flex flex-col h-[500px]">
      <div class="flex border-b border-gray-800 mb-0">
        <button 
          class="px-6 py-3 text-xs uppercase tracking-widest font-bold transition-colors {editTab === 'write' ? 'text-[#86BC25] border-b-2 border-[#86BC25]' : 'text-gray-500 hover:text-gray-300'}"
          onclick={() => editTab = 'write'}
        >
          Write
        </button>
        <button 
          class="px-6 py-3 text-xs uppercase tracking-widest font-bold transition-colors {editTab === 'preview' ? 'text-[#86BC25] border-b-2 border-[#86BC25]' : 'text-gray-500 hover:text-gray-300'}"
          onclick={() => editTab = 'preview'}
        >
          Preview
        </button>
      </div>

      <div class="flex-1 overflow-hidden border-x border-b border-gray-800">
        {#if editTab === 'write'}
          <Textarea 
            bind:value={content} 
            rows={15} 
            placeholder="Start writing in Markdown..." 
            class="bg-black border-none text-gray-300 font-mono text-lg focus:ring-0 resize-none h-full rounded-none"
          >
            <Toolbar slot="header" embedded class="bg-black border-b border-gray-800 rounded-none">
              <ToolbarGroup>
                <ToolbarButton name="Bold" class="text-gray-400 hover:text-white" onclick={() => content += '**bold**'}><BoldOutline size="sm" /></ToolbarButton>
                <ToolbarButton name="Italic" class="text-gray-400 hover:text-white" onclick={() => content += '_italic_'}><ItalicOutline size="sm" /></ToolbarButton>
                <ToolbarButton name="List" class="text-gray-400 hover:text-white" onclick={() => content += '\n- '}><ListOutline size="sm" /></ToolbarButton>
              </ToolbarGroup>
            </Toolbar>
          </Textarea>
        {:else}
          <div class="markdown-preview prose prose-invert max-w-none h-full overflow-y-auto bg-black rounded-none p-8 border-none">
            {@html renderedContent}
          </div>
        {/if}
      </div>
    </div>
  </div>

  <svelte:fragment slot="footer">
    <Button color="alternative" class="rounded-none border-gray-700 font-bold uppercase text-[10px] tracking-widest" onclick={onClose}>Cancel</Button>
    <Button color="none" class="bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest px-8 py-2 rounded-none transition-all ml-auto" onclick={handleSave}>
      <CloudArrowUpOutline size="sm" class="mr-2" />
      Save Document
    </Button>
  </svelte:fragment>
</Modal>

<style>
  .markdown-preview {
    color: #d1d7e0;
    line-height: 1.8;
  }

  :global(.markdown-preview h1) { font-size: 1.8rem; border-bottom: 1px solid #30363d; padding-bottom: 0.3em; margin-top: 20px; margin-bottom: 12px; font-weight: 600; color: #fff; }
  :global(.markdown-preview h2) { font-size: 1.4rem; border-bottom: 1px solid #30363d; padding-bottom: 0.3em; margin-top: 24px; margin-bottom: 16px; font-weight: 600; color: #fff; }
  :global(.markdown-preview p) { margin-top: 0; margin-bottom: 12px; }
  :global(.markdown-preview ul) { padding-left: 2em; margin-bottom: 12px; list-style-type: disc; }
  :global(.markdown-preview code) { padding: 0.2em 0.4em; margin: 0; font-size: 85%; background-color: rgba(110, 118, 129, 0.4); border-radius: 0px; font-family: var(--mono); }
  :global(.markdown-preview pre) { padding: 12px; overflow: auto; font-size: 85%; line-height: 1.45; background-color: #111; border-radius: 0px; border: 1px solid #222; margin-bottom: 12px; }
</style>
