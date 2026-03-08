# Helm Upgrade, Rollback, and Zero-Downtime Runbook

This runbook covers operational rollout and recovery for Smith Helm releases.

## Scope

- Chart: `helm/smith`
- Environments: `local`, `staging`, `prod`
- Profiles: `helm/smith/values/{local,staging,prod}.yaml`

## Compatibility and Ordering Constraints

1. Use immutable image tags for production rollouts (`v*` or `sha-*`), not branch tags.
2. Keep all control-plane components (`core`, `api`, `console`) on the same chart revision.
3. Apply schema-compatible releases only:
   - state/journal/handoff are `v1` records;
   - rolling versions must continue to read existing `v1` data.
4. Rotate secrets before upgrade when changing secret names referenced by values.

## Preflight Checklist

1. Confirm target context and namespace:
   - `kubectl config current-context`
   - `kubectl get ns smith-system`
2. Validate chart and selected profile:
   - `helm lint ./helm/smith -f ./helm/smith/values/<profile>.yaml`
3. Render manifests for review:
   - `helm template smith ./helm/smith -n smith-system -f ./helm/smith/values/<profile>.yaml >/tmp/smith-<profile>.yaml`
4. Verify runtime secret exists for non-local profiles:
   - `kubectl -n smith-system get secret smith-stage-runtime` (staging)
   - `kubectl -n smith-system get secret smith-prod-runtime` (prod)
5. Capture release baseline:
   - `helm -n smith-system list`
   - `helm -n smith-system history smith`

## Upgrade Procedure

Use one profile per rollout:

```bash
helm upgrade --install smith ./helm/smith \
  -n smith-system \
  --create-namespace \
  -f ./helm/smith/values/staging.yaml \
  --wait --timeout 10m
```

Post-upgrade checks:

1. `helm -n smith-system status smith`
2. `kubectl -n smith-system get deploy,pod,svc`
3. `kubectl -n smith-system rollout status deploy/smith-smith-core --timeout=5m`
4. `kubectl -n smith-system rollout status deploy/smith-smith-api --timeout=5m`
5. `kubectl -n smith-system rollout status deploy/smith-smith-console --timeout=5m`

## Zero-Downtime Guidance

1. Prefer rolling upgrades with `--wait` and generous timeout.
2. Keep at least two API replicas in staging/prod during rollouts.
3. Avoid simultaneous disruptive changes:
   - do not rotate secrets and chart structure in one deploy;
   - do not change autoscaling bounds and resource limits in the same window.
4. Roll environments in order:
   - `local` -> `staging` -> `prod`.
5. Watch live service health during rollout:
   - `kubectl -n smith-system get pods -w`
   - verify `/readyz` on API service from an internal probe job or port-forward.

## Rollback Procedure

1. Identify target revision:
   - `helm -n smith-system history smith`
2. Roll back:

```bash
helm -n smith-system rollback smith <REVISION> --wait --timeout 10m
```

3. Re-run post-upgrade checks (`status`, `rollout status`, basic API health).
4. Confirm image tags and values match expected rollback baseline.
5. Record incident details and failed revision in ops notes.

## Known Failure Modes and Recovery

1. `ImagePullBackOff`
   - Cause: invalid tag or missing pull secret.
   - Recovery: fix image tag/secret, then `helm upgrade` again.
2. Pods fail readiness after secret changes
   - Cause: missing key names in runtime secret.
   - Recovery: restore secret keys expected by `secrets.keys.*`; redeploy.
3. HPA thrash during rollout
   - Cause: overly aggressive stabilization/window settings.
   - Recovery: revert autoscaling changes or rollback release revision.
4. API unavailable during rollout
   - Cause: single replica + restart window.
   - Recovery: raise replica count in profile before next upgrade window.

## Non-Prod Validation Record

Validation run date: 2026-03-08

Executed successfully:

1. `helm lint ./helm/smith -f ./helm/smith/values/local.yaml`
2. `helm lint ./helm/smith -f ./helm/smith/values/staging.yaml`
3. `helm lint ./helm/smith -f ./helm/smith/values/prod.yaml`
4. `helm template smith ./helm/smith -n smith-system -f ./helm/smith/values/staging.yaml >/tmp/smith-staging.yaml`

This validates chart/profile integrity in non-prod tooling before cluster rollout.
