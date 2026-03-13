# Smith Helm Chart

## Install

```bash
helm upgrade --install smith ./helm/smith -n smith-system --create-namespace
```

## Environment Overlays

Prerequisites:

- Kubernetes context points to the target environment cluster.
- Runtime secret exists in target namespace (`secrets.existingSecret`) unless using local chart-managed secret flow.
- Image pull credentials are configured when using private registries (`global.imagePullSecrets`).

Single-command deploys by environment:

```bash
helm upgrade --install smith ./helm/smith -n smith-system -f ./helm/smith/values/local.yaml
helm upgrade --install smith ./helm/smith -n smith-system -f ./helm/smith/values/staging.yaml
helm upgrade --install smith ./helm/smith -n smith-system -f ./helm/smith/values/prod.yaml
```

Equivalent make targets:

```bash
make deploy-local
make deploy-staging
make deploy-prod
```

Local note: `make deploy-local` builds the Smith images locally and imports them into the k3d cluster before Helm runs, so the default `ghcr.io/smith/*:v0.1.0` refs resolve without a remote registry pull.

## Values Contract

- Schema file: `helm/smith/values.schema.json`
- Validation: `helm lint ./helm/smith` (fails on invalid values)

Required value groups:
- `global.environment`, `global.featureFlags`, `global.imagePullSecrets`
- `etcd.endpoints`, `etcd.tls.*`
- `secrets.*` (existing secret reference or optional chart-managed secret path)
- `core.*` (image/serviceAccount/resources/env + `loopPolicy.*`)
- `core.autoscaling.*` (HPA settings and stabilization policy)
- `core.replicaTemplate.*` (replica Job defaults forwarded to Agent Core: serviceAccount/resources/nodeSelector/tolerations/env)
- `api.*` (image/service/serviceAccount/resources/env + `autoscaling.*`)
- `console.*` (image/service/serviceAccount/resources/env + `autoscaling.*`)
- `rbac.create`

Loop policy defaults:
- `core.loopPolicy.maxAttempts: 3`
- `core.loopPolicy.backoffInitial: 5s`
- `core.loopPolicy.backoffMax: 2m`
- `core.loopPolicy.timeout: 30m`
- `core.loopPolicy.terminateOnError: false`
- `core.replicaTemplate.serviceAccountName: ""` (defaults to `<release>-smith-replica` when empty)

Image tag defaults:
- `core.image.tag: v0.1.0`
- `api.image.tag: v0.1.0`
- `console.image.tag: v0.1.0`
- See `docs/image-tagging-versioning.md` for semver/SHA/branch policy and rollback matrix.
- Private registries: set `global.imagePullSecrets` and all component pods inherit it.

Environment examples:
- `values/local.yaml`: single-replica local baseline, faster retry cadence.
- `values/staging.yaml`: pre-prod sizing with moderate retry bounds.
- `values/prod.yaml`: production sizing with stricter retry/timeout defaults and HPA enabled.

Minimal profile deltas:

| Profile | Core/API/Console replicas | Secret mode | Autoscaling |
| --- | --- | --- | --- |
| `local` | `1/1/1` | chart-managed (`secrets.create=true`) | disabled |
| `staging` | `2/2/1` | pre-created (`secrets.existingSecret`) | disabled |
| `prod` | `3/3/2` | pre-created (`secrets.existingSecret`) | enabled for all components |

Autoscaling behavior:
- HPAs are emitted from `templates/hpa.yaml` when `<component>.autoscaling.enabled=true`.
- CPU and memory utilization targets are set per component in values.
- Stabilization windows are configurable through `scaleUpStabilizationWindowSeconds` and `scaleDownStabilizationWindowSeconds`.

## Secrets Strategy

Preferred path (pre-created secret):
- Set `secrets.create=false`.
- Set `secrets.existingSecret=<name>`.
- Secret must contain keys matching `secrets.keys.gitPat` and `secrets.keys.runtimeCredentials`.

Optional chart-managed path:
- Set `secrets.create=true` and keep `secrets.existingSecret=""`.
- Provide bootstrap values under `secrets.managed.*`.
- This is intended for local/bootstrap usage; production should prefer external secret provisioning.

Runtime injection:
- Core and API receive `SMITH_GIT_PAT` and `SMITH_RUNTIME_CREDENTIALS` from the configured secret.
- Console receives `SMITH_RUNTIME_CREDENTIALS` from the configured secret.
- Chart templates do not print secret values in `NOTES.txt`.

## Rotation Procedure

Pre-created secret rotation:
1. Create/update a replacement secret with the same key names.
2. Keep `secrets.existingSecret` stable (or update to new secret name).
3. Run `helm upgrade --install` with the same values overlay.
4. Restart Deployments or let rollout happen from config drift to pick up updated secret data.

Chart-managed secret rotation:
1. Update `secrets.managed.*` values through your secure delivery path.
2. Run `helm upgrade --install`.
3. Confirm new Secret revision and Deployment rollout.

Operational note:
- Avoid passing secret values through shell history or plaintext checked-in values files.
- Prefer CI secret stores or sealed/external secret controllers for production rotation.
- Enable Kubernetes Secret encryption at rest for production clusters (see `docs/kubernetes-secrets-encryption-provider-runbook.md`).
- See `docs/helm-upgrade-rollback-runbook.md` for upgrade/rollback and zero-downtime procedures.

## Notes

This scaffold deploys the Smith control plane components. Replica worker Jobs are created dynamically by `smith-core`.
