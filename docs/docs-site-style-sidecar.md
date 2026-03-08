# Smith Docs Site Style Contract (Sidecar-Inspired)

Reference style: https://sidecar.haplab.com/

Goal: build Smith docs with a visual style similar to Sidecar docs while keeping Smith branding, structure, and copy original.

## Visual Direction

- Clean documentation-first layout with high readability.
- Dark-first presentation with tasteful contrast and soft panel surfaces.
- Monospace-forward accents for commands, keybinds, and terminal snippets.
- Lightweight, fast-loading pages with minimal visual clutter.

## Layout Requirements

- Sticky top navigation with:
  - Smith brand/title
  - Docs section links
  - Theme selector/toggle
  - GitHub link
- Left sidebar navigation for docs sections on desktop.
- Main content column optimized for long-form technical docs.
- On mobile, collapsible nav and readable typography scaling.

## Component Requirements

- Prominent code/terminal blocks for commands and workflows.
- Compact callout blocks for notes/warnings/decisions.
- Tables styled for requirement/traceability matrices.
- Search entrypoint visible in top nav or sidebar.

## Theme Requirements

- Include at least one dark and one light theme.
- Optional additional presets inspired by Sidecar's docs theme choices.
- Persist theme choice per user session/local storage.

## Motion and Interaction

- Subtle transitions only (theme switch, hover states, nav reveal).
- Respect reduced-motion preferences.

## Accessibility

- Keyboard-navigable sidebar and top navigation.
- Visible focus states for interactive elements.
- Color contrast suitable for code-heavy content.

## Content IA (Initial)

- Getting Started
- Architecture
- Requirements (FR/NFR)
- Traceability Matrix
- Deployment (Helm)
- Agent Providers & Authentication (Codex)
- Operations (Backup/Restore, Runbooks)

## Non-Goals

- Do not reuse Sidecar branding assets or copy text/content structure verbatim.
- Do not optimize for marketing pages before core docs information architecture is complete.

