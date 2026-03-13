# Implementation Plan: Fix Critical zlib Vulnerability in smith-replica

#### Phase 1: Baseline Verification
- [x] **Task: Confirm baseline failure**
    - [ ] Run `make build-local` to ensure a fresh local image is available.
    - [ ] Run `make trivy-scan-local` and confirm the `smith-replica:local` failure for CVE-2026-22184.
- [ ] **Task: Conductor - User Manual Verification 'Baseline Verification' (Protocol in workflow.md)**

#### Phase 2: Remediation
- [ ] **Task: Update base image in Dockerfile**
    - [ ] Inspect `docker/replica.Dockerfile` to identify the current base image version.
    - [ ] Update the base image to a version known to include the `zlib` patch (e.g., Alpine 3.21.3 -> newer, or latest Go base).
- [ ] **Task: Rebuild the image**
    - [ ] Run `make replica-build-local` to build the image with the new base.
- [ ] **Task: Conductor - User Manual Verification 'Remediation' (Protocol in workflow.md)**

#### Phase 3: Final Verification
- [ ] **Task: Run security scan**
    - [ ] Run `make trivy-scan-local` and verify that `smith-replica:local` no longer reports CRITICAL vulnerabilities.
- [ ] **Task: Run functional smoke tests**
    - [ ] Run `make test-acceptance-smoke` to ensure the replica remains functional after the base image change.
- [ ] **Task: Conductor - User Manual Verification 'Final Verification' (Protocol in workflow.md)**
