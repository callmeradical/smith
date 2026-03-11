import { expect, test } from '@playwright/test';

test.describe('Deployment Verification', () => {
  test('should load app and verify modular structure', async ({ page }) => {
    // Navigate to the local deployment
    const url = process.env.PW_UI_BASE_URL || 'http://localhost:8080';
    console.log(`Verifying deployment at ${url}`);
    
    await page.goto(url);

    // Check if the app loads and shows the Pods title
    await expect(page.locator('.topbar-title')).toContainText('Pods');

    // Verify that module files are being loaded (check network requests)
    const [response] = await Promise.all([
      page.waitForResponse(resp => resp.url().includes('/js/modules/state.js') && resp.status() === 200),
    ]);
    expect(response).toBeDefined();

    // Check for any console errors
    const errors = [];
    page.on('pageerror', err => errors.push(err.message));
    
    // Refresh to trigger re-load with error tracking active
    await page.reload();
    await page.waitForTimeout(1000);
    
    expect(errors).toHaveLength(0);
  });
});
