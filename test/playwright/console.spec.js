import { expect, test } from '@playwright/test';

const loopsFixture = [
  {
    record: {
      loop_id: 'loop-alpha',
      project_id: 'alpha',
      state: 'unresolved',
      attempt: 1,
      reason: 'awaiting execution',
    },
    revision: 12,
  },
  {
    record: {
      loop_id: 'loop-beta',
      project_id: 'alpha',
      state: 'running',
      attempt: 3,
      reason: 'running worker',
    },
    revision: 27,
  },
  {
    record: {
      loop_id: 'loop-gamma',
      project_id: 'beta',
      state: 'flatline',
      attempt: 2,
      reason: 'manual stop',
    },
    revision: 39,
  },
];

function jsonResponse(route, payload, status = 200) {
  return route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(payload),
  });
}

async function mockConsoleApi(page, options = {}) {
  const loopsState = loopsFixture.map((item) => ({
    ...item,
    record: { ...item.record },
  }));
  
  const projectsState = [
    {
      id: 'alpha',
      name: 'alpha',
      repo_url: 'https://github.com/callmeradical/smith.git',
      github_user: 'smith-bot',
    }
  ];

  const authState = {
    connected: false,
    account_id: '',
    auth_method: '',
    expires_at: '',
    last_refresh_at: '',
    api_key: '',
  };
  const overridePayloads = [];
  const createdLoopPayloads = [];
  const commandPayloads = [];
  const attachedSessions = new Set();
  const failCommands = new Set(
    Array.isArray(options.failCommands) && options.failCommands.length > 0
      ? options.failCommands
      : ['fail-command'],
  );
  const journalEntriesByLoopID = {};
  let journalSequence = Date.now();

  await page.addInitScript(() => {
    window.__SMITH_CONFIG__ = {
      apiBaseUrl: '',
      operatorToken: 'test-token',
    };
    window.__lastConfirmMessage = '';
    window.confirm = (message) => {
      window.__lastConfirmMessage = String(message || '');
      return true;
    };
    window.__mockEventSources = [];
    window.__emitMockJournalEntry = (loopID, entry) => {
      if (!entry || !loopID) return;
      const payload = { data: JSON.stringify(entry) };
      for (const source of window.__mockEventSources) {
        if (!source || source.readyState !== 1) continue;
        if (String(source.loopID || "") !== String(loopID)) continue;
        source.emit("message", payload);
      }
    };

    class MockEventSource {
      constructor(url) {
        this.url = url;
        this.listeners = {};
        this.readyState = 1;
        const match = String(url || "").match(
          /\/v1\/loops\/([^/]+)\/journal\/stream/,
        );
        this.loopID = match ? decodeURIComponent(match[1]) : "";
        window.__mockEventSources.push(this);
        setTimeout(
          () =>
            window.__emitMockJournalEntry(this.loopID, {
              sequence: Date.now(),
              timestamp: new Date().toISOString(),
              level: "info",
              phase: "replica",
              actor_id: "replica-test",
              message: "worker started",
            }),
          10,
        );
      }
      addEventListener(type, callback) {
        if (!this.listeners[type]) this.listeners[type] = [];
        this.listeners[type].push(callback);
      }
      set onmessage(cb) { this.addEventListener("message", cb); }
      set onopen(cb) { setTimeout(cb, 0); }
      set onerror(cb) {}
      close() {
        this.readyState = 2;
        window.__mockEventSources = window.__mockEventSources.filter(s => s !== this);
      }
      emit(type, payload) {
        const listeners = this.listeners[type] || [];
        for (const listener of listeners) listener(payload);
      }
    }
    window.EventSource = MockEventSource;

    window.__mockWebSockets = [];
    class MockWebSocket {
      constructor(url) {
        this.url = url;
        this.readyState = 0;
        window.__mockWebSockets.push(this);
        setTimeout(() => {
          this.readyState = 1;
          if (this.onopen) this.onopen();
        }, 10);
      }
      send(data) {
        if (!this.__sentMessages) this.__sentMessages = [];
        this.__sentMessages.push(data);
        if (this.onsend) this.onsend(data);
      }
      close() {
        this.readyState = 3;
        if (this.onclose) this.onclose();
      }
    }
    window.WebSocket = MockWebSocket;
  });

  function loopIDFromURL(url, suffix) {
    const path = new URL(url).pathname;
    if (!path.includes('/v1/loops/')) return '';
    const parts = path.split('/v1/loops/')[1].split(suffix);
    return decodeURIComponent(parts[0]).trim();
  }

  function loopRecord(loopID) {
    return loopsState.find((item) => item?.record?.loop_id === loopID) || null;
  }

  function loopActive(loopID) {
    const current = loopRecord(loopID);
    if (!current) return false;
    const state = String(current.record?.state || '').toLowerCase();
    return state === 'unresolved' || state === 'running' || state === 'flatline';
  }

  function loopRuntimePayload(loopID) {
    const current = loopRecord(loopID);
    if (!current) return null;
    const attachable = loopActive(loopID);
    return {
      loop_id: loopID,
      namespace: 'smith-system',
      pod_name: attachable ? `smith-replica-${loopID}-abc` : '',
      container_name: 'replica',
      pod_phase: attachable ? 'Running' : 'Succeeded',
      attachable,
      reason: attachable ? '' : 'loop not active',
    };
  }

  async function emitJournal(loopID, actorID, message, level = 'info') {
    journalSequence = Math.max(journalSequence + 1, Date.now());
    const entry = {
      sequence: journalSequence,
      timestamp: new Date().toISOString(),
      level,
      phase: 'operator',
      actor_id: actorID,
      message: String(message || ''),
    };
    if (!journalEntriesByLoopID[loopID]) journalEntriesByLoopID[loopID] = [];
    journalEntriesByLoopID[loopID].push(entry);
    await page.evaluate(
      ({ targetLoopID, streamEntry }) => {
        if (typeof window.__emitMockJournalEntry === 'function') {
          window.__emitMockJournalEntry(targetLoopID, streamEntry);
        }
      },
      { targetLoopID: loopID, streamEntry: entry },
    );
  }

  await page.route(/\/v1\/loops$/, async (route) => {
    if (route.request().method() === 'GET') return jsonResponse(route, loopsState.map(s => ({
      loop_id: s.record.loop_id,
      project: s.record.project_id,
      status: s.record.state,
      attempt: s.record.attempt,
      revision: s.revision,
      reason: s.record.reason
    })));
    if (route.request().method() === 'POST') {
      const payload = route.request().postDataJSON();
      createdLoopPayloads.push(payload);
      const nextLoopID = payload.loop_id || `loop-${Date.now()}`;
      loopsState.push({
        record: {
          loop_id: nextLoopID,
          project_id: 'alpha',
          state: 'unresolved',
          attempt: 0,
          reason: 'created-via-api',
        },
        revision: Date.now(),
      });
      return jsonResponse(route, { loop_id: nextLoopID, status: 'unresolved', created: true }, 201);
    }
  });

  await page.route(/\/v1\/loops\/[^/]+$/, async (route) => {
    const id = decodeURIComponent(route.request().url().split('/v1/loops/')[1] || '').trim();
    if (route.request().method() === 'DELETE') {
      const idx = loopsState.findIndex(s => s.record.loop_id === id);
      if (idx >= 0) loopsState.splice(idx, 1);
      return jsonResponse(route, { status: 'deleted' });
    }
    const s = loopsState.find(item => item.record.loop_id === id);
    return s ? jsonResponse(route, s) : jsonResponse(route, { error: 'not found' }, 404);
  });

  await page.route(/\/v1\/loops\/[^/]+\/runtime$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/runtime');
    const runtime = loopRuntimePayload(loopID);
    return runtime ? jsonResponse(route, runtime) : jsonResponse(route, { error: 'not found' }, 404);
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/attach$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/attach');
    attachedSessions.add(`${loopID}:operator`);
    return jsonResponse(route, { loop_id: loopID, status: 'attached' });
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/detach$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/detach');
    attachedSessions.delete(`${loopID}:operator`);
    return jsonResponse(route, { loop_id: loopID, status: 'detached' });
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/command$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/command');
    const payload = route.request().postDataJSON();
    commandPayloads.push(payload);
    if (failCommands.has(payload.command)) return jsonResponse(route, { error: 'failed' }, 500);
    await emitJournal(loopID, 'operator', 'ok');
    return jsonResponse(route, { status: 'completed', result: 'success' });
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/cancel$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/cancel');
    overridePayloads.push({ loopID, target_state: 'cancelled' });
    return jsonResponse(route, { ok: true });
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/terminate$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/terminate');
    overridePayloads.push({ loopID, target_state: 'cancelled', terminate: true });
    return jsonResponse(route, { ok: true });
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/override$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/override');
    const payload = route.request().postDataJSON();
    overridePayloads.push({ ...payload, loopID });
    return jsonResponse(route, { ok: true });
  });

  await page.route(/\/v1\/auth\/codex\/status$/, async (route) => jsonResponse(route, authState));
  await page.route(/\/v1\/auth\/codex\/connect\/api-key$/, async (route) => {
    authState.connected = true;
    return jsonResponse(route, { ok: true });
  });
  await page.route(/\/v1\/auth\/codex\/disconnect$/, async (route) => {
    authState.connected = false;
    return jsonResponse(route, { ok: true });
  });
  await page.route(/\/v1\/projects$/, async (route) => {
    if (route.request().method() === 'GET') return jsonResponse(route, projectsState);
    if (route.request().method() === 'POST') {
      const payload = route.request().postDataJSON();
      const next = { id: payload.name.toLowerCase(), name: payload.name, repo_url: payload.repo_url };
      projectsState.push(next);
      return jsonResponse(route, next, 201);
    }
  });

  return { overridePayloads, createdLoopPayloads, commandPayloads, projectsState };
}

async function setupPage(page) {
  await page.goto('/');
  await page.evaluate(async () => {
    localStorage.clear();
    if (window.sidebarEl) window.sidebarEl.classList.remove('open');
    window.scrollTo(0, 0);
    if (typeof window.refreshLoops === "function") await window.refreshLoops();
    if (typeof window.renderProviderList === "function") window.renderProviderList();
    if (typeof window.refreshProjects === "function") await window.refreshProjects();
  });
}

test('renders loop tiles and summary stats', async ({ page }) => {
  await mockConsoleApi(page);
  await setupPage(page);
  await expect(page.locator('#stat-total')).toHaveText('3');
  await expect(page.locator('#grid .pod-tile')).toHaveCount(3);
});

test('supports pod detail terminal control states', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await setupPage(page);
  await page.locator('#grid .pod-tile', { hasText: 'loop-alpha' }).click();
  await expect(page.locator('#pod-view-title')).toHaveText('Pod: loop-alpha');
  await expect(page.locator('#pod-view-terminal-state')).toHaveText('idle');
  
  await page.locator('#pod-view-attach').click();
  await expect(page.locator('#pod-view-terminal-state')).toHaveText('attached');
  
  await page.locator('#pod-view-command').fill('echo ok');
  await page.keyboard.press('Enter');
  await expect(page.locator('#pod-view-command')).toHaveValue('');
  await expect(page.locator('#terminal')).toContainText('ok');
});

test('supports cancel and terminate controls from pod detail', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await setupPage(page);
  await page.locator('#grid .pod-tile', { hasText: 'loop-beta' }).click();
  await page.locator('#pod-view-cancel').click();
  await expect.poll(() => api.overridePayloads.length).toBe(1);
  expect(api.overridePayloads[0].target_state).toBe('cancelled');
});

test('deletes a completed loop from pod detail view', async ({ page }) => {
  await mockConsoleApi(page);
  await setupPage(page);
  await page.locator('#grid .pod-tile', { hasText: 'loop-gamma' }).click();
  await page.locator('#pod-view-delete').click();
  await expect(page.locator('#grid .pod-tile', { hasText: 'loop-gamma' })).toHaveCount(0);
});

test('filters tiles by state and search input', async ({ page }) => {
  await mockConsoleApi(page);
  await setupPage(page);
  await page.locator('#search').fill('alpha');
  await expect(page.locator('#grid .pod-tile')).toHaveCount(1);
  await page.locator('#search').fill('');
  await page.locator('#state-filter').selectOption('flatline');
  await expect(page.locator('#grid .pod-tile')).toHaveCount(1);
  await expect(page.locator('#grid .pod-tile')).toContainText('loop-gamma');
});

test('validates and submits override actions', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await setupPage(page);
  await page.locator('#grid .pod-tile', { hasText: 'loop-alpha' }).click();
  await page.locator('.page.active .topbar .sidebar-toggle').click();
  await page.locator('[data-page-link="controls"]').click();
  
  await page.locator('#override-apply').click();
  await expect(page.locator('#journal-status')).toHaveText('override reason required');
  
  await page.locator('#override-reason').fill('test');
  await page.locator('#override-apply').click();
  await expect(page.locator('#journal-status')).toHaveText('type APPLY to confirm');
  
  await page.locator('#override-confirm').fill('APPLY');
  await page.locator('#override-apply').click();
  await expect(page.locator('#journal-status')).toHaveText('override applied');
  await expect.poll(() => api.overridePayloads.length).toBe(1);
});

test('supports provider catalog API key auth and delete', async ({ page }) => {
  await mockConsoleApi(page);
  await setupPage(page);
  await page.locator('.page.active .topbar .sidebar-toggle').click();
  await page.locator('[data-page-link="providers"]').click();

  await expect(page.locator('#provider-config-panel')).toBeHidden();
  await expect(page.locator('#provider-list button[data-provider-config="codex"]')).toBeVisible();
  await page.locator('#provider-list button[data-provider-config="codex"]').click();

  await expect(page.locator('#provider-config-panel')).toBeVisible();
  await page.locator('#auth-api-key').fill('sk-test-key');
  await page.locator('#auth-save-api-key').click();

  await expect(page.locator('#provider-config-panel')).toBeHidden();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(1);

  await page.locator('#provider-list button[data-provider-config="codex"]').click();
  await page.locator('#auth-disconnect').click();
  await expect(page.locator('#provider-config-panel')).toBeHidden();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(0);
});

test('manages projects through project drawer', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await setupPage(page);
  await page.locator('.page.active .topbar .sidebar-toggle').click();
  await page.locator('[data-page-link="projects"]').click();

  await expect(page.locator('#project-list')).toContainText('alpha');

  await page.locator('#project-add').click();
  await expect(page.locator('#project-config-panel')).toBeVisible();

  await page.locator('#project-name').fill('new-project');
  await page.locator('#project-repo-url').fill('https://github.com/org/repo.git');
  await page.locator('#project-save').click();

  await expect(page.locator('#project-config-panel')).toBeHidden();
  await expect(page.locator('#project-list')).toContainText('new-project');
  expect(api.projectsState.find(p => p.name === 'new-project')).toBeDefined();
});
