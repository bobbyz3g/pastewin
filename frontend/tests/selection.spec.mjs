import { test, expect } from "playwright/test";

const selected = (page) => page.locator(".clip.selected");
const search = (page) => page.getByRole("textbox", { name: "搜索剪贴板" });
const shown = (page, reset = true) =>
  page.evaluate((detail) => {
    window.dispatchEvent(new CustomEvent("panel:shown", { detail }));
  }, reset);
const add = (page, id = 100) =>
  page.evaluate((id) => {
    const fixture = window.historyTest;
    const entry = fixture.entries.find((e) => e.id === id) ?? {
      ...fixture.entries[0],
      id,
      text: `新内容 ${id}`,
      preview: `新内容 ${id}`,
      pinned: false,
    };
    fixture.entries = [entry, ...fixture.entries.filter((e) => e.id !== id)];
    window.dispatchEvent(new CustomEvent("history:changed"));
  }, id);

test.beforeEach(async ({ page }) => {
  await page.goto("/?preview=1");
  await expect(page.locator(".clip")).toHaveCount(8);
  await page.evaluate(async () => {
    const { demo } = await import("/src/preview.ts");
    const fixture = (window.historyTest = {
      entries: Array.from({ length: 65 }, (_, i) => ({
        id: i + 1,
        text: `记录 ${i + 1} 中文测试\n${"长文本换行测试。".repeat(25)}`,
        preview: `记录 ${i + 1} 中文测试\n${"长文本换行测试。".repeat(25)}`,
        characters: 210,
        createdAt: 1000 - i,
        updatedAt: 1000 - i,
        pinned: i % 3 === 0,
        hasImage: false,
        imageWidth: 0,
        imageHeight: 0,
        imageBytes: 0,
      })),
      hold: false,
      pending: [],
      pastes: [],
      copies: [],
      failPaste: false,
    });
    demo.List = async (query, pinned, offset) => {
      const filtered = fixture.entries.filter(
        (e) => (!pinned || e.pinned) && e.text.includes(query),
      );
      const result = structuredClone({
        entries: filtered.slice(offset, offset + 50),
        total: filtered.length,
        all: fixture.entries.length,
        pinned: fixture.entries.filter((e) => e.pinned).length,
      });
      if (fixture.hold && offset === 0) {
        fixture.hold = false;
        await new Promise((resolve) => fixture.pending.push(resolve));
      }
      return result;
    };
    demo.Get = async (id) =>
      structuredClone(fixture.entries.find((e) => e.id === id));
    demo.Delete = async (id) => {
      fixture.entries = fixture.entries.filter((e) => e.id !== id);
      window.dispatchEvent(new CustomEvent("history:changed"));
    };
    demo.Pin = async (id, pinned) => {
      fixture.entries.find((e) => e.id === id).pinned = pinned;
      window.dispatchEvent(new CustomEvent("history:changed"));
    };
    demo.Paste = async (id) => {
      fixture.pastes.push(id);
      if (fixture.failPaste) throw Error("测试：目标窗口未接受粘贴");
    };
    demo.Copy = async (id) => {
      fixture.copies.push(id);
    };
    const status = demo.Status;
    demo.Status = async () => ({ ...(await status()), canPaste: true });
  });
  await shown(page);
  await expect(selected(page)).toContainText("记录 1 中文测试");
});

test("category switching preserves counts and geometry while clearing stale actions", async ({
  page,
}) => {
  const tabs = page.locator(".tab-segment button");
  const geometry = () =>
    tabs.evaluateAll((buttons) =>
      buttons.map((button) => {
        const { x, width } = button.getBoundingClientRect();
        return { x, width };
      }),
    );
  const before = await geometry();
  await page.evaluate(() => {
    window.historyTest.hold = true;
  });
  await tabs.nth(1).click();
  await expect
    .poll(() => page.evaluate(() => window.historyTest.pending.length))
    .toBe(1);
  await expect(tabs.nth(0)).toHaveText("全部 65");
  await expect(tabs.nth(1)).toHaveText("收藏 22");
  await expect(selected(page)).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "粘贴", exact: true }),
  ).toBeDisabled();
  expect(await geometry()).toEqual(before);
  await page.evaluate(() => window.historyTest.pending.shift()());
  await expect(page.locator(".clip")).toHaveCount(22);
  expect(await geometry()).toEqual(before);
  await tabs.nth(0).focus();
  await page.keyboard.press("Enter");
  await expect(tabs.nth(0)).toHaveAttribute("aria-pressed", "true");
  await expect(page.locator(".clip")).toHaveCount(50);
});

test("arrow navigation moves focus from top controls to the history list", async ({
  page,
}) => {
  const capture = page.getByRole("button", { name: "暂停记录", exact: true });
  await capture.focus();
  await page.keyboard.press("ArrowDown");
  await expect(page.locator(".clip-list")).toBeFocused();
  await expect(selected(page)).toContainText("记录 2 中文测试");
  await page.keyboard.press("ArrowUp");
  await expect(page.locator(".clip-list")).toBeFocused();
  await expect(selected(page)).toContainText("记录 1 中文测试");
  await page.keyboard.press("Enter");
  await expect
    .poll(() => page.evaluate(() => window.historyTest.pastes))
    .toEqual([1]);
});

test("reopening clears filters and dialogs, selects latest and scrolls to top", async ({
  page,
}, testInfo) => {
  await page.getByRole("button", { name: /^收藏 \d/ }).click();
  await search(page).fill("记录 4 中文测试");
  await expect(selected(page)).toContainText("记录 4 中文测试");
  await page.getByRole("button", { name: "查看内容", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "内容预览" })).toBeVisible();
  await add(page);
  await shown(page);
  await expect(search(page)).toHaveValue("");
  await expect(search(page)).toBeFocused();
  await expect(page.getByRole("button", { name: /^全部 \d/ })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(selected(page)).toContainText("新内容 100");
  await expect
    .poll(() => page.locator(".clip-list").evaluate((e) => e.scrollTop))
    .toBe(0);
  await page.screenshot({ path: testInfo.outputPath("reopened.png") });
});

test("late captures follow latest until keyboard navigation; Enter uses that selection", async ({
  page,
}) => {
  await add(page);
  await expect(selected(page)).toContainText("新内容 100");
  await page.evaluate(() => {
    window.historyTest.hold = true;
  });
  await add(page, 101);
  await expect
    .poll(() => page.evaluate(() => window.historyTest.pending.length))
    .toBe(1);
  await search(page).press("ArrowDown");
  await expect(selected(page)).toContainText("记录 1 中文测试");
  await page.evaluate(() => window.historyTest.pending.shift()());
  await expect(page.locator(".clip").first()).toContainText("新内容 101");
  await expect(selected(page)).toContainText("记录 1 中文测试");
  await search(page).press("Enter");
  await expect
    .poll(() => page.evaluate(() => window.historyTest.pastes))
    .toEqual([1]);
});

test("scrolling stops following; reopening the same selected record resets scroll", async ({
  page,
}) => {
  await page.locator(".clip-list").hover();
  await page.mouse.wheel(0, 700);
  await expect
    .poll(() => page.locator(".clip-list").evaluate((e) => e.scrollTop))
    .toBeGreaterThan(0);
  await add(page);
  await expect(page.locator(".clip").first()).toContainText("新内容 100");
  await expect(selected(page)).toContainText("记录 1 中文测试");
  await shown(page);
  await expect(selected(page)).toContainText("新内容 100");
  await page.locator(".clip-list").hover();
  await page.mouse.wheel(0, 700);
  await expect
    .poll(() => page.locator(".clip-list").evaluate((e) => e.scrollTop))
    .toBeGreaterThan(0);
  await shown(page);
  await expect
    .poll(() => page.locator(".clip-list").evaluate((e) => e.scrollTop))
    .toBe(0);
});

test("deletion selects the next neighbour, then previous, then empty", async ({
  page,
}) => {
  await page.evaluate(() => {
    window.historyTest.entries = window.historyTest.entries.slice(0, 3);
  });
  await shown(page);
  await search(page).press("ArrowDown");
  await search(page).press("Delete");
  await expect(selected(page)).toContainText("记录 3 中文测试");
  await search(page).press("Delete");
  await expect(selected(page)).toContainText("记录 1 中文测试");
  await search(page).press("Delete");
  await expect(selected(page)).toHaveCount(0);
  await expect(page.getByText("从第一次复制开始")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "粘贴", exact: true }),
  ).toBeDisabled();
});

test("refresh retains a selection on a loaded second page and pinning does not reset it", async ({
  page,
}) => {
  await page.getByRole("button", { name: "加载更多记录" }).click();
  await expect(page.locator(".clip")).toHaveCount(65);
  await page.locator(".clip").filter({ hasText: "记录 60 中文测试" }).click();
  await page.getByRole("button", { name: "收藏记录", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "取消收藏", exact: true }),
  ).toBeEnabled();
  await add(page);
  await expect(page.locator(".clip")).toHaveCount(66);
  await expect(selected(page)).toContainText("记录 60 中文测试");
});

test("stale search response cannot overwrite reopening or a newer capture", async ({
  page,
}) => {
  await page.evaluate(() => {
    window.historyTest.hold = true;
  });
  await search(page).fill("记录 4 中文测试");
  await expect
    .poll(() => page.evaluate(() => window.historyTest.pending.length))
    .toBe(1);
  await shown(page);
  await add(page);
  await expect(selected(page)).toContainText("新内容 100");
  await page.evaluate(async () => {
    window.historyTest.pending.shift()();
    await new Promise((resolve) =>
      requestAnimationFrame(() => requestAnimationFrame(resolve)),
    );
  });
  await expect(search(page)).toHaveValue("");
  await expect(selected(page)).toContainText("新内容 100");
});

test("repeated shortcut preserves selection, dialog and focus; failed paste preserves selection", async ({
  page,
}) => {
  await search(page).press("ArrowDown");
  await expect(selected(page)).toContainText("记录 2 中文测试");
  await page.getByRole("button", { name: "查看内容", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "内容预览" })).toBeVisible();
  await shown(page, false);
  await expect(page.getByRole("dialog", { name: "内容预览" })).toBeVisible();
  await expect(search(page)).not.toBeFocused();
  await page.keyboard.press("Escape");
  await search(page).focus();
  await page.evaluate(() => {
    window.historyTest.failPaste = true;
  });
  await search(page).press("Enter");
  await expect(page.getByRole("alert")).toContainText("目标窗口未接受粘贴");
  await expect(selected(page)).toContainText("记录 2 中文测试");
});

test("reopening never pastes the old selection while its list is still loading", async ({
  page,
}) => {
  await search(page).press("ArrowDown");
  await page.evaluate(() => {
    window.historyTest.hold = true;
  });
  await shown(page);
  await expect
    .poll(() => page.evaluate(() => window.historyTest.pending.length))
    .toBe(1);
  await expect(selected(page)).toHaveCount(0);
  await search(page).press("Enter");
  expect(await page.evaluate(() => window.historyTest.pastes)).toEqual([]);
  await page.evaluate(() => window.historyTest.pending.shift()());
  await expect(selected(page)).toContainText("记录 1 中文测试");
});

test("recopying an existing entry moves selection by recency, not by id", async ({
  page,
}) => {
  await add(page, 30);
  await expect(selected(page)).toContainText("记录 30 中文测试");
  await search(page).press("Control+Enter");
  await expect
    .poll(() => page.evaluate(() => window.historyTest.copies))
    .toEqual([30]);
});

test("a new capture cannot displace the selected last item of a page", async ({
  page,
}) => {
  await page.locator(".clip").filter({ hasText: "记录 50 中文测试" }).click();
  await add(page);
  await expect(page.locator(".clip").first()).toContainText("新内容 100");
  await expect(selected(page)).toContainText("记录 50 中文测试");
  await expect(page.locator(".clip")).toHaveCount(66);
});

test("Space toggles selected preview and restores list focus without pasting", async ({
  page,
}) => {
  await search(page).press("ArrowDown");
  const list = page.getByRole("main");
  const dialog = page.getByRole("dialog", { name: "内容预览" });
  await page.keyboard.down("Space");
  await expect(dialog).toBeVisible();
  await expect(dialog.locator(".preview-content")).toBeFocused();
  await expect(dialog.locator(".content-text")).toContainText(
    "记录 2 中文测试",
  );
  await page.keyboard.down("Space");
  await expect(dialog).toBeVisible();
  await page.keyboard.up("Space");
  await page.keyboard.press("Space");
  await expect(dialog).not.toBeVisible();
  await expect(list).toBeFocused();
  await expect(selected(page)).toContainText("记录 2 中文测试");
  await selected(page).click();
  await page.keyboard.press("Space");
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Space");
  await expect(dialog).not.toBeVisible();
  expect(await page.evaluate(() => window.historyTest.pastes)).toEqual([]);
});

test("Space keeps text input and ignores modified or composing shortcuts", async ({
  page,
}) => {
  await search(page).press("Space");
  await expect(search(page)).toHaveValue(" ");
  await search(page).fill("");
  await search(page).press("ArrowDown");
  await page.keyboard.press("Control+Space");
  await page
    .getByRole("main")
    .dispatchEvent("keydown", { key: " ", isComposing: true });
  await expect(
    page.getByRole("dialog", { name: "内容预览" }),
  ).not.toBeVisible();
});
