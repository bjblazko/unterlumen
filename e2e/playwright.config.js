import { defineConfig, devices } from '@playwright/test';
import { fileURLToPath } from 'url';
import path from 'path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  testDir: './specs',
  fullyParallel: false,
  workers: 1,
  retries: 1,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  timeout: 60_000,
  use: {
    baseURL: 'http://localhost:8082',
    headless: true,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 20_000,
  },
  webServer: {
    // Pass the root as a CLI arg (not UNTERLUMEN_ROOT_PATH) so the server runs in
    // desktop/non-server-role mode. This keeps the same browse root while enabling
    // /api/export/save and allowing absolute-path exports from library search results.
    command: `${path.resolve(__dirname, '../unterlumen')} --port 8082 --bind 127.0.0.1 ${path.resolve(__dirname, 'fixtures', 'photos')}`,
    url: 'http://localhost:8082',
    reuseExistingServer: !process.env.CI,
    timeout: 15_000,
    env: {
      UNTERLUMEN_LIB_DIR: path.resolve(__dirname, 'fixtures', '.unterlumen-test'),
    },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
