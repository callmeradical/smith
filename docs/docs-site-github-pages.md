# Docs Site: Zensical + GitHub Pages

## Local Build

Install Zensical and build the docs site:

```bash
pip install zensical
./scripts/docs/quality-check.sh
```

Output is written to `site/`.

The quality-check script validates local markdown links, validates lifecycle metadata,
builds the full source docs site, and then builds the public GitHub Pages site.

## Base URL and Path Configuration

- CI pins `site_url` to `https://callmeradical.github.io/smith/` and rewrites `zensical.toml` during the workflow run.
- For local builds, set `site_url` in `zensical.toml` to your target URL and keep the trailing slash.

## CI + Deployment

Workflow file: `.github/workflows/docs-pages.yml`

Behavior:

- Pull requests: build validation only.
- Push to `main`: build, upload Pages artifact, and deploy to GitHub Pages.

Public publishing rules:

- GitHub Pages publishes a filtered public site build.
- Internal workflow content under `docs/planning/` and `docs/prds/` is kept in the source tree but excluded from the published site.
- `docs/docs-to-prd-lifecycle.md` is also excluded from the public build because it documents internal planning automation rather than product-facing behavior.

## GitHub Pages Settings

In repository settings:

1. `Pages` -> `Build and deployment` -> `Source`: `GitHub Actions`.
2. Ensure workflow permissions allow `pages: write` and `id-token: write`.
