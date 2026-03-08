# Docs Site: Zensical + GitHub Pages

## Local Build

Install Zensical and build the docs site:

```bash
pip install zensical
./scripts/docs/quality-check.sh
```

Output is written to `site/`.

The quality-check script validates local markdown links first, then runs `zensical build`.

## Base URL and Path Configuration

- CI derives `site_url` as a project-site URL (`https://<owner>.github.io/<repo>/`) from the active repository and rewrites `zensical.toml` during the workflow run.
- For local builds, set `site_url` in `zensical.toml` to your target URL and keep the trailing slash.

## CI + Deployment

Workflow file: `.github/workflows/docs-pages.yml`

Behavior:

- Pull requests: build validation only.
- Push to `main`: build, upload Pages artifact, and deploy to GitHub Pages.

## GitHub Pages Settings

In repository settings:

1. `Pages` -> `Build and deployment` -> `Source`: `GitHub Actions`.
2. Ensure workflow permissions allow `pages: write` and `id-token: write`.
