# Track Specification: Fix Critical Vulnerabilities in smith-console

#### Overview
The `smith-console:ci` image currently contains 4 critical vulnerabilities reported via Trivy (Issue #156). These include vulnerabilities in `libcrypto3`, `libssl3` (CVE-2025-15467), and `libxml2` (CVE-2025-49794, CVE-2025-49796). This track aims to remediate these by updating the base image to a stable, patched version.

#### Functional Requirements
- **Base Image Update:** Update `docker/console.Dockerfile` to use `alpine:3.20` or a derived image (like `nginx:alpine3.20`) that contains the fixed versions of the affected libraries.
- **Library Patching:** If the base image update alone does not resolve all findings, perform explicit `apk upgrade` for the specific vulnerable packages (`libcrypto3`, `libssl3`, `libxml2`).
- **Image Rebuild:** Ensure the `smith-console:local` image builds successfully after the change.

#### Non-Functional Requirements
- **Security:** The resulting `smith-console:local` image must have 0 `CRITICAL` vulnerabilities when scanned by Trivy.
- **Functionality:** The operator console must remain fully functional (verified by existing E2E tests).

#### Acceptance Criteria
- [ ] `make build-local` completes without errors.
- [ ] `make trivy-scan-local` confirms 0 `CRITICAL` vulnerabilities in `smith-console:local`.
- [ ] `make test-frontend` passes successfully.

#### Out of Scope
- Fixing vulnerabilities in other images (e.g., `smith-replica`), which are handled in separate tracks.
- Upgrading unrelated dependencies unless necessary for the base image migration.
