# Docs Site: Zensical + GitHub Pages

## Local Build

Install Zensical and build the docs site:

```bash
pip install zensical
zensical build
```

Output is written to `site/`.

## Base URL and Path Configuration

`zensical.toml` must set `site_url` to the final GitHub Pages URL:

- User/organization site repo: `https://<org-or-user>.github.io/`
- Project site repo: `https://<org-or-user>.github.io/<repo>/`

For this repository, use the project-site form and include the trailing slash.

## CI + Deployment

Workflow file: `.github/workflows/docs-pages.yml`

Behavior:

- Pull requests: build validation only.
- Push to `main`: build, upload Pages artifact, and deploy to GitHub Pages.

## GitHub Pages Settings

In repository settings:

1. `Pages` -> `Build and deployment` -> `Source`: `GitHub Actions`.
2. Ensure workflow permissions allow `pages: write` and `id-token: write`.
