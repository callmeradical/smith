<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { postJSON, requestJSON, deleteJSON, fetchJSON } from '$lib/api';
	import { slugifySegment } from '$lib/utils';

	interface Props {
		open: boolean;
		onClose: () => void;
		projectToEdit?: any;
	}

	let { open, onClose, projectToEdit = null }: Props = $props();

	let isEditing = $derived(!!projectToEdit);
	
	// Form fields
	let id = $state('');
	let name = $state('');
	let repoUrl = $state('');
	let githubUser = $state('');
	let githubCredential = $state('');
	let runtimeImage = $state('');
	let runtimePullPolicy = $state('IfNotPresent');
	let skillsImage = $state('');
	let skillsPullPolicy = $state('IfNotPresent');
	let busy = $state(false);

	$effect(() => {
		if (open) {
			if (projectToEdit) {
				id = projectToEdit.id || '';
				name = projectToEdit.name || '';
				repoUrl = projectToEdit.repo_url || '';
				githubUser = projectToEdit.github_user || '';
				runtimeImage = projectToEdit.runtime_image || '';
				runtimePullPolicy = projectToEdit.runtime_pull_policy || 'IfNotPresent';
				skillsImage = projectToEdit.skills_image || '';
				skillsPullPolicy = projectToEdit.skills_pull_policy || 'IfNotPresent';
				githubCredential = '';
			} else {
				id = '';
				name = '';
				repoUrl = '';
				githubUser = '';
				githubCredential = '';
				runtimeImage = '';
				runtimePullPolicy = 'IfNotPresent';
				skillsImage = '';
				skillsPullPolicy = 'IfNotPresent';
			}
		}
	});

	async function refreshProjects() {
		try {
			const projects = await fetchJSON("/v1/projects");
			appState.update(s => ({ ...s, projects: Array.isArray(projects) ? projects : [] }));
		} catch (err) {
			console.error("Failed to load projects", err);
		}
	}

	async function saveProject() {
		if (!name || !repoUrl) {
			pushToast("Name and Repo URL are required", "err");
			return;
		}

		busy = true;
		const projectId = isEditing ? id : slugifySegment(name);
		const projectPayload = {
			id: projectId,
			name,
			repo_url: repoUrl,
			github_user: githubUser,
			runtime_image: runtimeImage,
			runtime_pull_policy: runtimePullPolicy,
			skills_image: skillsImage,
			skills_pull_policy: skillsPullPolicy
		};

		try {
			if (isEditing) {
				await requestJSON(`/v1/projects/${projectId}`, "PUT", projectPayload);
			} else {
				await postJSON("/v1/projects", projectPayload);
			}

			// If credential was provided, save it too
			if (githubCredential) {
				await postJSON("/v1/projects/credentials/github", {
					actor: "operator",
					project_id: projectId,
					github_user: githubUser,
					credential: githubCredential
				});
			}

			pushToast(`Project ${isEditing ? 'updated' : 'created'} successfully`, "ok");
			await refreshProjects();
			onClose();
		} catch (err: any) {
			pushToast(err.message || "Failed to save project", "err");
		} finally {
			busy = false;
		}
	}

	async function deleteProject() {
		if (!confirm(`Are you sure you want to delete project ${name}?`)) return;
		busy = true;
		try {
			await deleteJSON(`/v1/projects/${id}`);
			// Also delete credential if any
			await requestJSON("/v1/projects/credentials/github", "DELETE", {
				actor: "operator",
				project_id: id
			}).catch(() => {}); // Ignore credential delete errors if it didn't exist
			
			pushToast("Project deleted", "ok");
			await refreshProjects();
			onClose();
		} catch (err: any) {
			pushToast(err.message || "Failed to delete project", "err");
		} finally {
			busy = false;
		}
	}

	async function deleteCredential() {
		if (!confirm("Are you sure you want to delete the GitHub credential for this project?")) return;
		busy = true;
		try {
			await requestJSON("/v1/projects/credentials/github", "DELETE", {
				actor: "operator",
				project_id: id
			});
			pushToast("Credential deleted", "ok");
		} catch (err: any) {
			pushToast(err.message || "Failed to delete credential", "err");
		} finally {
			busy = false;
		}
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="provider-drawer-overlay" style="opacity: 1; visibility: visible; pointer-events: auto; backdrop-filter: blur(8px);" onclick={onClose}></div>
	<aside class="provider-drawer open" aria-hidden="false">
		<div class="provider-drawer-head">
			<div class="provider-drawer-title">{isEditing ? 'Edit Project' : 'New Project'}</div>
			<button type="button" class="provider-drawer-close" onclick={onClose} disabled={busy}>&times;</button>
		</div>
		<section class="panel" style="display: flex; flex-direction: column; gap: 12px;">
			<input type="text" placeholder="project name" bind:value={name} disabled={busy} />
			<input type="url" placeholder="repository URL (https://github.com/org/repo.git)" bind:value={repoUrl} disabled={busy} />
			<input type="text" placeholder="GitHub username or app/bot name" bind:value={githubUser} disabled={busy} />
			<input type="password" placeholder="GitHub credential (PAT or token)" bind:value={githubCredential} disabled={busy} />
			
			<input type="text" placeholder="runtime image (optional)" bind:value={runtimeImage} disabled={busy} />
			<select bind:value={runtimePullPolicy} aria-label="Runtime image pull policy" disabled={busy}>
				<option value="IfNotPresent">Runtime pull policy: IfNotPresent</option>
				<option value="Always">Runtime pull policy: Always</option>
				<option value="Never">Runtime pull policy: Never</option>
			</select>
			
			<input type="text" placeholder="skills/seed image (optional)" bind:value={skillsImage} disabled={busy} />
			<select bind:value={skillsPullPolicy} aria-label="Skills seed image pull policy" disabled={busy}>
				<option value="IfNotPresent">Skills image pull policy: IfNotPresent</option>
				<option value="Always">Skills image pull policy: Always</option>
				<option value="Never">Skills image pull policy: Never</option>
			</select>
			
			<div class="status-note">Credential is saved to backend secret storage.</div>
			
			<div class="project-actions" style="display: grid; gap: 8px; grid-template-columns: 1fr 1fr;">
				<button class="primary" onclick={saveProject} disabled={busy}>save project</button>
				{#if isEditing}
					<button class="danger" onclick={deleteCredential} disabled={busy}>delete credential</button>
				{/if}
			</div>
		</section>
		{#if isEditing}
			<button class="danger drawer-delete-action" style="margin-top: auto;" onclick={deleteProject} disabled={busy}>delete project</button>
		{/if}
	</aside>
{/if}

<style>
	.provider-drawer {
		transform: translateX(0);
		opacity: 1;
		visibility: visible;
		pointer-events: auto;
	}
	.drawer-delete-action {
		width: 100%;
		padding: 10px;
	}
</style>
