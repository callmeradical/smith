import { expect, test } from '@playwright/test';
import { mockEventSource, mockApiRoutes, emitLoopUpdates, loopsFixture } from './helpers.js';

test.describe('Modular Console', () => {
  test.beforeEach(async ({ page }) => {
    await mockEventSource(page);
    await mockApiRoutes(page);
    await page.goto('/pods');
    await emitLoopUpdates(page, loopsFixture);
    // Wait for at least one pod card to render
    await expect(page.locator('.pod-card-container').first()).toBeVisible();
  });

  test('should render loops and stats', async ({ page }) => {
    // Stat cards – the stat value is a sibling span within the same card div
    await expect(page.locator('.grid > div').filter({ hasText: 'Total Pods' }).locator('span.text-3xl')).toHaveText('3');
    await expect(page.locator('.grid > div').filter({ hasText: 'Active Loops' }).locator('span.text-3xl')).toHaveText('2');
    await expect(page.locator('.grid > div').filter({ hasText: 'Flatline' }).locator('span.text-3xl')).toHaveText('1');

    // Pod tiles
    await expect(page.locator('.pod-card-container')).toHaveCount(3);
  });

  test('should filter loops by state', async ({ page }) => {
    await page.locator('select').selectOption('flatline');
    await expect(page.locator('.pod-card-container')).toHaveCount(1);
    await expect(page.locator('.pod-card-container .font-mono').first()).toHaveText('loop-gamma');
  });

  test('should filter loops by search', async ({ page }) => {
    await page.getByPlaceholder('Filter ID...').fill('loop-beta');
    await expect(page.locator('.pod-card-container')).toHaveCount(1);
    await expect(page.locator('.pod-card-container .font-mono').first()).toHaveText('loop-beta');
  });

  test('should navigate to documents page', async ({ page }) => {
    // Click Documents link in the top navbar (visible on desktop)
    await page.locator('a[href="/documents"]').first().click();
    await expect(page).toHaveURL(/\/documents/);
  });
});
