## MANDATORY: Use td for Task Management

You must run td usage --new-session at conversation start (or after /clear) to see current work.
Use td usage -q for subsequent reads.

## Base Image Validation

Build the loop base image:

```bash
docker build -t loop-base:local .
```

Run tooling smoke checks:

```bash
./scripts/check-base-tooling-smoke.sh loop-base:local
./scripts/check-base-internal-binaries-smoke.sh loop-base:local
```
