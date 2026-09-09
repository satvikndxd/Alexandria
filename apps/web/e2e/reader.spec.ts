import { expect, test } from "@playwright/test";

const edition = process.env.READER_EDITION_ID;

test.describe("reader typography", () => {
  test.skip(!edition, "READER_EDITION_ID not seeded in this environment");

  test("type size and theme persist across reloads", async ({ page }) => {
    await page.goto(`/read/${edition}?ch=0`);
    await expect(page.getByRole("button", { name: "XL" })).toBeVisible();

    await page.getByRole("button", { name: "XL", exact: true }).click();
    await page.getByRole("button", { name: "ink", exact: true }).click();

    const stored = await page.evaluate(() => localStorage.getItem("alexandria-reader"));
    expect(stored).toContain('"size":"xl"');
    expect(stored).toContain('"theme":"ink"');

    await page.reload();
    const after = await page.evaluate(() => localStorage.getItem("alexandria-reader"));
    expect(after).toContain('"size":"xl"');

    // The ink theme darkens the reading surface. The surface animates
    // (transition-colors), so wait for the settle instead of sampling mid-flight.
    await page.waitForFunction(
      () => getComputedStyle(document.querySelector("article")!).backgroundColor === "rgb(23, 26, 20)",
      undefined,
      { timeout: 5000 }
    );
  });
});
