# Specification: Implement per-component GitHub Action build pipelines and container image/Helm chart storage

## Overview
This track focuses on updating the CI/CD workflow to provide independent build and test pipelines for each component of the Smith platform, while also maintaining a global deployment pipeline. Additionally, it aims to automate the building and storage of container images and Helm charts within GitHub's ecosystem (GitHub Container Registry and GitHub Packages).

## Functional Requirements
- **Per-Component Pipelines:** Create separate GitHub Action workflows for `smith-api`, `smith-core`, `smith-replica`, `smithctl`, and `smith-console`.
- **Global Deployment Pipeline:** Maintain a top-level pipeline that coordinates the build and test of the entire platform.
- **Image Building & Storage:** Automate the building of Docker images for each service and push them to the GitHub Container Registry (GHCR).
- **Helm Chart Storage:** Automate the packaging and pushing of Helm charts to GitHub Packages.

## Non-Functional Requirements
- **Efficiency:** Pipelines should only trigger on changes to their respective components.
- **Traceability:** Build artifacts (images, charts) should be clearly tagged and associated with their respective commits.

## Acceptance Criteria
- [ ] Each component has its own functional build and test pipeline.
- [ ] Container images for all services are successfully built and pushed to GHCR.
- [ ] Helm charts are packaged and available in GitHub Packages.
- [ ] The global deployment pipeline correctly coordinates overall platform verification.
