#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

go test ./internal/source/completion/... -run 'TestExecute(CommitFailureIsRetryable|SyncFailureCompensates|SyncFailureAndRevertFailureSignalsCompensationRequired)'
go test ./internal/source/reconcile/... -run 'TestReconcileOne'
