import { expect, test } from "@playwright/test";

test("edits, saves, sends, inspects, replays, and runs workflows", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  await page.getByRole("button", { name: "Build an event" }).click();

  await page.locator("select").selectOption("builtin:chat/basic-message");
  const content = page.getByLabel("Content", { exact: true });
  await expect(content).toHaveValue("Hello from Kick Sim");
  await content.fill("stable Studio delivery");

  await page.getByRole("button", { name: "Save as copy" }).click();
  await expect(page.getByLabel("Scenario ID", { exact: true })).toHaveValue("chat/basic-message-copy");
  await page.getByLabel("Scenario ID", { exact: true }).fill("stable/basic-message");
  await page.getByLabel("Scenario name", { exact: true }).fill("Stable chat scenario");
  await page.getByRole("button", { name: "Save copy", exact: true }).click();
  await expect(page.getByText("Saved stable/basic-message")).toBeVisible();

  await page.getByRole("button", { name: /Send event/ }).click();
  await expect(page.getByText("http_accepted · HTTP 204")).toBeVisible();

  await page.getByRole("button", { name: "Activity" }).click();
  await page
    .getByRole("button", { name: /chat\.message\.sent@1/ })
    .first()
    .click();
  await expect(page.getByRole("heading", { name: "Attempt inspector" })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Exact body" }).locator("xpath=following-sibling::pre[1]"),
  ).toContainText("stable Studio delivery");

  const replay = page.waitForResponse(
    (response) => response.url().endsWith("/replay") && response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Replay exact" }).click();
  expect((await replay).status()).toBe(200);

  await page.getByRole("button", { name: "Workflows" }).click();
  await page.getByRole("button", { name: "Run timeline" }).click();
  await expect(page.getByText("7 deliveries passed")).toBeVisible();

  await page.getByRole("button", { name: "Run suite" }).first().click();
  await expect(page.getByText("2 passed, 0 failed")).toBeVisible();
});

test("previews and sends non-chat events, and saves named custom copies", async ({ page }) => {
  page.on("dialog", (dialog) => {
    throw new Error(`Unexpected browser dialog: ${dialog.message()}`);
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Build an event" }).click();
  for (const id of ["channel/new-follower", "livestream/started", "livestream/stopped", "moderation/ban"]) {
    await page.getByLabel("Scenario", { exact: true }).selectOption(`builtin:${id}`);
    await page.getByRole("tab", { name: "Raw HTTP Preview" }).click();
    await expect(page.locator(".code-preview")).toContainText("Kick-Event-Signature:");
    await expect(page.getByRole("alert")).toHaveCount(0);
    await page.getByRole("button", { name: /Send event/ }).click();
    await expect(page.getByText("http_accepted · HTTP 204")).toBeVisible();
  }
  await page.getByRole("button", { name: "Scenarios", exact: true }).click();
  const card = page
    .locator(".item-card")
    .filter({ has: page.getByRole("heading", { name: "livestream-started", exact: true }) });
  await card.getByRole("button", { name: "Duplicate" }).click();
  const dialog = page.getByRole("dialog", { name: "Save scenario copy" });
  await expect(dialog.getByLabel("Scenario ID", { exact: true })).toHaveValue("livestream/started-copy");
  await dialog.getByLabel("Scenario ID", { exact: true }).fill("builtin:invalid-copy");
  await expect(dialog.getByRole("button", { name: "Save copy", exact: true })).toBeDisabled();
  await dialog.getByLabel("Scenario ID", { exact: true }).fill("livestream/started-copy");
  await dialog.getByLabel("Scenario name", { exact: true }).fill("My stream start");
  await dialog.getByRole("button", { name: "Save copy", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  const copy = page
    .locator(".item-card")
    .filter({ has: page.getByRole("heading", { name: "My stream start", exact: true }) });
  await expect(copy.getByText("Custom", { exact: true })).toBeVisible();
  await copy.getByRole("button", { name: "Edit source" }).click();
  await page.getByRole("button", { name: "Save as copy", exact: true }).click();
  await dialog.getByLabel("Scenario name", { exact: true }).fill("My source copy");
  await dialog.getByRole("button", { name: "Save copy", exact: true }).click();
  await expect(page.getByRole("heading", { name: "My source copy", exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Keys & setup", exact: true }).click();
  const rotate = page.getByRole("button", { name: "Rotate key pair", exact: true });
  await rotate.click();
  await expect(page.getByRole("dialog", { name: "Rotate simulator key pair?" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(rotate).toBeFocused();
});
