import { defineConfig } from "@playwright/test"

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? "http://127.0.0.1:4173"

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  fullyParallel: true,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  webServer: {
    command: "npm run preview -- --host 127.0.0.1 --port 4173",
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
  projects: [
    {
      name: "mobile",
      use: {
        browserName: "chromium",
        isMobile: true,
        hasTouch: true,
        viewport: { width: 390, height: 844 },
      },
    },
    {
      name: "tablet",
      use: {
        browserName: "chromium",
        hasTouch: true,
        viewport: { width: 768, height: 1024 },
      },
    },
    {
      name: "desktop",
      use: {
        browserName: "chromium",
        viewport: { width: 1280, height: 900 },
      },
    },
  ],
})
