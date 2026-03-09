# Kubernetes Secrets Encryption Provider Runbook

This runbook hardens Smith credential storage by enabling Kubernetes API server encryption at rest for `Secret` objects.

## Scope

- Protects Kubernetes `Secret` data stored in etcd.
- Applies to Smith runtime secrets, including provider auth secret data (for example `smith-smith-auth-store`).
- Complements (does not replace) RBAC and network policy.

## Recommended Provider Order

Use one of these provider strategies:

1. Production: `kms` (KMS v2 plugin) then `identity`.
2. Baseline/self-managed: `aescbc` then `identity`.

`identity` should stay last as a fallback provider.

## EncryptionConfiguration Example (AES-CBC)

```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - aescbc:
          keys:
            - name: key1
              secret: <base64-32-byte-key>
      - identity: {}
```

Generate a 32-byte key:

```bash
head -c 32 /dev/urandom | base64
```

## Self-Managed Cluster Steps

1. Place `EncryptionConfiguration` on each control-plane node (for example `/etc/kubernetes/encryption-config.yaml`).
2. Configure kube-apiserver with:
   - `--encryption-provider-config=/etc/kubernetes/encryption-config.yaml`
3. Restart/roll the API server across control-plane nodes.
4. Verify API server health and normal secret reads.
5. Rewrite Smith secrets so they are re-encrypted with the active provider.

Rewrite Smith namespace secrets:

```bash
kubectl -n smith-system get secret -o name | while read -r s; do
  kubectl -n smith-system annotate --overwrite "$s" smith.dev/encryption-rewrite="$(date +%s)"
done
```

### Local k3d (Smith Dev Environment)

For the local `k3d` environment used by this repo, run:

```bash
./scripts/integration/enable-k3d-secrets-encryption.sh
```

This helper:
- creates/reuses `/etc/rancher/k3s/encryption-config.yaml` on `k3d-<cluster>-server-0`
- configures k3s API server with `encryption-provider-config=...`
- restarts `server-0` and `serverlb`
- rewrites `smith-system` secrets
- validates with a probe secret that plaintext/base64 marker values are not visible in the k3s datastore

## Managed Kubernetes

For managed control planes, enable provider-managed secret encryption (KMS-backed) in the cluster service:

- Enable etcd/secret encryption at rest.
- Select a customer-managed KMS key where supported.
- Confirm `secrets` are included in the encryption scope.

## Validation Checklist

1. `kubectl get secrets -A` returns normally.
2. Smith provider auth flow still works end-to-end.
3. No API server errors related to encryption provider config.
4. Backup/restore procedures are updated with KMS/encryption dependencies.

## Notes

- Key rotation is intentionally out of scope for this runbook.
- Keep encryption config and key material in your standard infrastructure secret management path.
