import { defineConfig } from "@playwright/test";

/**
 * E2E against a running stack (api + web + postgres). The suite asserts the
 * folio chrome and the reader's persisted typography — the two surfaces a
 * unit test cannot see. Specs that need seeded data skip honestly when the
 * environment does not provide it.
 */
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: process.env.BASE_URL ?? "http://127.0.0.1:3000",
    headless: true,
    // Containerised CI/sandbox browsers need these; a developer machine
    // ignores them harmlessly.
    launchOptions: {
      args: [
        "--no-sandbox",
        "--disable-dev-shm-usage",
        "--disable-gpu",
        // Constrained sandboxes can fail V8's big virtual-memory reservation;
        // jitless mode avoids it entirely at the cost of speed.
        ...(process.env.PW_JITLESS ? ["--js-flags=--jitless"] : []),
      ],
    },
  },
  projects: [
    {
      name: "chromium",
      use: {
        browserName: "chromium",
        // Sandboxes/CI without a full graphics stack can select the lighter
        // headless shell: PW_CHANNEL=chromium-headless-shell.
        ...(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {}),
      },
    },
  ],
});
