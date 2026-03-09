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
      state: 'overwriting',
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
  const authState = {
    connected: false,
    account_id: '',
    auth_method: '',
    expires_at: '',
    last_refresh_at: '',
    api_key: '',
  };
  const projectCredentialState = {};
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
    window.__lastConfirmMessage = '';
    window.confirm = (message) => {
      window.__lastConfirmMessage = String(message || '');
      return true;
    };
    window.__mockEventSources = [];
    window.__emitMockJournalEntry = (loopID, entry) => {
      if (!entry || !loopID) return;
      const payload = { data: JSON.stringify({ entry }) };
      for (const source of window.__mockEventSources) {
        if (!source || source.readyState !== 1) continue;
        if (String(source.loopID || '') !== String(loopID)) continue;
        source.emit('entry', payload);
      }
    };

    class MockEventSource {
      constructor(url) {
        this.url = url;
        this.listeners = {};
        this.readyState = 1;
        const match = String(url || '').match(/\/v1\/loops\/([^/]+)\/journal\/stream/);
        this.loopID = match ? decodeURIComponent(match[1]) : '';
        window.__mockEventSources.push(this);
        setTimeout(() => this.emit('ready', { data: '{}' }), 0);
        setTimeout(
          () =>
            window.__emitMockJournalEntry(this.loopID, {
              sequence: Date.now(),
              timestamp: new Date().toISOString(),
              level: 'info',
              phase: 'replica',
              actor_id: 'replica-test',
              message: 'worker started',
            }),
          10,
        );
      }

      addEventListener(type, callback) {
        if (!this.listeners[type]) {
          this.listeners[type] = [];
        }
        this.listeners[type].push(callback);
      }

      close() {
        this.readyState = 2;
        window.__mockEventSources = window.__mockEventSources.filter((source) => source !== this);
      }

      emit(type, payload) {
        const listeners = this.listeners[type] || [];
        for (const listener of listeners) {
          listener(payload);
        }
      }
    }

    window.EventSource = MockEventSource;
  });

  function loopIDFromURL(url, suffix) {
    const path = new URL(url).pathname;
    if (!path.startsWith('/v1/loops/') || !path.endsWith(suffix)) {
      return '';
    }
    const encoded = path.slice('/v1/loops/'.length, path.length - suffix.length);
    return decodeURIComponent(encoded).trim();
  }

  function loopRecord(loopID) {
    return loopsState.find((item) => item?.record?.loop_id === loopID) || null;
  }

  function loopActive(loopID) {
    const current = loopRecord(loopID);
    if (!current) return false;
    const state = String(current.record?.state || '').toLowerCase();
    return state === 'unresolved' || state === 'overwriting';
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

  function sessionKey(loopID, actor) {
    return `${loopID}:${actor || 'operator'}`;
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
    if (!journalEntriesByLoopID[loopID]) {
      journalEntriesByLoopID[loopID] = [];
    }
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
    if (route.request().method() === 'GET') {
      await jsonResponse(route, loopsState);
      return;
    }
    if (route.request().method() === 'POST') {
      const payload = route.request().postDataJSON();
      createdLoopPayloads.push(payload);
      const nextLoopID = payload.loop_id || `loop-${Date.now()}`;
      loopsState.push({
        record: {
          loop_id: nextLoopID,
          project_id: nextLoopID.includes('/') ? nextLoopID.split('/')[0] : 'default',
          state: 'unresolved',
          attempt: 0,
          reason: 'created-via-api',
        },
        revision: Date.now(),
      });
      await jsonResponse(route, { loop_id: nextLoopID, status: 'unresolved', created: true }, 201);
      return;
    }
    await jsonResponse(route, { error: 'method not allowed' }, 405);
  });

  await page.route(/\/v1\/loops\/[^/]+$/, async (route) => {
    const method = route.request().method();
    const id = decodeURIComponent(route.request().url().split('/v1/loops/')[1] || '').trim();
    if (method === 'DELETE') {
      const index = loopsState.findIndex((item) => item?.record?.loop_id === id);
      if (index < 0) {
        await jsonResponse(route, { error: 'loop not found' }, 404);
        return;
      }
      loopsState.splice(index, 1);
      await jsonResponse(route, { loop_id: id, status: 'deleted' });
      return;
    }
    if (method === 'GET') {
      const current = loopsState.find((item) => item?.record?.loop_id === id);
      if (!current) {
        await jsonResponse(route, { error: 'loop not found' }, 404);
        return;
      }
      await jsonResponse(route, { state: current });
      return;
    }
    await jsonResponse(route, { error: 'method not allowed' }, 405);
  });

  await page.route(/\/v1\/loops\/[^/]+\/runtime$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/runtime');
    const runtime = loopRuntimePayload(loopID);
    if (!runtime) {
      await jsonResponse(route, { error: 'loop not found' }, 404);
      return;
    }
    await jsonResponse(route, runtime);
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/attach$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/attach');
    const runtime = loopRuntimePayload(loopID);
    if (!runtime) {
      await jsonResponse(route, { error: 'loop not found' }, 404);
      return;
    }
    if (!runtime.attachable) {
      await jsonResponse(route, { error: runtime.reason || 'runtime target not attachable' }, 409);
      return;
    }
    const payload = route.request().postDataJSON();
    const actor = String(payload?.actor || 'operator').trim() || 'operator';
    attachedSessions.add(sessionKey(loopID, actor));
    await jsonResponse(route, {
      loop_id: loopID,
      status: 'attached',
      actor,
      attach_count: 1,
      active_attach_count: 1,
      runtime_target_ref: `${runtime.namespace}/${runtime.pod_name}:${runtime.container_name}`,
    });
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/detach$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/detach');
    const runtime = loopRuntimePayload(loopID);
    if (!runtime) {
      await jsonResponse(route, { error: 'loop not found' }, 404);
      return;
    }
    const payload = route.request().postDataJSON();
    const actor = String(payload?.actor || 'operator').trim() || 'operator';
    const key = sessionKey(loopID, actor);
    if (!attachedSessions.has(key)) {
      await jsonResponse(route, { error: 'actor is not attached' }, 409);
      return;
    }
    attachedSessions.delete(key);
    await jsonResponse(route, {
      loop_id: loopID,
      status: 'detached',
      actor,
      attach_count: 1,
      active_attach_count: 0,
      runtime_target_ref: `${runtime.namespace}/${runtime.pod_name}:${runtime.container_name}`,
    });
  });

  await page.route(/\/v1\/loops\/[^/]+\/control\/command$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/control/command');
    const runtime = loopRuntimePayload(loopID);
    if (!runtime) {
      await jsonResponse(route, { error: 'loop not found' }, 404);
      return;
    }
    if (!loopActive(loopID)) {
      await jsonResponse(route, { error: 'loop is not active' }, 409);
      return;
    }
    const payload = route.request().postDataJSON();
    commandPayloads.push(payload);
    const actor = String(payload?.actor || 'operator').trim() || 'operator';
    const key = sessionKey(loopID, actor);
    if (!attachedSessions.has(key)) {
      await jsonResponse(route, { error: 'actor must attach before issuing commands' }, 409);
      return;
    }
    const command = String(payload?.command || '').trim();
    if (!command) {
      await jsonResponse(route, { error: 'command is required' }, 400);
      return;
    }
    if (failCommands.has(command)) {
      await jsonResponse(route, { error: 'command failed in runtime' }, 500);
      return;
    }
    const stdout =
      command === 'pwd'
        ? `/workspace/${loopID}\n`
        : command === 'echo ok'
          ? 'ok\n'
          : `executed: ${command}\n`;
    for (const line of stdout.split('\n').map((value) => value.trim()).filter(Boolean)) {
      await emitJournal(loopID, actor, line, 'info');
    }
    await jsonResponse(route, {
      loop_id: loopID,
      status: 'completed',
      actor,
      command,
      delivered: true,
      result: 'success',
      exit_code: 0,
      stdout,
      stderr: '',
      runtime_target_ref: `${runtime.namespace}/${runtime.pod_name}:${runtime.container_name}`,
    });
  });

  await page.route(/^https:\/\/api\.github\.com\/repos\/[^/]+\/[^/]+\/issues.*$/, async (route) => {
    await jsonResponse(route, [
      {
        number: 132,
        title: 'Simplified console UX with sidebar and pod tiles',
        body: 'Implement the next iteration of pods/projects/providers layout.',
        html_url: 'https://github.com/callmeradical/smith/issues/132',
        labels: [{ name: 'frontend' }, { name: 'p1' }],
      },
      {
        number: 133,
        title: 'Add missing provider auth cleanup',
        body: 'Cleanup auth state handling in console.',
        html_url: 'https://github.com/callmeradical/smith/issues/133',
        labels: [{ name: 'bug' }],
      },
    ]);
  });

  await page.route(/\/v1\/auth\/codex\/status$/, async (route) => {
    if (!authState.connected) {
      await jsonResponse(route, { connected: false });
      return;
    }
    await jsonResponse(route, { ...authState });
  });

  await page.route(/\/v1\/auth\/codex\/credential(\?.*)?$/, async (route) => {
    if (!authState.connected) {
      await jsonResponse(route, { connected: false });
      return;
    }
    const url = new URL(route.request().url());
    const reveal = String(url.searchParams.get('reveal') || '').toLowerCase() === 'true';
    const key = String(authState.api_key || '');
    const masked = key.length <= 8 ? '*'.repeat(key.length) : `${key.slice(0, 4)}${'*'.repeat(key.length - 8)}${key.slice(-4)}`;
    const payload = {
      connected: true,
      provider: 'codex',
      auth_method: authState.auth_method,
      account_id: authState.account_id,
      api_key_masked: masked,
    };
    if (reveal) {
      payload.api_key = key;
    }
    await jsonResponse(route, payload);
  });

  await page.route(/\/v1\/auth\/codex\/connect\/api-key$/, async (route) => {
    const payload = route.request().postDataJSON();
    authState.connected = true;
    authState.account_id = payload.account_id || 'api-key';
    authState.auth_method = 'api_key';
    authState.expires_at = '2030-01-01T00:00:00Z';
    authState.last_refresh_at = '2030-01-01T00:00:00Z';
    authState.api_key = payload.api_key || '';
    await jsonResponse(route, { ok: true });
  });

  await page.route(/\/v1\/auth\/codex\/disconnect$/, async (route) => {
    authState.connected = false;
    authState.account_id = '';
    authState.auth_method = '';
    authState.expires_at = '';
    authState.last_refresh_at = '';
    authState.api_key = '';
    await jsonResponse(route, { ok: true });
  });

  await page.route(/\/v1\/projects\/credentials\/github(\?.*)?$/, async (route) => {
    const method = route.request().method();
    const url = new URL(route.request().url());
    const projectID = String(url.searchParams.get('project_id') || '').trim();
    if (method === 'GET') {
      if (!projectID || !projectCredentialState[projectID]) {
        await jsonResponse(route, { project_id: projectID, credential_set: false });
        return;
      }
      const reveal = String(url.searchParams.get('reveal') || '').toLowerCase() === 'true';
      const entry = projectCredentialState[projectID];
      const key = String(entry.credential || '');
      const masked = key.length <= 8 ? '*'.repeat(key.length) : `${key.slice(0, 4)}${'*'.repeat(key.length - 8)}${key.slice(-4)}`;
      const payload = {
        project_id: projectID,
        credential_set: true,
        github_user: entry.github_user || '',
        credential_masked: masked,
      };
      if (reveal) {
        payload.credential = key;
      }
      await jsonResponse(route, payload);
      return;
    }
    if (method === 'POST') {
      const payload = route.request().postDataJSON();
      const id = String(payload.project_id || '').trim();
      projectCredentialState[id] = {
        credential: String(payload.credential || ''),
        github_user: String(payload.github_user || ''),
      };
      const key = String(payload.credential || '');
      const masked = key.length <= 8 ? '*'.repeat(key.length) : `${key.slice(0, 4)}${'*'.repeat(key.length - 8)}${key.slice(-4)}`;
      await jsonResponse(route, {
        project_id: id,
        credential_set: true,
        github_user: String(payload.github_user || ''),
        credential_masked: masked,
      });
      return;
    }
    if (method === 'DELETE') {
      let body = {};
      try {
        body = route.request().postDataJSON() || {};
      } catch (_) {}
      const id = projectID || String(body.project_id || '').trim();
      delete projectCredentialState[id];
      await jsonResponse(route, { project_id: id, credential_set: false });
      return;
    }
    await jsonResponse(route, { error: 'method not allowed' }, 405);
  });

  await page.route(/\/v1\/control\/override$/, async (route) => {
    const payload = route.request().postDataJSON();
    overridePayloads.push(payload);
    await jsonResponse(route, { revision: 101 });
  });

  await page.route(/\/v1\/loops\/[^/]+\/journal(\?.*)?$/, async (route) => {
    const loopID = loopIDFromURL(route.request().url(), '/journal');
    await jsonResponse(route, journalEntriesByLoopID[loopID] || []);
  });

  return { overridePayloads, createdLoopPayloads, commandPayloads };
}

test('renders loop tiles and summary stats', async ({ page }) => {
  await mockConsoleApi(page);
  await page.goto('/');

  await expect(page.locator('#stat-total')).toHaveText('3');
  await expect(page.locator('#stat-active')).toHaveText('2');
  await expect(page.locator('#stat-flatline')).toHaveText('1');
  await expect(page.locator('#grid .pod-tile')).toHaveCount(3);
  await expect(page.locator('#grid .pod-tile .tile-loop').first()).toContainText('alpha');
  await expect(page.locator('#page-pod-view')).toBeHidden();

  await page.locator('#grid .pod-tile', { hasText: 'loop-alpha' }).click();
  await expect(page.locator('#page-pod-view')).toBeVisible();
  await expect(page).toHaveURL(/#pod-view\/loop-alpha$/);
  await expect(page.locator('#pod-view-title')).toContainText('loop-alpha');
  await expect(page.locator('#journal-title')).toContainText('loop-alpha');
  await expect(page.locator('#terminal')).toContainText('worker started');
});

test('supports pod detail terminal control states', async ({ page }) => {
  const api = await mockConsoleApi(page, { failCommands: ['fail-command'] });
  await page.goto('/');

  await page.locator('#grid .pod-tile', { hasText: 'loop-alpha' }).click();
  await expect(page.locator('#page-pod-view')).toBeVisible();
  await expect(page.locator('#pod-view-runtime-target')).toContainText('smith-system');
  await expect(page.locator('#pod-view-terminal-state')).toHaveText('idle');
  await expect(page.locator('#pod-view-control-message')).toContainText('Attach to enable');
  await expect(page.locator('#pod-view-attach')).toBeEnabled();
  await expect(page.locator('#pod-view-cancel')).toBeEnabled();
  await expect(page.locator('#pod-view-terminate')).toBeDisabled();
  await expect(page.locator('#pod-view-command')).toBeDisabled();
  await expect(page.locator('#pod-view-run')).toBeDisabled();

  await page.locator('#pod-view-attach').click();
  await expect(page.locator('#pod-view-terminal-state')).toHaveText('attached');
  await expect(page.locator('#pod-view-attach')).toHaveText('detach');
  await expect(page.locator('#pod-view-command')).toBeEnabled();
  await expect(page.locator('#pod-view-control-message')).toContainText('Press Enter or Run');

  await page.locator('#pod-view-command').fill('echo ok');
  await expect(page.locator('#pod-view-run')).toBeEnabled();
  await page.locator('#pod-view-command').press('Enter');
  await expect(page.locator('#pod-view-terminal-state')).toHaveText('attached');
  await expect(page.locator('#pod-view-command')).toHaveValue('');
  await expect(page.locator('#terminal')).toContainText('ok');
  await expect
    .poll(() => api.commandPayloads.length)
    .toBe(1);

  await page.locator('#pod-view-command').fill('fail-command');
  await page.locator('#pod-view-run').click();
  await expect(page.locator('#pod-view-terminal-state')).toHaveText('error');
  await expect(page.locator('#pod-view-command')).toHaveValue('fail-command');
  await expect(page.locator('#pod-view-control-message')).toContainText('command failed in runtime');

  await page.locator('#pod-view-attach').click();
  await expect(page.locator('#pod-view-terminal-state')).toHaveText('idle');
  await expect(page.locator('#pod-view-attach')).toHaveText('attach');
  await expect(page.locator('#pod-view-command')).toBeDisabled();
  await expect(page.locator('#pod-view-run')).toBeDisabled();

  await page.locator('#pod-view-back').click();
  await page.locator('#grid .pod-tile', { hasText: 'loop-gamma' }).click();
  await expect(page.locator('#pod-view-control-message')).toContainText('loop not active');
  await expect(page.locator('#pod-view-terminal-state')).toHaveText('idle');
  await expect(page.locator('#pod-view-attach')).toBeDisabled();
  await expect(page.locator('#pod-view-cancel')).toBeDisabled();
  await expect(page.locator('#pod-view-terminate')).toBeDisabled();
  await expect(page.locator('#pod-view-command')).toBeDisabled();
  await expect(page.locator('#pod-view-run')).toBeDisabled();
});

test('supports cancel and terminate controls from pod detail', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await page.route(/\/v1\/loops\/loop-beta\/runtime$/, async (route) => {
    await jsonResponse(route, {
      loop_id: 'loop-beta',
      namespace: 'smith-system',
      pod_name: 'smith-replica-loop-beta-abc',
      container_name: 'replica',
      pod_phase: 'Pending',
      attachable: false,
      reason: 'runtime pod not running',
    });
  });
  await page.goto('/');

  await page.locator('#grid .pod-tile', { hasText: 'loop-alpha' }).click();
  await expect(page.locator('#pod-view-cancel')).toBeEnabled();
  await expect(page.locator('#pod-view-terminate')).toBeDisabled();
  await page.locator('#pod-view-cancel').click();
  await expect
    .poll(() => api.overridePayloads.length)
    .toBe(1);
  expect(api.overridePayloads[0]).toMatchObject({
    loop_id: 'loop-alpha',
    target_state: 'cancelled',
  });

  await page.locator('#pod-view-back').click();
  await page.locator('#grid .pod-tile', { hasText: 'loop-beta' }).click();
  await expect(page.locator('#pod-view-terminate')).toBeEnabled();
  await page.locator('#pod-view-terminate').click();
  await expect
    .poll(() => api.overridePayloads.length)
    .toBe(2);
  expect(api.overridePayloads[1]).toMatchObject({
    loop_id: 'loop-beta',
    target_state: 'flatline',
  });
});

test('retries runtime lookup on repeated tile attach attempts', async ({ page }) => {
  await mockConsoleApi(page);
  let alphaRuntimeCalls = 0;
  await page.route(/\/v1\/loops\/loop-alpha\/runtime$/, async (route) => {
    alphaRuntimeCalls += 1;
    if (alphaRuntimeCalls === 1) {
      await jsonResponse(route, {
        loop_id: 'loop-alpha',
        namespace: 'smith-system',
        container_name: 'replica',
        attachable: false,
        reason: 'runtime pod not found',
      });
      return;
    }
    await jsonResponse(route, {
      loop_id: 'loop-alpha',
      namespace: 'smith-system',
      pod_name: 'smith-replica-loop-alpha-xyz',
      container_name: 'replica',
      pod_phase: 'Running',
      attachable: true,
      reason: '',
    });
  });

  await page.goto('/');
  const alphaTile = page.locator('#grid .pod-tile', { hasText: 'loop-alpha' });

  await alphaTile.locator('[data-tile-attach]').click();
  await expect.poll(() => alphaRuntimeCalls).toBe(1);

  await alphaTile.locator('[data-tile-attach]').click();
  await expect(page.locator('#toast-region .toast').last()).toContainText('Attached to loop-alpha');
  await expect.poll(() => alphaRuntimeCalls).toBeGreaterThanOrEqual(2);
});

test('deletes a completed loop from pod detail view', async ({ page }) => {
  await mockConsoleApi(page);
  await page.goto('/');

  await page.locator('#grid .pod-tile', { hasText: 'loop-gamma' }).click();
  await expect(page.locator('#page-pod-view')).toBeVisible();
  await expect(page.locator('#pod-view-delete')).toBeEnabled();
  await page.locator('#pod-view-delete').click();
  await expect
    .poll(() => page.evaluate(() => window.__lastConfirmMessage))
    .toContain('Delete loop loop-gamma?');
  await expect(page.locator('#toast-region .toast').last()).toContainText('Loop deleted: loop-gamma');
  await expect(page).toHaveURL(/#pods$/);
  await expect(page.locator('#grid .pod-tile')).toHaveCount(2);
  await expect(page.locator('#grid .pod-tile', { hasText: 'loop-gamma' })).toHaveCount(0);
});

test('filters tiles by state and search input', async ({ page }) => {
  await mockConsoleApi(page);
  await page.goto('/');

  await page.locator('#state-filter').selectOption('flatline');
  await expect(page.locator('#grid .pod-tile')).toHaveCount(1);
  await expect(page.locator('#grid .pod-tile .loop-id')).toHaveText('loop-gamma');

  await page.locator('#search').fill('beta');
  await expect(page.locator('#grid .empty')).toContainText('No pods');

  await page.locator('#state-filter').selectOption('all');
  await expect(page.locator('#grid .pod-tile')).toHaveCount(1);
  await expect(page.locator('#grid .pod-tile .loop-id')).toHaveText('loop-beta');
});

test('supports provider catalog API key auth and delete', async ({ page }) => {
  await mockConsoleApi(page);
  await page.goto('/');
  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-providers').click();

  await expect(page.locator('#provider-config-panel')).toBeHidden();
  await expect(page.locator('#provider-list button[data-provider-config="codex"]')).toBeVisible();
  await page.locator('#provider-list button[data-provider-config="codex"]').click();
  await expect(page.locator('#provider-config-panel')).toBeVisible();
  await expect(page.locator('#provider-config-title')).toHaveText('OpenAI Codex CLI Configuration');
  await page.locator('#auth-api-key').fill('invalid');
  await page.locator('#auth-save-api-key').click();
  await expect(page.locator('#toast-region .toast').last()).toContainText('valid OpenAI API key');
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(0);
  await page.locator('#auth-api-key').fill('sk-test-123');
  await page.locator('#auth-account-id').fill('acct-api');
  await page.locator('#auth-save-api-key').click();
  await expect(page.locator('#toast-region .toast').last()).toContainText('API key saved');
  await expect(page.locator('#provider-config-panel')).toBeHidden();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(1);

  await page.locator('#provider-list button[data-provider-config="codex"]').click();
  await expect(page.locator('#provider-config-panel')).toBeVisible();
  await expect(page.locator('#auth-api-key')).toHaveValue('sk-t***-123');
  await expect(page.locator('#auth-account-id')).toHaveValue('acct-api');

  await page.locator('#auth-reveal-api-key').click();
  await expect(page.locator('#auth-api-key')).toHaveValue('sk-test-123');
  await expect(page.locator('#auth-reveal-api-key')).toHaveAttribute('aria-label', 'Hide API key');

  await page.locator('#auth-reveal-api-key').click();
  await expect(page.locator('#auth-api-key')).toHaveValue('sk-t***-123');
  await expect(page.locator('#auth-reveal-api-key')).toHaveAttribute('aria-label', 'Reveal API key');

  await page.locator('#auth-api-key').fill('sk-unsaved-999');
  await page.locator('#auth-reveal-api-key').click();
  await expect(page.locator('#auth-api-key')).toHaveValue('sk-unsaved-999');
  await expect(page.locator('#auth-reveal-api-key')).toHaveAttribute('aria-label', 'Hide API key');
  await page.locator('#auth-reveal-api-key').click();
  await expect(page.locator('#auth-api-key')).toHaveValue('sk-unsaved-999');
  await expect(page.locator('#auth-reveal-api-key')).toHaveAttribute('aria-label', 'Reveal API key');

  await page.locator('#auth-disconnect').click();
  await expect
    .poll(() => page.evaluate(() => window.__lastConfirmMessage))
    .toBe('Are you sure you wish to delete these credentials?');
  await expect(page.locator('#toast-region .toast').last()).toContainText('Credential deleted');
  await expect(page.locator('#provider-config-panel')).toBeHidden();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(0);
});

test('validates and submits override actions', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await page.goto('/');

  await page.locator('#grid .pod-tile', { hasText: 'loop-beta' }).click();
  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-controls').click();

  await page.locator('#override-apply').click();
  await expect(page.locator('#journal-status')).toHaveText('override reason required');

  await page.locator('#override-reason').fill('manual recovery');
  await page.locator('#override-apply').click();
  await expect(page.locator('#journal-status')).toHaveText('type APPLY to confirm');

  await page.locator('#override-confirm').fill('APPLY');
  await page.locator('#override-state').selectOption('synced');
  await page.locator('#override-actor').fill('console-operator');
  await page.locator('#override-apply').click();

  await expect(page.locator('#journal-status')).toHaveText('override applied');
  await expect
    .poll(() => api.overridePayloads.length)
    .toBe(1);

  expect(api.overridePayloads[0]).toMatchObject({
    loop_id: 'loop-beta',
    target_state: 'synced',
    reason: 'manual recovery',
    actor: 'console-operator',
  });
});

test('adds and renders project configuration entries', async ({ page }) => {
  await mockConsoleApi(page);
  await page.goto('/');
  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-projects').click();

  await expect(page.locator('#project-config-panel')).toBeHidden();
  await page.locator('#project-add').click();
  await expect(page.locator('#project-config-panel')).toBeVisible();

  await page.locator('#project-save').click();
  await expect(page.locator('#project-form-status')).toHaveText('project name is required');

  await page.locator('#project-name').fill('beta');
  await page.locator('#project-repo-url').fill('https://github.com/acme/beta.git');
  await page.locator('#project-github-user').fill('acme-bot');
  await page.locator('#project-github-credential').fill('ghp_123');
  await page.locator('#project-save').click();

  await expect(page.locator('#project-config-panel')).toBeHidden();
  await expect(page.locator('#toast-region .toast').last()).toContainText('Project saved: beta');
  await expect(page.locator('#project-list .project-tile')).toHaveCount(1);
  await expect(page.locator('#project-list .project-name')).toHaveText('beta');
  await expect(page.locator('#project-list button[data-project-action="remove"]')).toHaveCount(0);
  await expect(page.locator('#project-list .project-loop-id')).toContainText('loop-gamma');

  await page.locator('#project-list button[data-project-action="review-work"][data-loop-id="loop-gamma"]').click();
  await expect(page.locator('#project-action-status')).toContainText('Review started for loop-gamma');
  await expect(page.locator('#project-list .project-loop-row', { hasText: 'loop-gamma' }).locator('.project-meta .badge')).toContainText('in review');

  await page.locator('#project-list button[data-project-action="approve"][data-loop-id="loop-gamma"]').click();
  await expect(page.locator('#project-action-status')).toContainText('Approved work for loop-gamma');
  await expect(page.locator('#project-list .project-loop-row', { hasText: 'loop-gamma' }).locator('.project-meta .badge')).toContainText('approved');

  await page.locator('#project-list button[data-project-action="submit-pr"][data-loop-id="loop-gamma"]').click();
  await expect(page.locator('#project-action-status')).toContainText('PR submitted for loop-gamma');
  await expect(page.locator('#project-list .project-loop-row', { hasText: 'loop-gamma' }).locator('.project-meta .badge')).toContainText('pr submitted');

  await page.locator('#project-list button[data-project-action="edit"]').click();
  await expect(page.locator('#project-config-panel')).toBeVisible();
  await expect(page.locator('#project-delete')).toBeVisible();
  await expect(page.locator('#project-delete-credential')).toBeVisible();
  await page.locator('#project-delete-credential').click();
  await expect
    .poll(() => page.evaluate(() => window.__lastConfirmMessage))
    .toBe('Are you sure you wish to delete this project credential?');
  await expect(page.locator('#project-config-panel')).toBeHidden();
  await expect(page.locator('#toast-region .toast').last()).toContainText('Project credential deleted');

  await page.locator('#project-list button[data-project-action="submit-pr"][data-loop-id="loop-gamma"]').click();
  await expect(page.locator('#project-action-status')).toContainText('missing GitHub credential');

  await page.locator('#project-list button[data-project-action="edit"]').click();
  await expect(page.locator('#project-config-panel')).toBeVisible();
  await page.locator('#project-delete').click();
  await expect
    .poll(() => page.evaluate(() => window.__lastConfirmMessage))
    .toBe('Are you sure you wish to delete project beta?');
  await expect(page.locator('#project-config-panel')).toBeHidden();
  await expect(page.locator('#project-list .project-tile')).toHaveCount(0);
  await expect(page.locator('#toast-region .toast').last()).toContainText('Project removed: beta');
});

test('starts loop from project issue with loop modal flow', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await page.goto('/');

  await page.evaluate(async () => {
    await fetch('/v1/auth/codex/connect/api-key', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        actor: 'operator',
        api_key: 'sk-test-123',
        account_id: 'acct-api',
      }),
    });
  });

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-providers').click();
  await page.locator('#provider-list button[data-provider-config="codex"]').click();
  await expect(page.locator('#provider-config-panel')).toBeVisible();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(1);
  await page.locator('#provider-back').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-projects').click();
  await page.locator('#project-add').click();
  await page.locator('#project-name').fill('alpha');
  await page.locator('#project-repo-url').fill('https://github.com/callmeradical/smith.git');
  await page.locator('#project-github-user').fill('smith-bot');
  await page.locator('#project-github-credential').fill('ghp_test_123');
  await page.locator('#project-save').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-pods').click();
  await page.keyboard.press('n');

  await page.locator('#pod-create-next').click();
  await page.locator('#pod-create-project').selectOption({ label: 'alpha' });
  await expect(page.locator('#pod-create-issue option[value="132"]')).toHaveCount(1);
  await page.locator('#pod-create-issue').selectOption('132');
  await page.locator('#pod-create-next').click();

  await expect(page.locator('#pod-create-loop-name')).toHaveValue('');
  await expect(page.locator('#pod-create-branch')).toHaveValue('alpha-issue-132');
  await expect(page.locator('#pod-create-provider')).toHaveValue('codex');
  await page.locator('#pod-create-prompt').fill('Focus on pods and providers UX clean-up.');
  await page.locator('#pod-create-submit').click();

  await expect(page.locator('#pod-create-panel')).toBeHidden();
  await expect(page.locator('#grid .pod-tile', { hasText: 'alpha-issue-132-simplified-console-ux' })).toHaveCount(1);
  await expect
    .poll(() => api.createdLoopPayloads.length)
    .toBe(1);
  expect(api.createdLoopPayloads[0]).toMatchObject({
    provider_id: 'codex',
    source_type: 'github_issue',
    source_ref: 'callmeradical/smith#132',
    metadata: {
      workspace_branch: 'alpha-issue-132',
      workspace_agent: 'codex',
    },
  });
  expect(api.createdLoopPayloads[0].loop_id).toContain('alpha-issue-132-');
});

test('starts prompt-based loop when no issue is selected', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await page.goto('/');

  await page.evaluate(async () => {
    await fetch('/v1/auth/codex/connect/api-key', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        actor: 'operator',
        api_key: 'sk-test-123',
        account_id: 'acct-api',
      }),
    });
  });

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-providers').click();
  await page.locator('#provider-list button[data-provider-config="codex"]').click();
  await expect(page.locator('#provider-config-panel')).toBeVisible();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(1);
  await page.locator('#provider-back').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-projects').click();
  await page.locator('#project-add').click();
  await page.locator('#project-name').fill('alpha');
  await page.locator('#project-repo-url').fill('https://github.com/callmeradical/smith.git');
  await page.locator('#project-github-user').fill('smith-bot');
  await page.locator('#project-github-credential').fill('ghp_test_123');
  await page.locator('#project-save').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-pods').click();
  await page.keyboard.press('n');

  await page.locator('#pod-create-next').click();
  await page.locator('#pod-create-project').selectOption({ label: 'alpha' });
  await page.locator('#pod-create-next').click();

  await expect(page.locator('#pod-create-provider')).toHaveValue('codex');
  await page.locator('#pod-create-prompt').fill('Create a PRD for improving pod runtime diagnostics.');
  await page.locator('#pod-create-submit').click();

  await expect(page.locator('#pod-create-panel')).toBeHidden();
  await expect
    .poll(() => api.createdLoopPayloads.length)
    .toBe(1);
  expect(api.createdLoopPayloads[0]).toMatchObject({
    provider_id: 'codex',
    source_type: 'prompt',
    metadata: {
      workspace_prompt: 'Create a PRD for improving pod runtime diagnostics.',
      workspace_agent: 'codex',
    },
  });
  expect(String(api.createdLoopPayloads[0].source_ref || '')).toContain('prompt:');
});

test('accepts pasted PRD JSON in loop prompt field', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await page.goto('/');

  await page.evaluate(async () => {
    await fetch('/v1/auth/codex/connect/api-key', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        actor: 'operator',
        api_key: 'sk-test-123',
        account_id: 'acct-api',
      }),
    });
  });

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-providers').click();
  await page.locator('#provider-list button[data-provider-config="codex"]').click();
  await expect(page.locator('#provider-config-panel')).toBeVisible();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(1);
  await page.locator('#provider-back').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-projects').click();
  await page.locator('#project-add').click();
  await page.locator('#project-name').fill('alpha');
  await page.locator('#project-repo-url').fill('https://github.com/callmeradical/smith.git');
  await page.locator('#project-github-user').fill('smith-bot');
  await page.locator('#project-github-credential').fill('ghp_test_123');
  await page.locator('#project-save').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-pods').click();
  await page.keyboard.press('n');

  await page.locator('#pod-create-next').click();
  await page.locator('#pod-create-project').selectOption({ label: 'alpha' });
  await page.locator('#pod-create-next').click();

  await expect(page.locator('#pod-create-provider')).toHaveValue('codex');
  await page
    .locator('#pod-create-prompt')
    .fill('{"stories":[{"id":"US-001","title":"Story","status":"open"}]}');
  await page.locator('#pod-create-submit').click();

  await expect
    .poll(() => api.createdLoopPayloads.length)
    .toBe(1);
  expect(api.createdLoopPayloads[0]).toMatchObject({
    provider_id: 'codex',
    metadata: {
      workspace_prd_path: '.agents/tasks/prd.json',
      workspace_prompt: '',
    },
  });
  expect(String(api.createdLoopPayloads[0].metadata.workspace_prd_json || '')).toContain('"stories"');
});

test('starts generate PRD loop from method selector', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await page.goto('/');

  await page.evaluate(async () => {
    await fetch('/v1/auth/codex/connect/api-key', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        actor: 'operator',
        api_key: 'sk-test-123',
        account_id: 'acct-api',
      }),
    });
  });

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-providers').click();
  await page.locator('#provider-list button[data-provider-config="codex"]').click();
  await expect(page.locator('#provider-config-panel')).toBeVisible();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(1);
  await page.locator('#provider-back').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-projects').click();
  await page.locator('#project-add').click();
  await page.locator('#project-name').fill('alpha');
  await page.locator('#project-repo-url').fill('https://github.com/callmeradical/smith.git');
  await page.locator('#project-github-user').fill('smith-bot');
  await page.locator('#project-github-credential').fill('ghp_test_123');
  await page.locator('#project-save').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-pods').click();
  await page.keyboard.press('n');
  await page.locator('[data-pod-create-method="generate_prd"]').click();

  await page.locator('#pod-create-next').click();
  await page.locator('#pod-create-project').selectOption({ label: 'alpha' });
  await page.locator('#pod-create-next').click();
  await page.locator('#pod-create-prompt').fill('Build a PRD for better pod runtime diagnostics.');
  await page.locator('#pod-create-submit').click();

  await expect
    .poll(() => api.createdLoopPayloads.length)
    .toBe(1);
  expect(api.createdLoopPayloads[0]).toMatchObject({
    provider_id: 'codex',
    source_type: 'interactive_prompt',
    metadata: {
      invocation_method: 'generate_prd',
      workspace_prompt: 'Build a PRD for better pod runtime diagnostics.',
    },
  });
});

test('starts load PRD loop from method selector', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await page.goto('/');

  await page.evaluate(async () => {
    await fetch('/v1/auth/codex/connect/api-key', {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        actor: 'operator',
        api_key: 'sk-test-123',
        account_id: 'acct-api',
      }),
    });
  });

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-providers').click();
  await page.locator('#provider-list button[data-provider-config="codex"]').click();
  await expect(page.locator('#provider-config-panel')).toBeVisible();
  await expect(page.locator('#provider-list .provider-card.connected')).toHaveCount(1);
  await page.locator('#provider-back').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-projects').click();
  await page.locator('#project-add').click();
  await page.locator('#project-name').fill('alpha');
  await page.locator('#project-repo-url').fill('https://github.com/callmeradical/smith.git');
  await page.locator('#project-github-user').fill('smith-bot');
  await page.locator('#project-github-credential').fill('ghp_test_123');
  await page.locator('#project-save').click();

  await page.locator('.page.active .sidebar-toggle').click();
  await page.locator('#nav-pods').click();
  await page.keyboard.press('n');
  await page.locator('[data-pod-create-method="load_prd"]').click();

  await page.locator('#pod-create-next').click();
  await page.locator('#pod-create-project').selectOption({ label: 'alpha' });
  await page.locator('#pod-create-next').click();
  await page
    .locator('#pod-create-prompt')
    .fill('{"stories":[{"id":"US-001","title":"Story","status":"open"}]}');
  await page.locator('#pod-create-submit').click();

  await expect
    .poll(() => api.createdLoopPayloads.length)
    .toBe(1);
  expect(api.createdLoopPayloads[0]).toMatchObject({
    provider_id: 'codex',
    source_type: 'prompt',
    metadata: {
      invocation_method: 'load_prd',
      workspace_prd_path: '.agents/tasks/prd.json',
      workspace_prompt: '',
    },
  });
  expect(String(api.createdLoopPayloads[0].metadata.workspace_prd_json || '')).toContain('"stories"');
});
