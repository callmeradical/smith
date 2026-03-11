<script lang="ts">
  import { Card, Badge, Button } from 'flowbite-svelte';
  import { EditOutline, RocketOutline, ArchiveOutline, TrashBinOutline } from 'flowbite-svelte-icons';

	interface Props {
		doc: any;
		onEdit: (id: string) => void;
		onBuild: (id: string) => void;
		onArchive: (id: string) => void;
		onDelete: (id: string) => void;
	}

	let { doc, onEdit, onBuild, onArchive, onDelete }: Props = $props();
	
	const updatedAtLabel = $derived(doc.updated_at ? new Date(doc.updated_at).toLocaleString() : "unknown date");
</script>

<Card class="bg-slate-900/40 border-gray-800 hover:border-[#86BC25]/50 transition-all backdrop-blur-sm p-4 {doc.status === 'archived' ? 'opacity-60' : ''} rounded-none">
  <div class="flex flex-col h-full gap-3">
    <div class="flex justify-between items-start gap-2">
      <h3 class="text-sm font-bold text-white truncate uppercase" title={doc.title}>{doc.title || "Untitled"}</h3>
      <Badge color="dark" class="uppercase text-[10px] font-bold px-2 py-0.5 rounded-none {doc.status === 'active' ? 'bg-[#86BC25] text-black' : 'bg-slate-800 text-gray-400'}">
        {doc.status || 'active'}
      </Badge>
    </div>

    <div class="text-[10px] text-gray-500 uppercase tracking-widest font-mono">
      {doc.source_type || "direct"} &bull; {updatedAtLabel}
    </div>

    <div class="mt-auto pt-3 border-t border-gray-800/50 flex gap-2">
      <Button color="alternative" size="xs" class="px-2 rounded-none border-gray-700" onclick={() => onEdit(doc.id)} title="Edit">
        <EditOutline size="xs" />
      </Button>
      <Button color="none" size="xs" class="px-2 rounded-none bg-[#86BC25] text-black hover:bg-[#6b961d]" onclick={() => onBuild(doc.id)} title="Build">
        <RocketOutline size="xs" />
      </Button>
      <Button color="alternative" size="xs" class="px-2 rounded-none border-gray-700" onclick={() => onArchive(doc.id)} title={doc.status === "active" ? "Archive" : "Unarchive"}>
        <ArchiveOutline size="xs" />
      </Button>
      <Button color="red" size="xs" class="px-2 rounded-none" onclick={() => onDelete(doc.id)} title="Delete">
        <TrashBinOutline size="xs" />
      </Button>
    </div>
  </div>
</Card>
