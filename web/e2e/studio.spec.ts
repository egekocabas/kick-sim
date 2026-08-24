import { expect, test } from "@playwright/test";

test("edits, saves, sends, inspects, replays, and runs workflows", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  await page.getByRole("button", { name: "Build an event" }).click();

  await page.locator("select").selectOption("builtin:chat/basic-message");
  const content = page.getByLabel("Content", { exact: true });
  await expect(content).toHaveValue("Hello from Kick Sim");
  await content.fill("stable Studio delivery");

  page.once("dialog", async (dialog) => dialog.accept("stable/basic-message"));
  await page.getByRole("button", { name: "Save as copy" }).click();
  await expect(page.getByText("Saved stable/basic-message")).toBeVisible();

  await page.getByRole("button", { name: /Send event/ }).click();
  await expect(page.getByText("http_accepted · HTTP 204")).toBeVisible();

  await page.getByRole("button", { name: "Activity" }).click();
  await page.getByRole("button", { name: /chat\.message\.sent@1/ }).first().click();
  await expect(page.getByRole("heading", { name: "Attempt inspector" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Exact body" }).locator("xpath=following-sibling::pre[1]")).toContainText("stable Studio delivery");

  const replay = page.waitForResponse((response) => response.url().endsWith("/replay") && response.request().method() === "POST");
  await page.getByRole("button", { name: "Replay exact" }).click();
  expect((await replay).status()).toBe(200);

  await page.getByRole("button", { name: "Workflows" }).click();
  await page.getByRole("button", { name: "Run timeline" }).click();
  await expect(page.getByText("7 deliveries passed")).toBeVisible();

  await page.getByRole("button", { name: "Run suite" }).first().click();
  await expect(page.getByText("2 passed, 0 failed")).toBeVisible();
});
