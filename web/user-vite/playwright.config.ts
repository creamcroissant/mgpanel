import { defineConfig, devices } from '@playwright/test';

const authStatePath = process.env.PLAYWRIGHT_AUTH_STATE_PATH ?? '.auth/state.json';
const testResultsDir = process.env.PLAYWRIGHT_TEST_RESULTS_DIR ?? 'test-results';
const htmlReportDir = process.env.PLAYWRIGHT_HTML_REPORT_DIR ?? 'playwright-report';
const reuseExistingServer = process.env.PLAYWRIGHT_REUSE_EXISTING_SERVER
  ? process.env.PLAYWRIGHT_REUSE_EXISTING_SERVER === '1'
  : !process.env.CI;

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  outputDir: testResultsDir,
  reporter: [['html', { outputFolder: htmlReportDir }]],
  use: {
    baseURL: 'http://localhost:4173',
    trace: 'on-first-retry',
  },

  projects: [
    {
      name: 'setup',
      testMatch: /00-install\.spec\.ts/,
    },
    {
      name: 'chromium',
      dependencies: ['setup'],
      use: {
        ...devices['Desktop Chrome'],
        storageState: authStatePath,
      },
    },
  ],

  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:4173',
    reuseExistingServer,
    stdout: 'pipe',
  },
});
