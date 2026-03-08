# Journal Retention and Archival Policy

Smith keeps journal records append-only by default (`keep_forever`) and does not archive externally.

## Defaults

- `retention_mode = keep_forever`
- `retention_ttl = 0`
- `archive_mode = none`
- `archive_bucket = ""`

## Feature-Gated Operator Overrides

Enable override parsing in `smith-core`:

```bash
SMITH_JOURNAL_POLICY_CONFIG_ENABLED=true
```

Supported overrides:

- `SMITH_JOURNAL_RETENTION_MODE`: `keep_forever` or `ttl`
- `SMITH_JOURNAL_RETENTION_TTL`: duration string (required when mode is `ttl`, for example `168h`)
- `SMITH_JOURNAL_ARCHIVE_MODE`: `none` or `s3`
- `SMITH_JOURNAL_ARCHIVE_BUCKET`: required when archive mode is `s3`

## Validation Rules

- `keep_forever` requires `retention_ttl=0`.
- `ttl` requires `retention_ttl > 0`.
- `archive_mode=none` requires no archive bucket.
- `archive_mode=s3` requires `SMITH_JOURNAL_ARCHIVE_BUCKET`.

Invalid combinations fail `smith-core` startup so misconfiguration is caught before loop execution.

## Runtime Traceability

When feature-gated overrides are enabled, core and replica include journal policy metadata in loop journal entries:

- `journal_retention_mode`
- `journal_retention_ttl`
- `journal_archive_mode`
- `journal_archive_bucket` (when set)
