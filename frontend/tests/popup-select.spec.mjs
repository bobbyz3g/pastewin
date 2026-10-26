import { test, expect } from "playwright/test";

test("settings pop-up menus preserve selection and dismiss before the dialog", async ({
  page,
}, testInfo) => {
  await page.goto("/?preview=1");
  await page.getByRole("button", { name: "设置", exact: true }).click();
  const language = page.getByRole("combobox", { name: "语言" });
  await language.click();
  await expect(language).toHaveJSProperty("value", "zh-CN");
  await page.screenshot({ path: testInfo.outputPath("language-menu.png") });
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "设置" })).toBeVisible();
  await expect(language).toBeFocused();
  await language.click();
  await page.getByRole("option", { name: "English", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  const retention = page.getByRole("combobox", { name: "Keep history" });
  await expect(retention).toBeEnabled();
  await retention.click();
  await expect(
    page.getByRole("option", { name: "30 days", exact: true }),
  ).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("retention-menu.png") });
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await expect(retention).toHaveValue("90");
  await expect(page.getByRole("dialog", { name: "Settings" })).toBeVisible();
  await expect(retention).toBeFocused();
  await page.screenshot({
    path: testInfo.outputPath("settings-popup-buttons.png"),
  });
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).not.toBeVisible();
});
