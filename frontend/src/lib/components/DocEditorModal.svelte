<script lang="ts">
	import { Editor } from 'bytemd';
	import gfm from '@bytemd/plugin-gfm';
	import 'bytemd/dist/index.css';
	import { onMount, onDestroy } from 'svelte';

	interface Props {
		open: boolean;
		title: string;
		content: string;
		onClose: () => void;
		onSave: (title: string, content: string) => void;
	}

	let { open, title = $bindable(), content = $bindable(), onClose, onSave }: Props = $props();

	let editorEl: HTMLDivElement;
	let editor: Editor;

	const plugins = [gfm()];

	$effect(() => {
		if (open && editorEl && !editor) {
			editor = new Editor({
				target: editorEl,
				props: {
					value: content,
					plugins
				}
			});
			editor.$on('change', (e: any) => {
				content = e.detail.value;
			});
		}
		if (editor && open) {
			editor.$set({ value: content });
		}
	});

	onDestroy(() => {
		if (editor) {
			editor.$destroy();
		}
	});

	function handleSave() {
		onSave(title, content);
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="provider-drawer-overlay" style="opacity: 1; visibility: visible; pointer-events: auto; backdrop-filter: blur(8px);" onclick={onClose}></div>
	<aside class="doc-create-modal open" aria-hidden="false">
		<div class="provider-drawer-head">
			<div class="provider-drawer-title">Edit Document</div>
			<button type="button" class="provider-drawer-close" onclick={onClose}>&times;</button>
		</div>
		<section class="panel" style="flex: 1; display: flex; flex-direction: column; overflow: hidden; gap: 12px;">
			<input type="text" placeholder="Document Title" bind:value={title} style="font-size: 1.2rem; font-weight: bold; padding: 8px;" />
			
			<div class="editor-container" bind:this={editorEl}></div>

			<div class="doc-actions" style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px;">
				<button onclick={onClose}>cancel</button>
				<button class="primary" onclick={handleSave}>save</button>
			</div>
		</section>
	</aside>
{/if}

<style>
	.doc-create-modal {
		position: fixed;
		left: 50%;
		top: 50%;
		width: min(85vw, calc(100vw - 28px));
		height: min(85vh, calc(100vh - 28px));
		background: rgba(7, 9, 14, 0.98);
		border-radius: 14px;
		box-shadow: 0 16px 30px rgba(0, 0, 0, 0.45);
		padding: 12px;
		display: flex;
		flex-direction: column;
		transform: translate(-50%, -50%) scale(1);
		opacity: 1;
		visibility: visible;
		pointer-events: auto;
		z-index: 33;
	}
	.editor-container {
		flex: 1;
		overflow: hidden;
		background: white;
		color: black;
		border-radius: 8px;
	}
	:global(.bytemd) {
		height: 100%;
	}
</style>
