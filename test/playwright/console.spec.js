import { expect, test } from '@playwright/test';
import { mockEventSource, mockApiRoutes, emitLoopUpdates, loopsFixture } from './helpers.js';

// ── helpers ────────────────────────────────────────────────────────

async function setupPods(page) {
  await mockEventSource(page);
  const api = await mockApiRoutes(page);
  await page.goto('/pods');
  await emitLoopUpdates(page, loopsFixture);
  // Wait for at least one pod card to render
  await expect(page.locator('.pod-card-container').first()).toBeVisible();
  return api;
}

// ── tests ──────────────────────────────────────────────────────────

test('renders loop tiles and summary stats', async ({ page }) => {
  await setupPods(page);

  await expect(page.locator('.grid > div').filter({ hasText: 'Total Pods' }).locator('span.text-3xl')).toHaveText('3');
  await expect(page.locator('.pod-card-container')).toHaveCount(3);
});

test('pod detail and command execution', async ({ page }) => {
  const api = await setupPods(page);

  // Click first tile (loop-alpha)
  await page.locator('.pod-card-container', { hasText: 'loop-alpha' }).click();
  await expect(page).toHaveURL(/\/pod-view\/loop-alpha/);

  // Verify title
  await expect(page.locator('h1')).toContainText('Pod: loop-alpha');

  // Fill command input and submit
  await page.getByPlaceholder('Run command').fill('echo ok');
  await page.keyboard.press('Enter');

  // Verify command was sent
  await expect.poll(() => api.commandPayloads.length).toBe(1);
  expect(api.commandPayloads[0].command).toBe('echo ok');
});

test('cancel and terminate from pod detail', async ({ page }) => {
  const api = await setupPods(page);

  // Navigate to pod-view for loop-beta
  await page.locator('.pod-card-container', { hasText: 'loop-beta' }).click();
  await expect(page).toHaveURL(/\/pod-view\/loop-beta/);

  // Click Cancel button
  await page.getByRole('button', { name: 'Cancel' }).click();

  await expect.poll(() => api.overridePayloads.length).toBe(1);
  expect(api.overridePayloads[0].target_state).toBe('cancelled');

  // Click Terminate button
  await page.getByRole('button', { name: 'Terminate' }).click();

  await expect.poll(() => api.overridePayloads.length).toBe(2);
  expect(api.overridePayloads[1].target_state).toBe('flatline');
});

test('filters by state and search', async ({ page }) => {
  await setupPods(page);

  // Filter by search
  await page.getByPlaceholder('Filter ID...').fill('alpha');
  await expect(page.locator('.pod-card-container')).toHaveCount(1);

  // Clear search
  await page.getByPlaceholder('Filter ID...').fill('');
  await expect(page.locator('.pod-card-container')).toHaveCount(3);

  // Filter by state
  await page.locator('select').selectOption('flatline');
  await expect(page.locator('.pod-card-container')).toHaveCount(1);
  await expect(page.locator('.pod-card-container')).toContainText('loop-gamma');
});

test('provider API key config', async ({ page }) => {
  await mockEventSource(page);
  const api = await mockApiRoutes(page);

  await page.goto('/providers');

  // Click Configure button to open drawer
  await page.getByRole('button', { name: 'Configure' }).click();

  // Fill in the API key
  await page.locator('#api-key').fill('sk-test-key');

  // Submit the form
  await page.getByRole('button', { name: 'Update Credentials' }).click();

  // Verify auth state was updated
  await expect.poll(() => api.authState.connected).toBe(true);
});

test('project management', async ({ page }) => {
  await mockEventSource(page);
  const api = await mockApiRoutes(page);

  await page.goto('/projects');

  // Verify existing project is shown
  await expect(page.locator('body')).toContainText('alpha');

  // Click New Project button
  await page.getByRole('button', { name: 'New Project' }).click();

  // Fill project form
  await page.locator('#name').fill('new-project');
  await page.locator('#repo').fill('https://github.com/org/repo.git');

  // Submit
  await page.getByRole('button', { name: 'Create Project' }).click();

  // Verify project was created via API
  await expect.poll(() => api.projectsState.find(p => p.name === 'new-project')).toBeDefined();
});
