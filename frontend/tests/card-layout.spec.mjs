import { test, expect } from "playwright/test";
test("record date and icon alignment", async ({ page }, info) => {
  await page.goto("/?preview=1");
  await expect(page.locator(".clip")).toHaveCount(8);
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    const list = demo.List;
    demo.List = async (...args) => {
      const r = await list(...args);
      r.entries = r.entries.map((e, i) => ({
        ...e,
        updatedAt: i === 0 ? Date.now() : new Date(2025, 9, 1, 12).getTime(),
      }));
      return r;
    };
    window.dispatchEvent(new CustomEvent("history:changed"));
  });
  await expect(page.locator(".clip time").nth(1)).toHaveText("2025/10/01");
  await expect(page.locator(".clip time").first()).not.toContainText("/");
  for (const card of await page.locator(".clip").all()) {
    const b = await card.boundingBox(),
      i = await card.locator(".clip-kind").boundingBox(),
      t = await card.locator("time").boundingBox();
    expect(Math.abs(i.y + i.height / 2 - b.y - b.height / 2)).toBeLessThan(2);
    expect(b.x + b.width - t.x - t.width).toBeLessThan(16);
  }
  await page.screenshot({
    path: `../.work/card-layout-${info.project.name}.png`,
  });
});
