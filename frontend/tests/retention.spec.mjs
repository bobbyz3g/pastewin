import { test, expect } from "playwright/test";

async function openSettings(page) {
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await expect(page.getByRole("combobox", { name: "保留期限" })).toBeEnabled();
}

test("shorter retention requires confirmation, preserves favorites, and updates space", async ({
  page,
}, testInfo) => {
  await page.goto("/?preview=1");
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    const old = Date.now() - 10 * 86400000;
    (await demo.Get(7)).updatedAt = old;
    (await demo.Get(1)).updatedAt = old;
  });
  await openSettings(page);
  const space = page.locator(".history-space");
  const before = await space.innerText();
  const select = page.getByRole("combobox", { name: "保留期限" });
  await expect(select).toHaveValue("30");
  await page.locator(".history-settings").scrollIntoViewIfNeeded();
  await page.screenshot({ path: testInfo.outputPath("retention-zh.png") });
  await select.selectOption("7");
  await expect(page.locator(".retention-confirmation")).toContainText(
    "将删除 1 条过期记录",
  );
  await expect(
    page.getByRole("button", { name: "取消", exact: true }),
  ).toBeFocused();
  await page.locator(".retention-confirmation").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("retention-confirm-zh.png"),
  });
  await page.getByRole("combobox", { name: "语言" }).selectOption("en");
  await page.locator(".retention-confirmation").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("retention-confirm-en.png"),
  });
  await page.getByRole("combobox", { name: "Language" }).selectOption("zh-CN");
  await page.getByRole("button", { name: "取消", exact: true }).click();
  await expect(select).toHaveValue("30");
  await expect(select).toBeFocused();
  await expect(space).toHaveText(before);
  await select.selectOption("7");
  await page.getByRole("button", { name: "修改并清理" }).click();
  await expect(select).toHaveValue("7");
  await expect(space).not.toHaveText(before);
  expect(
    await page.evaluate(async () => {
      const { demo } = await import("/src/preview.ts");
      return (await demo.List("", false, 0)).entries.map((entry) => entry.id);
    }),
  ).toEqual(expect.arrayContaining([1]));
  await page.getByRole("combobox", { name: "语言" }).selectOption("en");
  await expect(
    page.getByRole("combobox", { name: "Keep history" }),
  ).toHaveValue("7");
  await page.locator(".history-settings").scrollIntoViewIfNeeded();
  expect(
    await page
      .locator(".history-settings")
      .evaluate((element) => element.scrollWidth <= element.clientWidth),
  ).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("retention-en.png") });
  await page.reload();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await expect(
    page.getByRole("combobox", { name: "Keep history" }),
  ).toHaveValue("7");
});

test("live space counts UTF-8 content and images, then decreases after manual cleanup", async ({
  page,
}) => {
  await page.goto("/?preview=1");
  await openSettings(page);
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    await demo.ClearUnpinned();
  });
  const bytes = await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    return (await demo.List("", false, 0)).entries.reduce(
      (sum, entry) =>
        sum + new TextEncoder().encode(entry.text).length + entry.imageBytes,
      0,
    );
  });
  await expect(page.locator(".history-space")).toHaveText(
    `${(bytes / 1000).toLocaleString("zh-CN", { minimumFractionDigits: 1, maximumFractionDigits: 1 })} KB`,
  );
});

test("closed settings ignore late responses and live refresh ignores older requests", async ({
  page,
}) => {
  await page.goto("/?preview=1");
  await openSettings(page);
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    window.settingsRequests = [];
    demo.HistorySettings = () =>
      new Promise((resolve) => window.settingsRequests.push(resolve));
    window.dispatchEvent(new CustomEvent("history:changed"));
    window.dispatchEvent(new CustomEvent("history:changed"));
  });
  await expect
    .poll(() => page.evaluate(() => window.settingsRequests.length))
    .toBe(2);
  await page.evaluate(() =>
    window.settingsRequests[1]({ retentionDays: 90, usedBytes: 2e6 }),
  );
  await expect(page.locator(".history-space")).toHaveText("2.0 MB");
  await page.evaluate(() =>
    window.settingsRequests[0]({ retentionDays: 7, usedBytes: 1e6 }),
  );
  await expect(page.locator(".history-space")).toHaveText("2.0 MB");
  await page.evaluate(() =>
    window.dispatchEvent(new CustomEvent("history:changed")),
  );
  await page.keyboard.press("Escape");
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    demo.HistorySettings = async () => ({ retentionDays: 30, usedBytes: 3e6 });
  });
  await openSettings(page);
  await page.evaluate(() =>
    window.settingsRequests[2]({ retentionDays: 7, usedBytes: 1e6 }),
  );
  await expect(page.locator(".history-space")).toHaveText("3.0 MB");
  await expect(page.getByRole("combobox", { name: "保留期限" })).toHaveValue(
    "30",
  );
});

test("failed retention saves preserve the existing setting", async ({
  page,
}) => {
  await page.goto("/?preview=1");
  await openSettings(page);
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    demo.SetRetentionDays = async () => {
      throw Error("retention save failed");
    };
  });
  await page.getByRole("combobox", { name: "保留期限" }).selectOption("90");
  await expect(page.getByRole("alert")).toContainText("retention save failed");
  await expect(page.getByRole("combobox", { name: "保留期限" })).toHaveValue(
    "30",
  );
});
