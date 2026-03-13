## MANDATORY: Use td for Task Management

You must run td usage --new-session at conversation start (or after /clear) to see current work.
Use td usage -q for subsequent reads.

## Git Hooks Workflow
- Hooks run in Docker by default (`SMITH_HOOKS_IN_DOCKER=1`).
- `pre-commit`: Fast checks (Go build/vet, Helm lint).
- `pre-push`: Comprehensive checks (Go unit/acceptance tests, Frontend build/Playwright, Docs check).
- If you need to force host execution: `export SMITH_HOOKS_IN_DOCKER=0`.
- To build the hooks image: `make hooks-image-build`.
