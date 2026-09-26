// Explicit development-only browser fixture. Never reads the system clipboard.
import type { Entry, Status } from "./api";
const now = Date.now();
let entries: Entry[] = [
  "把灵感留住，把重复交给剪贴板。\n\nPasteWin 是一个安静的本地工作台。复制文字后，按 Ctrl + Shift + V，随时找回你需要的那一条。\n\n所有记录保存在本机，随时可以暂停、收藏或删除。",
  "https://wails.io/docs/introduction",
  "项目笔记\n✓ 完成文本历史与搜索\n✓ 支持全局快捷键\n○ 验证多屏缩放和输入法",
  'func main() {\n    fmt.Println("Hello, PasteWin")\n}',
  "设计是让每一次操作都更自然。",
  "周五迭代评审\n14:30 — 原型体验\n15:00 — 交互与性能反馈",
].map((text, i) => ({
  id: i + 1,
  text,
  preview: text.slice(0, 180),
  characters: [...text].length,
  createdAt: now - i * 170000,
  updatedAt: now - i * 170000,
  pinned: i === 0 || i === 2,
  hasImage: false,
  imageWidth: 0,
  imageHeight: 0,
  imageBytes: 0,
}));
const canvas = document.createElement("canvas");
canvas.width = 640;
canvas.height = 360;
const context = canvas.getContext("2d")!;
context.fillStyle = "#337fc4";
context.fillRect(40, 40, 400, 220);
context.fillStyle = "#a885cc99";
context.beginPath();
context.arc(460, 200, 110, 0, Math.PI * 2);
context.fill();
context.fillStyle = "white";
context.font = "32px sans-serif";
context.fillText("PasteWin 图片预览", 65, 150);
const sampleImage = canvas.toDataURL("image/png");
entries.unshift(
  ...["", "这条记录同时包含文字和图片，默认复制文字。"].map(
    (text, i): Entry => ({
      id: 7 + i,
      text,
      preview: text,
      characters: text.length,
      createdAt: now,
      updatedAt: now,
      pinned: false,
      hasImage: true,
      imageWidth: 640,
      imageHeight: 360,
      imageBytes: 8192,
      previewImage: sampleImage,
    }),
  ),
);
let paused = false;
let autoStart = false;
const changed = () => window.dispatchEvent(new CustomEvent("history:changed"));
export const demo = {
  async List(query: string, pinned: boolean, offset: number) {
    const filtered = entries.filter(
      (e) =>
        (!pinned || e.pinned) &&
        e.text.toLowerCase().includes(query.toLowerCase()),
    );
    return {
      entries: filtered.slice(offset, offset + 50),
      total: filtered.length,
      all: entries.length,
      pinned: entries.filter((e) => e.pinned).length,
    };
  },
  async Get(id: number) {
    const e = entries.find((e) => e.id === id);
    if (!e) throw Error("记录不存在");
    return e;
  },
  async Pin(id: number, pinned: boolean) {
    entries = entries.map((e) => (e.id === id ? { ...e, pinned } : e));
    changed();
  },
  async Delete(id: number) {
    entries = entries.filter((e) => e.id !== id);
    changed();
  },
  async ClearUnpinned() {
    entries = entries.filter((e) => e.pinned);
    changed();
  },
  async Copy() {},
  async CopyImage() {},
  async Paste() {
    throw Error("浏览器预览不执行系统粘贴");
  },
  async SetPaused(value: boolean) {
    paused = value;
    window.dispatchEvent(new CustomEvent("status:changed"));
  },
  async PasteImage() {
    throw Error("浏览器预览不执行系统粘贴");
  },
  async Status(): Promise<Status> {
    return {
      paused,
      hotkey: "Ctrl+Shift+V",
      hotkeyError: "",
      databasePath: "仅浏览器预览 · 示例数据",
      canPaste: false,
    };
  },
  async AutoStart() {
    return autoStart;
  },
  async SetAutoStart(enabled: boolean) {
    autoStart = enabled;
    return autoStart;
  },
  async Hide() {},
  async Quit() {},
};
