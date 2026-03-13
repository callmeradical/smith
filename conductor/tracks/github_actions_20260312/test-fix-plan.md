# Plan: Resolve Smith Console E2E Test Failures

## Objective
Fix the recurring Playwright test failures in the Smith Console by aligning API path handling, resolving visibility/layout issues, and ensuring robust state synchronization between the JS modules and the DOM.

## Key Files & Context
- `console/js/modules/api.js`: Handles all API communication.
- `console/js/modules/terminal.js`: Manages pod view and terminal state.
- `console/index.html`: Main UI structure.
- `test/playwright/console.spec.js`: E2E tests for the console.

## Implementation Steps

### Phase 1: API Path Harmonization
1.  **Modify `console/js/modules/api.js`**:
    *   Simplify `fetchJSON` and `requestJSON`.
    *   Remove all conditional prepending of `/api`.
    *   If `apiBase` is set, prepend it. Otherwise, use the path as-is.
2.  **Modify `test/playwright/console.spec.js`**:
    *   Update all `page.route` patterns to match the paths exactly as requested by the app (e.g., starting with `/v1/`).
    *   Ensure `mockConsoleApi` sets `apiBaseUrl: ""` in the init script.

### Phase 2: UI Layout and Visibility
1.  **Modify `console/index.html`**:
    *   Ensure all `.page` sections are correctly closed and not nested.
    *   Move all modals/drawers (`#provider-config-panel`, `#pod-create-panel`, `#project-config-panel`, `#doc-create-panel`) to the end of the `<body>`, outside of any `<main>` or `.page` containers.
    *   Verify all `sidebar-toggle` buttons are consistently placed.
2.  **Modify `console/css/stylesheet.css`**:
    *   Ensure `.provider-drawer.open` and other active states have `visibility: visible` and `opacity: 1`.

### Phase 3: State and DOM Synchronization
1.  **Modify `console/js/modules/terminal.js`**:
    *   Ensure `syncPodViewActions` updates ALL relevant elements (text, disabled states, classes).
    *   Correctly handle "Attached. Ready for commands." vs. "Press Enter or Run".
2.  **Modify `console/js/modules/pods.js`**:
    *   Ensure `renderSelectedLoop` clears stale data and immediately calls `syncPodViewActions`.
3.  **Modify `console/js/modules/events.js`**:
    *   Add missing input listeners to trigger UI syncs (e.g., when typing in the command field).

### Phase 4: Test Stability
1.  **Modify `test/playwright/console.spec.js`**:
    *   Ensure every test calls `window.refreshLoops()` and `window.renderProviderList()` after `page.goto('/')`.
    *   Wait for selectors to be visible before interacting.

## Verification & Testing
1.  Run all Playwright tests: `npx playwright test test/playwright/console.spec.js`
2.  Verify manual operation of the console in a browser.
3.  Check browser console for any failed fetches or JS errors.
