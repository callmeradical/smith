# Plan: Integrate Trivy Vulnerability Scan with GitHub Issues

## Objective
Modify the CI pipeline to automatically create GitHub issues when Trivy detects vulnerabilities in built images and subsequently fail the build.

## Key Changes

### 1. Update Permissions
In `.github/workflows/ci.yml`, ensure the `GITHUB_TOKEN` has `issues: write` and `contents: read` permissions.

### 2. Modify Trivy Scan Step
Update the Trivy scan in the `build-images` job:
- Change `format` from `table` to `json`.
- Output the scan result to a file (e.g., `trivy-results-${{ matrix.image }}.json`).
- Keep `exit-code: '0'` for the initial scan so we can process the results.

### 3. Add Issue Creation Step
Add a new step using `actions/github-script` to:
- Read the Trivy JSON output.
- Check for vulnerabilities.
- If vulnerabilities are found:
    - Create or update a GitHub issue for each critical vulnerability.
    - Set an output or variable to signal build failure.

### 4. Fail the Build
Add a step to check for the failure signal and exit with code 1 if vulnerabilities were found.

## Implementation Steps

1.  **Modify `.github/workflows/ci.yml`**:
    - Update `permissions` block.
    - Update `Scan image with Trivy` step.
    - Add `Create GitHub Issues for vulnerabilities` step.
    - Add `Enforce security gate` step.

## Verification
- Push a change that triggers the CI.
- Monitor the `build-images` job.
- Verify that issues are created if vulnerabilities are present.
- Verify that the build fails if vulnerabilities are present.
