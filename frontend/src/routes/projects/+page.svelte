<script lang="ts">
	import { onMount } from 'svelte';
	import { appState, pushToast } from '$lib/stores';
	import TopBar from '$lib/components/TopBar.svelte';
	import ProjectEditorDrawer from '$lib/components/ProjectEditorDrawer.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
  import { Button, Card, Badge } from 'flowbite-svelte';
  import { PlusOutline, CogOutline, ArchiveOutline, GithubSolid, GitlabSolid, GlobeOutline } from 'flowbite-svelte-icons';

	let editorOpen = $state(false);
	let projectToEdit = $state<any>(null);

	function openNewProject() {
		projectToEdit = null;
		editorOpen = true;
	}

	function openEditProject(project: any) {
		projectToEdit = project;
		editorOpen = true;
	}

  function getRepoIcon(url: string) {
    if (!url) return GlobeOutline;
    const lowerUrl = url.toLowerCase();
    if (lowerUrl.includes('github.com')) return GithubSolid;
    if (lowerUrl.includes('gitlab.com')) return GitlabSolid;
    return GlobeOutline;
  }
</script>

<TopBar title="Projects" />

<!-- Inline Actions Header -->
<div class="flex justify-end gap-2 -mt-14 mb-8 relative z-50 px-4">
  <Button color="alternative" class="bg-[#86BC25] text-black font-bold uppercase text-[9px] tracking-widest py-1 px-3 h-7 rounded-none" onclick={openNewProject}>
    <PlusOutline size="xs" class="mr-1.5" />
    New Project
  </Button>
</div>

<ProjectEditorDrawer bind:open={editorOpen} onClose={() => editorOpen = false} {projectToEdit} />

<section class="projects-shell px-4">
	<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
		{#if $appState.projects.length === 0}
      <div class="col-span-full py-12">
        <EmptyState 
          title="No Projects Configured" 
          description="Projects define the repositories and environments for your autonomous loops. Create one to get started."
          buttonText="Create Project"
          buttonHref="#"
          icon="🏗️"
        />
      </div>
		{:else}
			{#each $appState.projects as project (project.id)}
        {@const Icon = getRepoIcon(project.repo_url)}
        <Card class="bg-black border-gray-800 hover:border-[#86BC25]/50 transition-all p-6 rounded-none">
          <div class="flex flex-col h-full gap-4">
            <div class="flex justify-between items-start gap-2">
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 bg-[#86BC25]/10 flex items-center justify-center text-[#86BC25]">
                  <Icon size="lg" />
                </div>
                <div>
                  <h3 class="text-lg font-bold text-white tracking-tight uppercase">{project.name}</h3>
                  <code class="text-[10px] text-gray-500 font-mono">{project.id}</code>
                </div>
              </div>
              <Button color="alternative" size="xs" class="p-2 border-gray-700 hover:bg-slate-800 rounded-none h-8 w-8" onclick={() => openEditProject(project)}>
                <CogOutline size="sm" />
              </Button>
            </div>

            <div class="space-y-2">
              <div class="text-[10px] font-bold uppercase tracking-widest text-gray-500">Repository</div>
              <div class="text-sm text-gray-300 font-mono truncate bg-black p-2 border border-gray-800">
                {project.repo_url}
              </div>
            </div>

            <div class="mt-auto pt-4 border-t border-gray-900 flex justify-between items-center">
              <div class="flex gap-2">
                <Badge color="gray" class="bg-slate-800 text-gray-400 text-[10px] rounded-none">{project.github_user || 'no-user'}</Badge>
              </div>
              <div class="text-[10px] font-mono text-gray-600 uppercase font-bold tracking-widest">
                RT: {project.runtime_image ? 'custom' : 'default'}
              </div>
            </div>
          </div>
        </Card>
			{/each}
		{/if}
	</div>
</section>
