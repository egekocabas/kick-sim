import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: "http://127.0.0.1:14321",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "node ./scripts/e2e-server.mjs",
    url: "http://127.0.0.1:14321/api/bootstrap",
    timeout: 120_000,
    reuseExistingServer: false,
  },
});
