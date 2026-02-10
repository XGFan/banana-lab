import { defineConfig, devices } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const repoRoot = path.resolve(__dirname, "..");

export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:5173",
    headless: true,
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
      },
    },
  ],
  webServer: [
    {
      command: "pids=$(lsof -ti tcp:18080 || true); if [ -n \"$pids\" ]; then kill -9 $pids; fi; npm --prefix web run build && BANANA_FORCE_RESET_DB=1 SERVER_PORT=18080 PROXY_BASE_URL=http://127.0.0.1:18081 PROXY_AUTH_HEADER_VALUE=mock MODEL_ID=mock STORAGE_ROOT=./storage-e2e DB_PATH=./storage-e2e/banana-lab.db go run ./cmd/server",
      cwd: repoRoot,
      url: "http://127.0.0.1:18080/api/health",
      reuseExistingServer: !process.env.CI,
    },
    {
      command: "pids=$(lsof -ti tcp:18081 || true); if [ -n \"$pids\" ]; then kill -9 $pids; fi; go run ./cmd/mock-llm -port 18081 -storage ./storage/images -seed 1",
      cwd: repoRoot,
      url: "http://127.0.0.1:18081/mock-images/fallback.png",
      reuseExistingServer: !process.env.CI,
    },
    {
      command: "pids=$(lsof -ti tcp:5173 || true); if [ -n \"$pids\" ]; then kill -9 $pids; fi; VITE_BACKEND_ORIGIN=http://127.0.0.1:18080 VITE_E2E=1 npm run dev -- --host 127.0.0.1 --port 5173",
      cwd: path.resolve(__dirname),
      url: "http://127.0.0.1:5173",
      reuseExistingServer: !process.env.CI,
    },

  ],
});
