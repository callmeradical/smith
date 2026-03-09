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

async function mockConsoleApi(page) {
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
  const overridePayloads = [];
  const createdLoopPayloads = [];

  await page.addInitScript(() => {
    window.__lastConfirmMessage = '';
    window.confirm = (message) => {
      window.__lastConfirmMessage = String(message || '');
      return true;
    };

    class MockEventSource {
      constructor(url) {
        this.url = url;
        this.listeners = {};
        this.readyState = 1;
        setTimeout(() => this.emit('ready', { data: '{}' }), 0);
        setTimeout(
          () =>
            this.emit('entry', {
              data: JSON.stringify({
                entry: {
                  sequence: Date.now(),
                  timestamp: new Date().toISOString(),
                  level: 'info',
                  phase: 'replica',
                  actor_id: 'replica-test',
                  message: 'worker started',
                },
              }),
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

  await page.route(/\/v1\/control\/override$/, async (route) => {
    const payload = route.request().postDataJSON();
    overridePayloads.push(payload);
    await jsonResponse(route, { revision: 101 });
  });

  await page.route(/\/v1\/loops\/[^/]+\/journal(\?.*)?$/, async (route) => {
    await jsonResponse(route, []);
  });

  return { overridePayloads, createdLoopPayloads };
}

test('renders loop tiles and summary stats', async ({ page }) => {
  await mockConsoleApi(page);
  await page.goto('/');

  await expect(page.locator('#stat-total')).toHaveText('3');
  await expect(page.locator('#stat-active')).toHaveText('2');
  await expect(page.locator('#stat-flatline')).toHaveText('1');
  await expect(page.locator('#grid .pod-group')).toHaveCount(2);
  await expect(page.locator('#grid .pod-tile')).toHaveCount(3);
  await expect(page.locator('#pod-detail')).toBeHidden();

  await page.locator('#grid .pod-tile', { hasText: 'loop-alpha' }).click();
  await expect(page.locator('#pod-detail')).toBeVisible();
  await expect(page.locator('#journal-title')).toContainText('loop-alpha');
  await expect(page.locator('#terminal')).toContainText('worker started');
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
  await expect(page.locator('#project-list .project-tile')).toHaveCount(1);
  await expect(page.locator('#project-list .project-name')).toHaveText('beta');
  await expect(page.locator('#project-list .project-repo')).toContainText(
    'https://github.com/acme/beta.git',
  );
  await expect(page.locator('#project-list')).toContainText('GitHub: acme-bot');
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
});

test('starts loop from project issue with workspace form flow', async ({ page }) => {
  const api = await mockConsoleApi(page);
  await page.goto('/');

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

  await page.locator('#pod-create-project').selectOption({ label: 'alpha' });
  await page.locator('#pod-create-load-issues').click();
  await expect(page.locator('#pod-create-status')).toContainText('Loaded 2 open issue(s).');

  await page.locator('#pod-create-issue').selectOption('132');
  await page.locator('#pod-create-branch').fill('feat/issue-132-prd');
  await page.locator('#pod-create-source-branch').fill('main');
  await page.locator('#pod-create-agent').selectOption('codex');
  await page.locator('#pod-create-prompt').fill('Focus on pods and providers UX clean-up.');
  await page.locator('#pod-create-submit').click();

  await expect(page.locator('#pod-create-panel')).toBeHidden();
  await expect(page.locator('#grid .pod-tile', { hasText: 'alpha/feat/issue-132-prd' })).toHaveCount(1);
  await expect
    .poll(() => api.createdLoopPayloads.length)
    .toBe(1);
  expect(api.createdLoopPayloads[0]).toMatchObject({
    loop_id: 'alpha/feat/issue-132-prd',
    source_type: 'github_issue',
    source_ref: 'callmeradical/smith#132',
  });
});
