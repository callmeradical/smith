<script lang="ts">
	import { appState, pushToast } from '$lib/stores';
	import { postJSON, requestJSON, deleteJSON, fetchJSON } from '$lib/api';
	import { slugifySegment } from '$lib/utils';
  import { Drawer, Button, Label, Input, Select, Helper } from 'flowbite-svelte';
  import { ArchiveOutline, TrashBinOutline, CheckOutline, CloseOutline, GlobeOutline } from 'flowbite-svelte-icons';
  import { sineIn } from 'svelte/easing';

	interface Props {
		open: boolean;
		onClose: () => void;
		projectToEdit?: any;
	}

	let { open = $bindable(), onClose, projectToEdit = null }: Props = $props();

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

  // Use bind:hidden for drawer control
  let isHidden = $state(true);
  $effect(() => { isHidden = !open; });
  $effect(() => { if (open && isHidden) onClose(); });

  let transitionParams = {
    x: 450,
    duration: 300,
    easing: sineIn
  };

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
			if (githubCredential) {
				await postJSON("/v1/projects/credentials/github", {
					actor: "operator", project_id: projectId, github_user: githubUser, credential: githubCredential
				});
			}
			pushToast(`Project ${isEditing ? 'updated' : 'created'} successfully`, "ok");
			const projects = await fetchJSON("/v1/projects");
			appState.update(s => ({ ...s, projects: Array.isArray(projects) ? projects : [] }));
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
			pushToast("Project deleted", "ok");
			const projects = await fetchJSON("/v1/projects");
			appState.update(s => ({ ...s, projects: Array.isArray(projects) ? projects : [] }));
			onClose();
		} catch (err: any) {
			pushToast(err.message || "Failed to delete project", "err");
		} finally {
			busy = false;
		}
	}
</script>

<Drawer 
  placement="right" 
  bind:hidden={isHidden} 
  outsideclose={true}
  id="project-editor-drawer" 
  width="default" 
  class="fixed top-0 right-0 bg-black border-l border-gray-800 z-50 overflow-y-auto h-full m-0 shadow-2xl p-0 w-[450px]"
>
  <div class="flex flex-col h-full relative">
    <!-- Header -->
    <div class="px-8 py-10 bg-slate-900/20 border-b border-gray-900 flex items-center justify-between">
      <div class="flex items-center gap-4">
        <div class="w-12 h-12 bg-[#86BC25]/10 flex items-center justify-center text-[#86BC25]">
          <ArchiveOutline size="lg" />
        </div>
        <div>
          <h2 class="text-2xl font-bold text-white uppercase tracking-tighter">{isEditing ? 'Edit Project' : 'New Project'}</h2>
          <p class="text-[10px] font-bold text-gray-500 uppercase tracking-[0.2em] mt-1">Configuration & Runtime</p>
        </div>
      </div>
      <button 
        class="text-white hover:text-[#86BC25] transition-colors p-2"
        onclick={onClose}
        aria-label="Close Drawer"
      >
        <CloseOutline size="md" />
      </button>
    </div>

    <!-- Body -->
    <form class="flex-1 p-8 space-y-8" onsubmit={(e) => { e.preventDefault(); saveProject(); }}>
      <div class="space-y-6">
        <div>
          <Label for="name" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Project Name</Label>
          <Input type="text" id="name" placeholder="Project Alpha" bind:value={name} disabled={busy} required class="bg-black border-gray-800 text-white font-bold rounded-none focus:border-[#86BC25]" />
          <Helper class="mt-2 text-gray-600 text-[10px] uppercase font-bold">This will be used to generate the system ID.</Helper>
        </div>
        
        <div>
          <Label for="repo" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Repository URL</Label>
          <div class="flex">
            <span class="inline-flex items-center px-3 bg-slate-900 border border-r-0 border-gray-800 text-gray-500">
              <GlobeOutline size="sm" />
            </span>
            <Input type="url" id="repo" placeholder="https://github.com/org/repo.git" bind:value={repoUrl} disabled={busy} required class="bg-black border-gray-800 text-white font-mono text-sm rounded-none focus:border-[#86BC25]" />
          </div>
        </div>

        <div class="my-8 border-t border-gray-900"></div>

        <div class="space-y-6">
          <h4 class="text-[10px] font-bold uppercase tracking-[0.3em] text-[#86BC25]">Authentication</h4>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <Label for="user" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Git User</Label>
              <Input type="text" id="user" placeholder="username" bind:value={githubUser} disabled={busy} class="bg-black border-gray-800 text-white font-bold rounded-none focus:border-[#86BC25]" />
            </div>
            <div>
              <Label for="token" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Secret Token</Label>
              <Input type="password" id="token" placeholder="••••••••" bind:value={githubCredential} disabled={busy} class="bg-black border-gray-800 text-white rounded-none focus:border-[#86BC25]" />
            </div>
          </div>
        </div>

        <div class="my-8 border-t border-gray-900"></div>

        <div class="space-y-6">
          <h4 class="text-[10px] font-bold uppercase tracking-[0.3em] text-[#86BC25]">Runtime & Skills</h4>
          <div class="grid grid-cols-1 gap-4">
            <div>
              <Label for="r-image" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Replica Image</Label>
              <Input type="text" id="r-image" placeholder="smith-replica:latest" bind:value={runtimeImage} disabled={busy} class="bg-black border-gray-800 text-white font-mono text-sm rounded-none focus:border-[#86BC25]" />
            </div>
            <div>
              <Label for="s-image" class="mb-2 text-gray-400 uppercase font-bold text-[10px] tracking-widest">Skills Image</Label>
              <Input type="text" id="s-image" placeholder="smith-skills:latest" bind:value={skillsImage} disabled={busy} class="bg-black border-gray-800 text-white font-mono text-sm rounded-none focus:border-[#86BC25]" />
            </div>
          </div>
        </div>
      </div>

      <!-- Footer Actions -->
      <div class="pt-10 pb-20 border-t border-gray-900 flex justify-between gap-4 mt-auto">
        {#if isEditing}
          <Button color="red" size="sm" class="rounded-none font-bold uppercase text-[10px] tracking-widest px-6" onclick={deleteProject} disabled={busy}>
            <TrashBinOutline size="xs" class="mr-2" />
            Delete
          </Button>
        {/if}
        <div class="flex gap-3 ml-auto">
          <Button color="alternative" size="sm" class="rounded-none font-bold uppercase text-[10px] tracking-widest border-gray-700 bg-slate-800 text-gray-300 hover:bg-slate-700 px-6" onclick={onClose} disabled={busy}>Cancel</Button>
          <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-[10px] tracking-widest rounded-none px-8 py-2 hover:bg-[#a1e02c]" type="submit" disabled={busy}>
            <CheckOutline size="xs" class="mr-2" />
            {isEditing ? 'Update Project' : 'Create Project'}
          </Button>
        </div>
      </div>
    </form>
  </div>
</Drawer>

<style>
  :global(#project-editor-drawer) {
    background-color: #000000 !important;
    left: auto !important;
    right: 0 !important;
  }
  /* Force hide Flowbite's default absolute-positioned close button */
  :global(#project-editor-drawer button[aria-label="Close"]) {
    display: none !important;
  }
  /* Preserve custom close button visibility */
  :global(#project-editor-drawer .relative > button[aria-label="Close Drawer"]) {
    display: flex !important;
  }
</style>
