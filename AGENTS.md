## MANDATORY: Use td for Task Management

You must run td usage --new-session at conversation start (or after /clear) to see current work.
Use td usage -q for subsequent reads.

## Base Image Validation

Build the loop base image:

```bash
docker build -t loop-base:local .
```

Run smoke checks:

```bash
./scripts/check-base-tooling-smoke.sh loop-base:local
./scripts/check-base-internal-binaries-smoke.sh loop-base:local
./scripts/check-base-skills-mount-smoke.sh loop-base:local
```

Run full reproducible quality gates (ordered, fail-fast):

```bash
./scripts/run-base-quality-gates.sh loop-base:local
```

Required local tooling for lint/security gates:

```bash
brew install hadolint shellcheck trivy syft
```

Negative trivy gate validation (expected failure inside trivy check):

```bash
./scripts/check-trivy-critical-negative.sh knqyf263/vuln-image:1.2.3
```
