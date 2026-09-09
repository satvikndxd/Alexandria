import { expect, test } from "@playwright/test";

test("the folio frame renders: wordmark, rail, and motto", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("link", { name: /^Alexandria$/ })).toBeVisible();
  await expect(page.getByRole("link", { name: "My Library" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Discover" })).toBeVisible();
  // The motto is stacked capitals in one paragraph; match the paragraph.
  await expect(page.locator("p").filter({ hasText: "Brighter" }).first()).toBeVisible();
});

test("unknown book renders the lost-folio page", async ({ page }) => {
  await page.goto("/books/no-such-folio");
  await expect(page.getByRole("heading", { name: "Lost Folio" })).toBeVisible();
});

test("search page offers a query field and index choice", async ({ page }) => {
  await page.goto("/search");
  await expect(page.getByLabel("Query")).toBeVisible();
  await expect(page.getByLabel("Search index")).toBeVisible();
});
