import { test, expect } from "playwright/test";

test("toolbar stays available while the newly selected full text loads", async ({
  page,
}) => {
  await page.goto("/?preview=1");
  await expect(page.locator(".clip")).toHaveCount(8);
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    const get = demo.Get;
    demo.Get = async (id) => {
      await new Promise((resolve) => {
        window.releaseEntry = resolve;
      });
      return get(id);
    };
  });
  await page.getByRole("textbox", { name: "搜索剪贴板" }).press("ArrowDown");
  await expect(page.locator(".clip").nth(1)).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  const toolbar = page.getByRole("toolbar");
  for (const name of ["查看内容", "收藏记录", "复制", "删除记录"]) {
    await expect(
      toolbar.getByRole("button", { name, exact: true }),
    ).toBeEnabled();
  }
  await toolbar.getByRole("button", { name: "查看内容", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("status")).toHaveText(
    "读取中",
  );
  await page.evaluate(() => window.releaseEntry());
  await expect(page.getByRole("dialog").getByRole("status")).toHaveCount(0);
});

test("floating tray leaves the final record reachable by keyboard", async ({
  page,
}, testInfo) => {
  await page.goto("/?preview=1");
  const cards = page.locator(".clip");
  await expect(cards).toHaveCount(8);
  const tray = page.getByRole("toolbar");
  const initialTray = await tray.boundingBox();
  const list = await page.locator(".clip-list").boundingBox();
  expect(list.y + list.height).toBeGreaterThan(
    initialTray.y + initialTray.height,
  );
  await page.getByRole("textbox", { name: "搜索剪贴板" }).focus();
  for (let i = 0; i < 7; i++) await page.keyboard.press("ArrowDown");
  await expect(cards.last()).toHaveAttribute("aria-pressed", "true");
  await expect
    .poll(async () => {
      const last = await cards.last().boundingBox();
      return last.y + last.height;
    })
    .toBeLessThanOrEqual(initialTray.y);
  expect((await tray.boundingBox()).y).toBe(initialTray.y);
  await page.screenshot({
    path: `../.work/floating-tray-${testInfo.project.name}.png`,
    animations: "disabled",
  });
});
