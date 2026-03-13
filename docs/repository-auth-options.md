# Repository Auth Options (Replica Git Access)

## Goal

Provide secure alternatives for Replica Git operations beyond PAT-only authentication.

## Supported Providers

- `pat`: Personal Access Token from Kubernetes Secret.
- `github_app`: GitHub App installation token flow using App ID, Installation ID, and private key secret.
- `ssh`: SSH private key auth from Kubernetes Secret (with optional known_hosts secret).

## Feature Flag

`github_app` mode is guarded by a feature flag in job configuration:

- `EnableGitHubAppAuth = true`

`ssh` mode is guarded by a feature flag in job configuration:

- `EnableSSHAuth = true`

If the flag is not enabled, `github_app` requests are rejected with validation error.

## Job Configuration Contract

Replica job request auth section:

- `GitAuth.Provider = "pat"`
  - requires `PATSecretName`, `PATSecretKey`
- `GitAuth.Provider = "github_app"`
  - requires `EnableGitHubAppAuth = true`
  - requires `GitHubApp.AppID`
  - requires `GitHubApp.InstallationID`
  - requires `GitHubApp.PrivateKeySecretName`
  - requires `GitHubApp.PrivateKeySecretKey`
- `GitAuth.Provider = "ssh"`
  - requires `EnableSSHAuth = true`
  - requires `SSH.PrivateKeySecretName`
  - requires `SSH.PrivateKeySecretKey`
  - optional `SSH.KnownHostsSecretName`, `SSH.KnownHostsSecretKey`

## Runtime Environment Variables

When configured, the generator emits:

- common: `SMITH_GIT_AUTH_PROVIDER`
- PAT mode:
  - `SMITH_GIT_PAT` (secret ref)
- GitHub App mode:
  - `SMITH_GITHUB_APP_ID`
  - `SMITH_GITHUB_APP_INSTALLATION_ID`
  - `SMITH_GITHUB_APP_PRIVATE_KEY` (secret ref)
- SSH mode:
  - `SMITH_GIT_SSH_PRIVATE_KEY` (secret ref)
  - `SMITH_GIT_SSH_KNOWN_HOSTS` (secret ref, optional)

## Custom Agent Identity

By default, Smith commits changes using the following identity:
- **User Name**: `smith-replica`
- **User Email**: `smith-replica@smith.io`

When using a PAT or other authentication methods, you can customize this identity to match a specific "Smith Service User" or a developer profile.

### Configuration

The identity is controlled via the following environment variables in the Replica Job:

- `SMITH_GIT_USER_NAME`: The display name for git commits (e.g., `Smith Bot [Acme]`).
- `SMITH_GIT_USER_EMAIL`: The email address for git commits (e.g., `smith@acme.corp`).

### Usage with PAT

If you are using a project PAT and want it to appear as a specific user, ensure these variables are set in your loop or project configuration. Smith's `completion` protocol will automatically run `git config user.name` and `git config user.email` within the workspace before committing.

---

## Rotation Guidance

PAT mode:

1. Create a new PAT with least-privilege repo scopes.
2. Update Kubernetes Secret referenced by `PATSecretName/PATSecretKey`.
3. Roll Replica Jobs to pick up the updated secret.
4. Revoke old PAT after rollout verification.

GitHub App mode:

1. Generate a new GitHub App private key.
2. Update Kubernetes Secret referenced by `PrivateKeySecretName/PrivateKeySecretKey`.
3. Restart or roll Replica workloads.
4. Delete old private key from GitHub App settings.

Operational recommendation:

- Prefer `github_app` mode for scoped, revocable installation-based access and shorter-lived credentials.
- Prefer `ssh` mode over PAT where Git host policy requires key-based access and strict host verification.

SSH mode key management:

1. Generate a new deploy key pair and register public key with least-privilege repo access.
2. Store private key in Kubernetes Secret referenced by `SSH.PrivateKeySecretName/PrivateKeySecretKey`.
3. Store pinned host keys in `known_hosts` secret and wire `KnownHostsSecret*` fields.
4. Roll Replica Jobs, verify clone/fetch success, then revoke old keys.
