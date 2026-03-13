/**
 * Shared test helpers for Svelte frontend Playwright tests.
 *
 * The Svelte app opens EventSource streams on layout mount and fetches
 * API data via /api/v1/... paths.  These helpers intercept both so
 * that tests run entirely against mocked data.
 */

/** Standard loop fixtures used across test suites. */
export const loopsFixture = [
  {
    loopID: 'loop-alpha',
    project: 'alpha',
    status: 'unresolved',
    attempt: 1,
    reason: 'awaiting execution',
    revision: 12,
  },
  {
    loopID: 'loop-beta',
    project: 'alpha',
    status: 'running',
    attempt: 3,
    reason: 'running worker',
    revision: 27,
  },
  {
    loopID: 'loop-gamma',
    project: 'beta',
    status: 'flatline',
    attempt: 2,
    reason: 'manual stop',
    revision: 39,
  },
];

/**
 * Inject a mock EventSource into the page so that the Svelte layout
 * streams never hit a real backend.  Call this BEFORE page.goto().
 *
 * The mock tracks all created sources on window.__mockEventSources.
 * Each source exposes an `emit(type, payload)` helper used by
 * `emitLoopUpdates` below.
 */
export async function mockEventSource(page) {
  await page.addInitScript(() => {
    window.__mockEventSources = [];

    class MockEventSource {
      constructor(url) {
        this.url = url;
        this.listeners = {};
        this.readyState = 1;
        window.__mockEventSources.push(this);
      }
      addEventListener(type, cb) {
        if (!this.listeners[type]) this.listeners[type] = [];
        this.listeners[type].push(cb);
      }
      set onmessage(cb) { this.addEventListener('message', cb); }
      set onopen(cb)    { setTimeout(cb, 0); }
      set onerror(cb)   { /* swallow */ }
      close() {
        this.readyState = 2;
        window.__mockEventSources = window.__mockEventSources.filter(s => s !== this);
      }
      emit(type, payload) {
        for (const cb of (this.listeners[type] || [])) cb(payload);
      }
    }

    window.EventSource = MockEventSource;
  });
}

/**
 * Wait for the Svelte layout to create its loops EventSource, then
 * push normalised loop objects through it.  The Svelte layout listens
 * for the `update` event type.
 */
export async function emitLoopUpdates(page, loops) {
  // Wait until at least one EventSource targeting /loops/stream exists
  await page.waitForFunction(
    () => (window.__mockEventSources || []).some(s => s.url.includes('/loops/stream')),
    null,
    { timeout: 5000 },
  );

  await page.evaluate((items) => {
    for (const source of (window.__mockEventSources || [])) {
      if (!source.url.includes('/loops/stream')) continue;
      for (const loop of items) {
        source.emit('update', {
          data: JSON.stringify({
            record: {
              loop_id: loop.loopID,
              project_id: loop.project,
              state: loop.status,
              attempt: loop.attempt,
              reason: loop.reason,
            },
            revision: loop.revision,
          }),
        });
      }
    }
  }, loops);
}

/**
 * Set up all API route mocks the Svelte frontend uses.
 *
 * Returns an object with observable arrays for verifying POST/PUT side
 * effects (override payloads, command payloads, etc.).
 */
export async function mockApiRoutes(page, options = {}) {
  const loops = options.loops || loopsFixture;
  const projectsState = options.projects || [
    {
      id: 'alpha',
      name: 'alpha',
      repo_url: 'https://github.com/callmeradical/smith.git',
      github_user: 'smith-bot',
    },
  ];

  const authState = { connected: false };
  const overridePayloads = [];
  const commandPayloads = [];

  // ── runtime config ────────────────────────────────────────────────
  await page.addInitScript(() => {
    window.__SMITH_CONFIG__ = { apiBaseUrl: '', operatorToken: 'test-token' };
    // auto-confirm dialogs
    window.__lastConfirmMessage = '';
    window.confirm = (msg) => { window.__lastConfirmMessage = String(msg || ''); return true; };
  });

  // ── HTTP routes ───────────────────────────────────────────────────

  // GET /v1/loops
  await page.route(/\/v1\/loops$/, async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(loops),
    });
  });

  // GET /v1/projects
  await page.route(/\/v1\/projects$/, async (route) => {
    if (route.request().method() === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(projectsState),
      });
    }
    if (route.request().method() === 'POST') {
      const payload = route.request().postDataJSON();
      const next = { id: payload.name?.toLowerCase() || payload.id, name: payload.name, repo_url: payload.repo_url };
      projectsState.push(next);
      return route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(next) });
    }
    return route.fallback();
  });

  // GET /v1/documents
  await page.route(/\/v1\/documents$/, async (route) => {
    return route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
  });

  // Auth status
  await page.route(/\/v1\/auth\/codex\/status$/, async (route) => {
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(authState) });
  });

  // Auth connect via API key
  await page.route(/\/v1\/auth\/codex\/connect\/api-key$/, async (route) => {
    authState.connected = true;
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{"ok":true}' });
  });

  // Control override (cancel / terminate)
  await page.route(/\/v1\/control\/override$/, async (route) => {
    const payload = route.request().postDataJSON();
    overridePayloads.push(payload);
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{"ok":true}' });
  });

  // Command execution
  await page.route(/\/v1\/loops\/[^/]+\/control\/command$/, async (route) => {
    const payload = route.request().postDataJSON();
    commandPayloads.push(payload);
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{"status":"completed"}' });
  });

  return { overridePayloads, commandPayloads, projectsState, authState };
}
