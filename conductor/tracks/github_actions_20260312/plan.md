# Implementation Plan: Implement per-component GitHub Action build pipelines and container image/Helm chart storage

## Phase 1: Foundation and Image Infrastructure [checkpoint: c139a75]
- [x] Task: Create per-component build/test workflows for backend services (Go)
    - [x] Create workflow for `smith-api`
    - [x] Create workflow for `smith-core`
    - [x] Create workflow for `smith-replica`
    - [x] Create workflow for `smithctl`
- [x] Task: Create build/test workflow for frontend (`smith-console`)
    - [x] Create Svelte/Vite workflow with Playwright tests
- [x] Task: Configure Docker builds and GHCR push for all services
    - [x] Setup multi-arch build support (if needed)
    - [x] Configure authentication for GHCR in workflows
- [x] Task: Conductor - User Manual Verification 'Phase 1: Foundation and Image Infrastructure' (Protocol in workflow.md)

## Phase 2: Helm and Global Coordination
- [ ] Task: Implement Helm chart packaging and GitHub Packages push
    - [ ] Create workflow for chart versioning and storage
- [ ] Task: Update global deployment pipeline to coordinate component builds
    - [ ] Implement triggering logic for overall platform verification
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Helm and Global Coordination' (Protocol in workflow.md)
