import { expect, test } from '@playwright/test';
import { mockEventSource, mockApiRoutes, emitLoopUpdates, loopsFixture } from './helpers.js';

test.describe('Deployment Verification', () => {
  test('should load Svelte app and show Pods page without errors', async ({ page }) => {
    const errors = [];
    page.on('pageerror', (err) => errors.push(err.message));

    await mockEventSource(page);
    await mockApiRoutes(page);

    await page.goto('/');
    // The root route redirects to /pods
    await page.waitForURL('**/pods');

    await emitLoopUpdates(page, loopsFixture);

    // Verify heading
    await expect(page.locator('h1')).toContainText('Pods');

    // No console errors
    expect(errors).toHaveLength(0);
  });
});
