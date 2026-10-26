import { test, expect } from "playwright/test";

async function settings(page) {
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.locator(".update-check-row").scrollIntoViewIfNeeded();
}

test("checks only on request and opens the reported release", async ({
  page,
}, testInfo) => {
  await page.goto("/?preview=1&update=available");
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    const check = demo.CheckForUpdates;
    window.updateCalls = 0;
    demo.CheckForUpdates = async () => {
      window.updateCalls++;
      return check();
    };
    window.addEventListener(
      "preview:update-release",
      (event) => (window.openedTag = event.detail),
    );
  });
  await settings(page);
  expect(await page.evaluate(() => window.updateCalls)).toBe(0);
  await expect(page.locator(".update-status")).toBeEmpty();
  const versionLabel = await page.locator(".app-version").boundingBox();
  const checkButton = await page
    .getByRole("button", { name: "检查更新", exact: true })
    .boundingBox();
  expect(
    Math.abs(
      versionLabel.y +
        versionLabel.height / 2 -
        checkButton.y -
        checkButton.height / 2,
    ),
  ).toBeLessThan(2);
  await page.screenshot({ path: testInfo.outputPath("update-idle-zh.png") });
  await page.getByRole("button", { name: "检查更新", exact: true }).click();
  await expect(page.locator(".update-status")).toContainText("新版");
  const version = await page.locator(".update-version").innerText();
  await page.getByRole("button", { name: "查看详情" }).click();
  expect(await page.evaluate(() => window.openedTag)).toBe(version);
  expect(await page.evaluate(() => window.updateCalls)).toBe(1);
  const row = page.locator(".update-check-row");
  expect(await row.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(
    true,
  );
  await page.screenshot({
    path: testInfo.outputPath("update-available-zh.png"),
  });
  await page.getByRole("combobox", { name: "语言" }).selectOption("en");
  await expect(
    page.getByRole("button", { name: "View details" }),
  ).toBeVisible();
  await row.scrollIntoViewIfNeeded();
  expect(await row.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(
    true,
  );
  await page.screenshot({
    path: testInfo.outputPath("update-available-en.png"),
  });
});

test("closing settings resets the check and ignores a late response", async ({
  page,
}) => {
  await page.goto("/?preview=1");
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    window.updateCalls = 0;
    demo.CheckForUpdates = () => {
      window.updateCalls++;
      return new Promise((resolve) => (window.finishUpdate = resolve));
    };
  });
  await settings(page);
  await page.getByRole("button", { name: "检查更新", exact: true }).click();
  await expect(page.getByRole("button", { name: "检查中…" })).toBeDisabled();
  expect(await page.evaluate(() => window.updateCalls)).toBe(1);
  await page.keyboard.press("Escape");
  await settings(page);
  await page.evaluate(() =>
    window.finishUpdate({
      status: "available",
      version: "9.9.9",
      tag: "9.9.9",
    }),
  );
  await expect(page.locator(".update-status")).toBeEmpty();
  await expect(
    page.getByRole("button", { name: "检查更新", exact: true }),
  ).toBeEnabled();
});

test("failed checks can retry and English states fit the narrow panel", async ({
  page,
}, testInfo) => {
  await page.goto("/?preview=1");
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    let calls = 0;
    demo.CheckForUpdates = async () => {
      if (++calls === 1) throw Error("无法检查更新，请检查网络连接");
      return { status: "current", version: "0.3.5", tag: "0.3.5" };
    };
  });
  await settings(page);
  await page.getByRole("button", { name: "检查更新", exact: true }).click();
  await expect(page.locator(".update-status")).toHaveText(
    "无法检查更新，请检查网络连接",
  );
  await page.getByRole("combobox", { name: "语言" }).selectOption("en");
  await page.locator(".update-check-row").scrollIntoViewIfNeeded();
  await expect(page.locator(".update-status")).toHaveText(
    "Unable to check. Check your connection.",
  );
  await page.screenshot({ path: testInfo.outputPath("update-error-en.png") });
  await page.getByRole("button", { name: "Retry", exact: true }).click();
  await expect(page.locator(".update-status")).toBeEmpty();
  await expect(page.getByRole("button", { name: "Up to date" })).toBeEnabled();
  expect(
    await page
      .locator(".update-check-row")
      .evaluate((el) => el.scrollWidth <= el.clientWidth),
  ).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("update-current-en.png") });
});
