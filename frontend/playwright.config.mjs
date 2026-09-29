import { defineConfig } from "playwright/test";

export default defineConfig({
  testDir: "./tests",
  outputDir: "../.work/browser-results",
  fullyParallel: true,
  workers: 2,
  use: {
    baseURL: "http://127.0.0.1:9246",
    channel: "msedge",
    headless: true,
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "default", use: { viewport: { width: 360, height: 680 } } },
    { name: "minimum", use: { viewport: { width: 320, height: 420 } } },
  ],
  webServer: {
    command: "node node_modules/vite/bin/vite.js --port 9246",
    url: "http://127.0.0.1:9246",
    reuseExistingServer: false,
  },
});
