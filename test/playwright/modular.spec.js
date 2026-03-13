import { expect, test } from '@playwright/test';

test.describe('Modular Console', () => {
  test.beforeEach(async ({ page }) => {
    // Mock API
    await page.route('**/api/v1/loops', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            record: {
              loop_id: 'loop-1',
              project_id: 'proj-1',
              state: 'unresolved',
              attempt: 1,
              reason: 'testing',
            },
            revision: 1,
          },
          {
            record: {
              loop_id: 'loop-2',
              project_id: 'proj-1',
              state: 'running',
              attempt: 2,
              reason: 'testing 2',
            },
            revision: 2,
          },
          {
            record: {
              loop_id: 'loop-3',
              project_id: 'proj-2',
              state: 'flatline',
              attempt: 1,
              reason: 'failed',
            },
            revision: 1,
          },
        ]),
      });
    });

    await page.route('**/api/v1/documents', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await page.route('**/api/v1/auth/codex/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ connected: false }),
      });
    });

    await page.route('**/api/v1/projects', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'proj-1',
            name: 'Project 1',
            repo_url: 'https://github.com/org/repo1'
          }
        ]),
      });
    });

    await page.goto('/');
  });

  test('should render loops and stats', async ({ page }) => {
    await expect(page.locator('#stat-total')).toHaveText('3');
    await expect(page.locator('#stat-active')).toHaveText('2');
    await expect(page.locator('#stat-flatline')).toHaveText('1');
    await expect(page.locator('#grid .pod-tile')).toHaveCount(3);
  });

  test('should filter loops by state', async ({ page }) => {
    await page.selectOption('#state-filter', 'flatline');
    await expect(page.locator('#grid .pod-tile')).toHaveCount(1);
    await expect(page.locator('#grid .pod-tile .loop-id')).toHaveText('loop-3');
  });

  test('should filter loops by search', async ({ page }) => {
    await page.fill('#search', 'loop-2');
    await expect(page.locator('#grid .pod-tile')).toHaveCount(1);
    await expect(page.locator('#grid .pod-tile .loop-id')).toHaveText('loop-2');
  });

  test('should open documents page', async ({ page }) => {
    // Open sidebar
    await page.click('.sidebar-toggle');
    // Click documents link
    await page.click('#nav-documents');
    // Check if documents page is active
    await expect(page.locator('#page-documents')).toHaveClass(/active/);
    await expect(page.locator('#page-pods')).not.toHaveClass(/active/);
  });
});
