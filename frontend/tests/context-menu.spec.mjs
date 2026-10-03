import { test, expect } from "playwright/test";

const menu = (page) => page.getByRole("menu", { name: "记录操作" });
const search = (page) => page.getByRole("textbox", { name: "搜索剪贴板" });
const link = (page) =>
  page
    .locator(".clip")
    .filter({ hasText: "https://wails.io/docs/introduction" });

test.beforeEach(async ({ page }) => {
  await page.goto("/?preview=1");
  await expect(page.locator(".clip")).toHaveCount(8);
});

test("right click targets the clicked record even before its full text loads", async ({
  page,
}) => {
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    const get = demo.Get;
    demo.Get = async (id) => {
      if (id === 2)
        await new Promise((resolve) => {
          window.releaseEntry = resolve;
        });
      return get(id);
    };
  });
  await link(page).click({ button: "right" });
  await expect(link(page)).toHaveAttribute("aria-pressed", "true");
  await expect(
    menu(page).getByRole("menuitem", { name: "收藏", exact: true }),
  ).toBeEnabled();
  await menu(page).getByRole("menuitem", { name: "收藏", exact: true }).click();
  await expect(link(page).locator(".starred")).toHaveCount(1);
  await expect(page.getByRole("button", { name: /^收藏 3/ })).toBeVisible();
  await page.evaluate(() => window.releaseEntry?.());
  await link(page).click({ button: "right" });
  await menu(page).getByRole("menuitem", { name: "取消收藏" }).click();
  await expect(link(page).locator(".starred")).toHaveCount(0);
});

test("context deletion removes the clicked record and preserves its neighbours", async ({
  page,
}) => {
  await link(page).click({ button: "right" });
  await menu(page).getByRole("menuitem", { name: "删除", exact: true }).click();
  await expect(link(page)).toHaveCount(0);
  await expect(page.locator(".clip")).toHaveCount(7);
  await expect(page.locator(".clip.selected")).toContainText("项目笔记");
  await expect(search(page)).toBeFocused();
  await expect(menu(page)).toHaveCount(0);
});

test("unpinning in favourites removes only that record from the filtered list", async ({
  page,
}) => {
  await page.getByRole("button", { name: /^收藏 2/ }).click();
  await expect(page.locator(".clip")).toHaveCount(2);
  await page.locator(".clip").first().click({ button: "right" });
  await menu(page).getByRole("menuitem", { name: "取消收藏" }).click();
  await expect(page.locator(".clip")).toHaveCount(1);
  await expect(page.locator(".clip.selected")).toContainText("项目笔记");
  await page.getByRole("button", { name: /^全部 8/ }).click();
  await expect(page.locator(".clip")).toHaveCount(8);
});

test("menu Escape keeps search; menu keys never invoke history paste or Delete", async ({
  page,
}) => {
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    window.pasteCalls = 0;
    demo.Paste = async () => {
      window.pasteCalls++;
    };
  });
  await search(page).fill("https");
  await expect(page.locator(".clip")).toHaveCount(1);
  await link(page).focus();
  await link(page).press("Shift+F10");
  await expect(menu(page)).toBeVisible();
  await page.keyboard.press("Delete");
  await expect(link(page)).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(menu(page)).toHaveCount(0);
  await expect(search(page)).toHaveValue("https");
  await expect(search(page)).toBeFocused();
  await link(page).click({ button: "right" });
  await page.keyboard.press("ArrowDown");
  await expect(
    menu(page).getByRole("menuitem", { name: "删除" }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByText("没有找到相关内容")).toBeVisible();
  expect(await page.evaluate(() => window.pasteCalls)).toBe(0);
});

test("menu stays inside the window and closes on outside click, scrolling and reopening", async ({
  page,
}, testInfo) => {
  const card = page.locator(".clip").last();
  await card.scrollIntoViewIfNeeded();
  const box = await card.boundingBox();
  await card.click({
    button: "right",
    position: { x: box.width - 8, y: box.height - 8 },
  });
  await expect(menu(page)).toBeVisible();
  const bounds = await menu(page).boundingBox();
  const viewport = page.viewportSize();
  expect(bounds.x).toBeGreaterThanOrEqual(8);
  expect(bounds.y).toBeGreaterThanOrEqual(8);
  expect(bounds.x + bounds.width).toBeLessThanOrEqual(viewport.width - 8);
  expect(bounds.y + bounds.height).toBeLessThanOrEqual(viewport.height - 8);
  // The scroll event that revealed the card may be delivered after the menu opens.
  await page.locator(".clip-list").dispatchEvent("scroll");
  await expect(menu(page)).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("context-menu.png") });
  await search(page).click();
  await expect(menu(page)).toHaveCount(0);
  await card.click({ button: "right" });
  await page.mouse.wheel(0, -200);
  await expect(menu(page)).toHaveCount(0);
  await card.click({ button: "right" });
  await page.evaluate(() =>
    window.dispatchEvent(new CustomEvent("panel:shown", { detail: true })),
  );
  await expect(menu(page)).toHaveCount(0);
  await expect(search(page)).toBeFocused();
});

test("image cards support the same menu operations", async ({ page }) => {
  const picture = page.locator(".clip").first();
  await expect(picture.locator("img")).toBeVisible();
  await picture.locator("img").click({ button: "right" });
  await menu(page).getByRole("menuitem", { name: "收藏", exact: true }).click();
  await expect(picture.locator(".starred")).toHaveCount(1);
  await picture.locator("img").click({ button: "right" });
  await menu(page).getByRole("menuitem", { name: "删除", exact: true }).click();
  await expect(page.locator(".clip")).toHaveCount(7);
  await expect(page.locator(".clip.selected")).toContainText(
    "这条记录同时包含文字和图片",
  );
});

test("failed menu actions show the existing error and keep the record", async ({
  page,
}) => {
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    demo.Delete = async () => {
      throw Error("测试：记录删除失败");
    };
  });
  await link(page).click({ button: "right" });
  await menu(page).getByRole("menuitem", { name: "删除", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("记录删除失败");
  await expect(link(page)).toHaveAttribute("aria-pressed", "true");
  await expect(menu(page)).toHaveCount(0);
});
