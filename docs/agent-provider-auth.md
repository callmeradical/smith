# Agent Provider + Codex Authentication Design

## Goal

Define how Smith selects and talks to agent providers, starting with Codex, and how users authenticate in a way consistent with current Codex CLI behavior.

## Scope

- Initial provider: Codex.
- Future providers: add via provider adapter interface.
- Authentication UX: browser-based/device-style login flow similar to Codex CLI, with secure token lifecycle handling.

## Architecture

### 1. Provider Registry

Smith Agent Core exposes a provider registry:
- `provider_id` (e.g., `codex`)
- capabilities (streaming, tools, max context, reasoning controls)
- auth mode and token requirements
- model catalog metadata

Loop definition includes:
- `provider_id`
- `model`
- provider-specific options (validated against provider schema)

Default provider for MVP: `codex`.

### 2. Provider Adapter Interface

Each provider implements a common interface:
- `CreateSession(ctx, request)`
- `SendTurn(ctx, session, input)`
- `StreamEvents(ctx, session)`
- `CloseSession(ctx, session)`
- `ValidateConfig(config)`

Codex adapter handles OpenAI/Codex-specific request/response mapping and event normalization into Smith journal records.

### 3. Codex Auth Flow (CLI-Compatible Pattern)

Authentication should mirror Codex CLI user experience:
1. Operator chooses "Connect Codex" in UI/CLI.
2. System initiates browser/device-style login flow.
3. On success, Smith receives scoped tokens.
4. Tokens are stored in secure backend (not plaintext in etcd).
5. Runtime retrieves short-lived access token for provider calls.
6. Refresh flow runs before expiry; failures emit actionable operator alerts.

## Token and Secret Handling

- Never store raw tokens in etcd task keys.
- Store provider credentials in secure secret store (Kubernetes Secret initially; external secret manager pluggable).
- Encrypt at rest per platform defaults + optional KMS integration.
- Redact tokens from logs/journal/errors.
- Track token metadata (issuer, expiry, scopes, last-refresh) for observability.

## Runtime Behavior

- On loop start, Agent Core resolves provider credentials by workspace/project scope.
- If auth missing/expired and non-refreshable:
  - mark anomaly as blocked with auth reason
  - surface reconnect action in Operator Console
- If refresh succeeds, continue without interrupting in-flight loop.

## Operator Experience

- Console includes provider account panel:
  - connect/disconnect Codex account
  - current auth status (connected/expired/error)
  - last refresh time and scope summary
- Loop creation form includes provider/model selector.

## Auditing

Audit events for:
- login initiated/succeeded/failed
- token refresh succeeded/failed
- credential revocation/disconnect
- provider call auth errors

Audit entries include actor, timestamp, provider, scope, and correlation ID.

## MVP Decisions

- Supported provider in MVP: Codex only.
- Auth mode in MVP: Codex CLI-style browser/device login flow.
- Credential backend in MVP: Kubernetes Secrets.
- Deferred: multi-provider routing policies and external secrets backends.

## Non-Goals (MVP)

- Full multi-tenant identity federation.
- Bring-your-own OAuth provider framework.
- Fine-grained per-turn model arbitration across providers.


## Implemented Surface (Current)

### Provider Auth Lifecycle Components

- `internal/source/provider/AuthManager`
- `internal/source/provider/TokenStore` interface
- `internal/source/provider/FileTokenStore` (0600 permissions)
- `internal/source/provider/MockDeviceAuthClient` (device-style flow harness)

Lifecycle behavior:
- Connect start -> device code session issued.
- Connect complete -> token stored in secure backend.
- Runtime token check -> refresh before expiry.
- Refresh/auth failures -> actionable errors (`ErrAuthRequired`, `ErrTokenExpired`, `ErrTokenRefresh`).

### API Endpoints (smith-api)

- `POST /v1/auth/codex/connect/start`
- `POST /v1/auth/codex/connect/complete`
- `GET /v1/auth/codex/status`
- `POST /v1/auth/codex/disconnect`

Environment variables:
- `SMITH_AUTH_STORE_PATH` for auth token storage path.
- `SMITH_OPERATOR_TOKEN` for operator auth gating.

Auth lifecycle actions emit audit records through Smith audit append path.
