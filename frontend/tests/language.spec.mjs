import { test, expect } from "playwright/test";

test("language changes immediately, preserves content and selection, and survives reload", async ({
  page,
}, testInfo) => {
  await page.goto("/?preview=1");
  const record = page
    .locator(".clip")
    .filter({ hasText: "https://wails.io/docs/introduction" });
  await record.click();
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.screenshot({ path: testInfo.outputPath("settings-zh.png") });
  await page.getByRole("combobox", { name: "语言" }).selectOption("en");
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.getByRole("combobox", { name: "Language" })).toBeEnabled();
  const dialog = page.getByRole("dialog", { name: "Settings" });
  expect(await dialog.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(
    true,
  );
  await page.screenshot({ path: testInfo.outputPath("settings-en.png") });
  await dialog
    .getByRole("button", { name: "Clear history", exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("settings-en-bottom.png"),
  });
  await page.getByRole("button", { name: "Close settings" }).click();
  await expect(record).toHaveAttribute("aria-pressed", "true");
  await record.click({ button: "right" });
  await expect(
    page.getByRole("menuitem", { name: "Favorites", exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await page
    .getByRole("textbox", { name: "Search clipboard" })
    .fill("no matching content");
  await expect(
    page.getByText("No results found", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator(".clip")).toHaveCount(8);
  await page.screenshot({ path: testInfo.outputPath("panel-en.png") });
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Settings", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "View content" }).click();
  await expect(
    page.getByRole("heading", { name: "Content preview" }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("dialog", { name: "Content preview" })
      .getByRole("img", { name: /Image, 640 × 360 pixels/ }),
  ).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("preview-en.png") });
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("combobox", { name: "Language" }).focus();
  await page.keyboard.press("Home");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { name: "设置" })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("textbox", { name: "搜索剪贴板" })).toBeVisible();
});

test("failed language saves retain the current language and report the failure", async ({
  page,
}) => {
  await page.goto("/?preview=1");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    demo.SetLanguage = async () => {
      throw Error("language save failed");
    };
  });
  await page.getByRole("combobox", { name: "语言" }).selectOption("en");
  await expect(page.getByRole("alert")).toContainText("language save failed");
  await expect(page.getByRole("combobox", { name: "语言" })).toHaveValue(
    "zh-CN",
  );
  await page.reload();
  await expect(page.getByRole("textbox", { name: "搜索剪贴板" })).toBeVisible();
});

test("English errors translate known wrappers and retain diagnostic details", async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem("pastewin.preview.language", "en"),
  );
  await page.goto("/?preview=1");
  await expect(
    page.getByRole("textbox", { name: "Search clipboard" }),
  ).toBeVisible();
  await page.evaluate(() =>
    window.dispatchEvent(
      new CustomEvent("app:error", {
        detail: "图片未保存：读取图片剪贴板：OS error 123",
      }),
    ),
  );
  await expect(page.getByRole("alert")).toContainText(
    "Image not saved: Read clipboard image: OS error 123",
  );
});
