# Smith Helm Chart

## Install

```bash
helm upgrade --install smith ./helm/smith -n smith-system --create-namespace
```

## Environment Overlays

```bash
helm upgrade --install smith ./helm/smith -n smith-system -f ./helm/smith/values/local.yaml
helm upgrade --install smith ./helm/smith -n smith-system -f ./helm/smith/values/stage.yaml
helm upgrade --install smith ./helm/smith -n smith-system -f ./helm/smith/values/prod.yaml
```

## Required Values

- `etcd.endpoints`
- `core.image.repository`, `core.image.tag`
- `api.image.repository`, `api.image.tag`
- `console.image.repository`, `console.image.tag`

## Notes

This scaffold deploys the Smith control plane components. Replica worker Jobs are created dynamically by `smith-core`.
