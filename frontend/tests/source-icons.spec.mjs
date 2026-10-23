import { test, expect } from "playwright/test";
test("source icons share URLs and fall back without changing card layout", async ({
  page,
}) => {
  const good = "a".repeat(64),
    bad = "b".repeat(64);
  await page.route("**/source-icon/*", (route) =>
    route.request().url().includes(good)
      ? route.fulfill({
          contentType: "image/svg+xml",
          body: '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" fill="blue"/></svg>',
        })
      : route.fulfill({ status: 404, body: "" }),
  );
  await page.goto("/?preview=1");
  await expect(page.locator(".clip")).toHaveCount(8);
  const height = (await page.locator(".clip").first().boundingBox()).height;
  await page.evaluate(
    async ({ good, bad }) => {
      const { demo } = await import("/src/preview.ts");
      const list = demo.List;
      demo.List = async (...args) => {
        const r = await list(...args);
        r.entries = r.entries.map((e, i) => ({
          ...e,
          sourceKey: i < 2 ? good : bad,
          sourceName: "fixture",
        }));
        return r;
      };
      window.dispatchEvent(new CustomEvent("history:changed"));
    },
    { good, bad },
  );
  const icons = page.locator(".clip .source-icon");
  await expect(icons).toHaveCount(2);
  expect(await icons.first().getAttribute("src")).toEqual(
    await icons.nth(1).getAttribute("src"),
  );
  expect((await page.locator(".clip").first().boundingBox()).height).toEqual(
    height,
  );
  await expect(
    page.locator(".clip").nth(2).locator(".clip-kind svg:not(.starred)"),
  ).toBeVisible();
});
