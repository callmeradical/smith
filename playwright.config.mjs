import { defineConfig, devices } from '@playwright/test';

const baseURL = process.env.PW_UI_BASE_URL || 'http://127.0.0.1:4173';
const useLocalWebServer = !process.env.PW_UI_BASE_URL;

export default defineConfig({
  testDir: './test/playwright',
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: [
    ['list'],
    ['html', { outputFolder: 'output/playwright/report', open: 'never' }],
  ],
  outputDir: 'output/playwright/test-results',
  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  webServer: useLocalWebServer
    ? {
        command: 'python3 -m http.server 4173 --bind 127.0.0.1 --directory console',
        url: 'http://127.0.0.1:4173',
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      }
    : undefined,
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
      },
    },
  ],
});
