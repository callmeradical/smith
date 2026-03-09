# Local Make Quickstart: Deploy and Run a Loop

This quickstart is copy/paste oriented for a fresh local machine.

## 1. Prerequisites and Bootstrap

```bash
make doctor
make bootstrap
```

If `make doctor` fails, follow the remediation lines printed in output.

## 2. Start Local Cluster and Deploy Smith

```bash
make cluster-up
make cluster-health
make build-local
make deploy-local
```

`make deploy-local` now builds the Smith container images locally and imports them into the `k3d` cluster before running `helm upgrade`, so the control-plane pods do not need to pull `ghcr.io/smith/*` during local development.

## 3. Expose API Locally

In a separate terminal:

```bash
kubectl -n smith-system port-forward svc/smith-smith-api 8080:8080
```

Keep this running while issuing `smithctl` commands.

## 4. Create and Inspect a Sample Loop

```bash
smithctl --server http://127.0.0.1:8080 --output json loop create \
  --title "Quickstart loop" \
  --description "Validate local make workflow" \
  --source-type interactive \
  --source-ref terminal/quickstart-01
```

Capture the returned `loop_id`, then:

```bash
smithctl --server http://127.0.0.1:8080 --output json loop get <loop_id>
smithctl --server http://127.0.0.1:8080 --output json loop logs <loop_id>
```

## 5. Run Local Validation Suites

```bash
make test
make test-e2e
make test-integration
```

Each target prints an `artifacts:` path for debugging evidence.

## 6. Stop and Clean Up

```bash
make undeploy-local
make cluster-down
```

## Troubleshooting

- `kubectl cannot reach a cluster context`:
  - run `make cluster-up`, then `make cluster-health`.
- `missing required command` in doctor:
  - run `make bootstrap` and re-run `make doctor`.
- API calls fail on `127.0.0.1:8080`:
  - verify `kubectl port-forward` is active.
- e2e/integration failures:
  - inspect the artifact path printed by make targets.
