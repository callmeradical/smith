<script lang="ts">
	import '../app.css';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Toast from '$lib/components/Toast.svelte';
	import { sidebarOpen, appState, pushToast } from '$lib/stores';
	import { onMount } from 'svelte';
	import { fetchJSON } from '$lib/api';

	let { children } = $props();

	function normalizeLoop(item: any) {
		const record = item.record || item.Record || item.State || {};
		const loopID = record.loop_id || record.LoopID || item.loop_id || item.LoopID || "unknown-loop";
		const status = (record.state || record.State || "unknown").toLowerCase();
		const attempt = Number(record.attempt || record.Attempt || 0);
		const reason = record.reason || record.Reason || "";
		const revision = Number(item.revision || item.Revision || record.observed_revision || 0);
		return {
			loopID,
			project: record.project_id || record.project || record.project_name || "default",
			status,
			attempt,
			reason,
			revision,
		};
	}

	async function initApp() {
		try {
			const projects = await fetchJSON("/v1/projects");
			appState.update(s => ({ ...s, projects: Array.isArray(projects) ? projects : [] }));
		} catch (err) {
			console.error("Failed to load projects", err);
		}

		connectStreams();
	}

	function connectStreams() {
		const loopsUrl = '/api/v1/loops/stream';
		const loopsSource = new EventSource(loopsUrl);
		loopsSource.addEventListener('update', (event) => {
			try {
				const data = JSON.parse(event.data);
				const normalized = normalizeLoop(data);
				appState.update(s => {
					const loops = [...s.loops];
					const idx = loops.findIndex((l: any) => l.loopID === normalized.loopID);
					if (idx >= 0) {
						loops[idx] = normalized;
					} else {
						loops.push(normalized as never);
					}
					return { ...s, loops };
				});
			} catch(e) {}
		});

		const docsUrl = '/api/v1/documents/stream';
		const docsSource = new EventSource(docsUrl);
		docsSource.addEventListener('update', (event) => {
			try {
				const doc = JSON.parse(event.data);
				appState.update(s => {
					const docs = [...s.documents];
					const idx = docs.findIndex((d: any) => d.id === doc.id);
					if (idx >= 0) {
						docs[idx] = doc as never;
					} else {
						docs.push(doc as never);
					}
					return { ...s, documents: docs };
				});
			} catch(e) {}
		});

		const auditUrl = '/api/v1/audit/stream';
		const auditSource = new EventSource(auditUrl);
		auditSource.addEventListener('update', (event) => {
			try {
				const rec = JSON.parse(event.data);
				pushToast(`[${rec.action}] ${rec.target_loop_id}`, "muted");
			} catch(e) {}
		});
	}

	onMount(() => {
		initApp();
	});
</script>

<div id="sidebar-overlay" class="sidebar-overlay" class:open={$sidebarOpen} aria-hidden="true" onclick={() => sidebarOpen.set(false)}></div>
<div id="provider-drawer-overlay" class="provider-drawer-overlay" aria-hidden="true"></div>

<Toast />

<div class="shell">
	<Sidebar />

	<main class="workspace">
		{@render children()}
	</main>
</div>

<style>
	.sidebar-overlay.open {
		display: block;
	}
</style>
