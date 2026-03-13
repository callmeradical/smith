# Implementation Plan: Fix Critical Vulnerabilities in smith-console

#### Phase 1: Baseline Verification [checkpoint: 9ceed47]
- [x] **Task: Confirm baseline vulnerability state**
    - [x] Run `make build-local` to ensure local images are fresh.
    - [x] Run `make trivy-scan-local` and confirm findings for `smith-console:local` (libcrypto3, libssl3, libxml2).
- [x] **Task: Conductor - User Manual Verification 'Baseline Verification' (Protocol in workflow.md)**


#### Phase 2: Remediation [checkpoint: cfb41d0]
- [x] **Task: Update base image in Dockerfile**
    - [x] Modify `docker/console.Dockerfile` to use `nginx:alpine3.20` or a verified stable base.
    - [x] Add explicit `apk upgrade` for `libcrypto3`, `libssl3`, and `libxml2` to ensure latest patches.
- [x] **Task: Rebuild and verify build integrity**
    - [x] Run `make build-local` and ensure no build regressions.
- [x] **Task: Conductor - User Manual Verification 'Remediation' (Protocol in workflow.md)**

#### Phase 3: Final Verification [checkpoint: cfb41d0]
- [x] **Task: Run security scan**
    - [x] Run `make trivy-scan-local` and verify `smith-console:local` reports 0 `CRITICAL` vulnerabilities.
- [x] **Task: Run functional E2E tests**
    - [x] Run `make test-frontend` and ensure all Playwright tests pass.
- [x] **Task: Conductor - User Manual Verification 'Final Verification' (Protocol in workflow.md)**
