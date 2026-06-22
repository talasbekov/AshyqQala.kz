import { defineConfig, devices } from '@playwright/test';

// Один smoke, закрепляющий сквозной web-путь (маршрут → Query → карточка). Гоняется против
// `vite preview` (собранный SPA); ответ /api замоканный (route.fulfill) — CI без docker.
// Полный байт Postgres→chi→Caddy→React — отдельная ручная проверка `docker compose --profile app up`.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:4173',
    trace: 'on-first-retry',
  },
  webServer: {
    command: 'npm run build && npm run preview -- --port 4173 --strictPort',
    url: 'http://localhost:4173',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
