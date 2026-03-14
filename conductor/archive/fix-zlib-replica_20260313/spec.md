# Track Specification: Fix Critical zlib Vulnerability in smith-replica

#### Overview
The `smith-replica:local` image currently contains a critical vulnerability in the `zlib` library (CVE-2026-22184). This track aims to remediate this vulnerability by updating the base image used in `docker/replica.Dockerfile` to a version that includes the patched library (v1.3.2-r0 or newer).

#### Functional Requirements
- **Base Image Update:** Identify and update the base image in `docker/replica.Dockerfile` to a version that includes the latest security patches for `zlib`.
- **Image Rebuild:** Ensure the `smith-replica:local` image can be rebuilt successfully with the new base image.

#### Non-Functional Requirements
- **Compatibility:** The Smith replica binary must remain fully functional and compatible with the new base image.
- **Security:** The resulting image must not contain any `CRITICAL` vulnerabilities as detected by Trivy.

#### Acceptance Criteria
- [ ] `make build-local` completes without errors.
- [ ] `make trivy-scan-local` confirms that `smith-replica:local` has 0 `CRITICAL` vulnerabilities.
- [ ] The `smith-replica` service starts correctly in the local environment (verified via existing smoke tests).

#### Out of Scope
- Fixing vulnerabilities in other Smith images (e.g., `smith-console`), which will be tracked separately.
- Upgrading other libraries unless required by the base image update.
