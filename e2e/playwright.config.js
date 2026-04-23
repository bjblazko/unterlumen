import { defineConfig, devices } from '@playwright/test';
import { fileURLToPath } from 'url';
import path from 'path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  testDir: './specs',
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:8082',
    headless: true,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  webServer: {
    command: `${path.resolve(__dirname, '../unterlumen')} --port 8082 --bind 127.0.0.1`,
    url: 'http://localhost:8082',
    reuseExistingServer: !process.env.CI,
    timeout: 15_000,
    env: {
      UNTERLUMEN_ROOT_PATH: path.resolve(__dirname, 'fixtures'),
    },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
