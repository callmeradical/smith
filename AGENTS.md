## MANDATORY: Use td for Task Management

You must run td usage --new-session at conversation start (or after /clear) to see current work.
Use td usage -q for subsequent reads.

## Git Hooks Workflow
- We use `act` to run CI jobs locally for git hooks.
- **MANDATORY**: Ensure `act` and Docker are installed.
- `pre-commit`: Runs the `lint-and-check` job via `act`.
- `pre-push`: Runs the parallel unit tests (`go-unit-tests`, `node-unit-tests`, `playwright-tests`) via `act`.
- To run the full local CI suite manually: `make ci-local-act`.

## Build and Test Notes
- Frontend dependencies live under `frontend/`; run `npm --prefix frontend install` before `npm --prefix frontend run build` or `npm --prefix frontend run check`.
