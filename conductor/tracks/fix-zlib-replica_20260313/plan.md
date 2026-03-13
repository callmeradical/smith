# Implementation Plan: Fix Critical zlib Vulnerability in smith-replica

#### Phase 1: Baseline Verification [checkpoint: ccfdd04]
- [x] **Task: Confirm baseline failure**
    - [x] Run `make build-local` to ensure a fresh local image is available.
    - [x] Run `make trivy-scan-local` and confirm the `smith-replica:local` failure for CVE-2026-22184.
- [x] **Task: Conductor - User Manual Verification 'Baseline Verification' (Protocol in workflow.md)**

#### Phase 2: Remediation [checkpoint: 5bddcd5]
- [x] **Task: Update base image in Dockerfile**
    - [x] Inspect `docker/replica.Dockerfile` to identify the current base image version.
    - [x] Update the base image to a version known to include the `zlib` patch (e.g., Alpine 3.21.3 -> newer, or latest Go base).
- [x] **Task: Rebuild the image**
    - [x] Run `make replica-build-local` to build the image with the new base.
- [x] **Task: Conductor - User Manual Verification 'Remediation' (Protocol in workflow.md)**

#### Phase 3: Final Verification
- [x] **Task: Run security scan**
    - [x] Run `make trivy-scan-local` and verify that `smith-replica:local` no longer reports CRITICAL vulnerabilities.
- [x] **Task: Run functional smoke tests**
    - [x] Run `make test-acceptance-smoke` to ensure the replica remains functional after the base image change.
- [ ] **Task: Conductor - User Manual Verification 'Final Verification' (Protocol in workflow.md)**
