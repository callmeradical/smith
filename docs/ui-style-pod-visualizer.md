# Smith Operator Console UI Style Contract

Reference: https://github.com/callmeradical/pod-visualizer

This project should match the visual language and interaction style of Pod Visualizer while keeping Smith-specific information architecture.

## Visual System

- Theme: dark-first, matte black background with translucent panels.
- Surface treatment: glassy cards (`rgba(255,255,255,0.05-0.10)`) with subtle borders.
- Accent colors:
  - Success/healthy: `#10b981`
  - Warning/pending: `#f59e0b`
  - Error/failed: `#ef4444`
  - Secondary accent: `#06b6d4`
- Typography: system sans stack; compact metrics with tabular numbers.
- Density: compact card grid with high information density.

## Layout Requirements

- Sticky top control bar with:
  - product title + connection/status dot
  - primary filters/selectors
  - refresh action
  - auto-refresh toggle
- Stats summary bar directly below top bar.
- Main area as responsive grid of status cards.
- Selected anomaly should open/attach a live terminal-style journal stream panel.

## Component Style Requirements

- Cards:
  - rounded (8-12px), low-contrast borders, translucent background
  - hover raises card slightly and increases border/background contrast
  - top-edge gradient reveal animation on hover
- Status badges:
  - uppercase micro-labels with tinted backgrounds per state
- Status indicators:
  - compact block/dot indicators for readiness and transitions
  - glow or pulse for state changes
- Controls:
  - low-profile buttons/selects matching translucent dark theme
  - explicit keyboard focus outlines

## Motion and Feedback

- Use restrained but visible motion:
  - card enter/exit animations
  - status transition pulse/glow
  - spinner for loading
- Respect `prefers-reduced-motion`.

## Responsiveness and Accessibility

- Must render cleanly at desktop, tablet, and mobile widths.
- Mobile behavior:
  - controls wrap in top bar
  - card grid reduces to single column at narrow widths
- Include high contrast and keyboard focus states.

## Non-Goals

- Do not clone Pod Visualizer markup directly.
- Do not carry over pod-specific naming; map style only to Smith concepts (anomalies, replicas, journals, overrides).

## Implementation Notes

- Keep design tokens centralized as CSS variables (or theme tokens if using component library).
- Start from this contract for:
  - `td-113f99` (Grid + Live Journal)
  - `td-d12fd7` (Loop policy config UI)
  - `td-59f69d` (Manual override UI)
