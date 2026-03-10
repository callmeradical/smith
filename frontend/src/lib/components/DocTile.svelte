<script lang="ts">
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

<div class="pod-tile" style={doc.status === "archived" ? "opacity: 0.6" : ""}>
	<div class="tile-head">
		<div class="tile-title loop-id">{doc.title || "Untitled"}</div>
		<div class="badge {doc.status === "active" ? "state-synced" : "state-cancelled"}">{doc.status || "active"}</div>
	</div>
	<div class="tile-meta">
		<span class="muted">{doc.source_type || "unknown"}: {doc.source_ref || "direct"}</span>
		<span class="muted">{updatedAtLabel}</span>
	</div>
	<div class="tile-footer" style="margin-top: 12px; display: flex; gap: 8px; justify-content: flex-start;">
		<button class="tile-action-button" onclick={() => onEdit(doc.id)}>Edit</button>
		<button class="tile-action-button primary" onclick={() => onBuild(doc.id)}>Build</button>
		<button class="tile-action-button" onclick={() => onArchive(doc.id)}>{doc.status === "active" ? "Archive" : "Unarchive"}</button>
		<button class="tile-action-button danger" onclick={() => onDelete(doc.id)}>Delete</button>
	</div>
</div>
