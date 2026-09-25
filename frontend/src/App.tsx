import { createPortal } from "react-dom";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Check,
  Clipboard,
  ClipboardPaste,
  Copy,
  CornerDownLeft,
  FileText,
  Link2,
  PanelsTopLeft,
  ScanText,
  Search,
  Settings,
  ShieldCheck,
  Star,
  Trash2,
  X,
} from "lucide-react";
import {
  api,
  listen,
  preview,
  type Entry,
  type Page,
  type Status,
} from "./api";
import buildInfo from "../../build/windows/info.json";
import "./style.css";

const empty: Page = { entries: [], total: 0, all: 0, pinned: 0 };
const time = (value: number) =>
  new Date(value).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
const isURL = (text: string) => /^https?:\/\/\S+$/.test(text.trim());
const errorText = (error: unknown) =>
  error instanceof Error ? error.message : String(error);

export default function App() {
  const [page, setPage] = useState<Page>(empty),
    [query, setQuery] = useState(""),
    [pinned, setPinned] = useState(false);
  const [selected, setSelected] = useState<number | null>(null),
    [entry, setEntry] = useState<Entry | null>(null);
  const [status, setStatus] = useState<Status | null>(null),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ text: string; error: boolean } | null>(
      null,
    ),
    [settings, setSettings] = useState(false),
    [details, setDetails] = useState(false),
    [confirmClear, setConfirmClear] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null),
    dialogRef = useRef<HTMLDialogElement>(null),
    detailRef = useRef<HTMLDialogElement>(null),
    request = useRef(0),
    noticeTimer = useRef<ReturnType<typeof setTimeout>>();
  const notify = useCallback((text: string, error = false) => {
    setNotice({ text, error });
    clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(
      () => setNotice(null),
      error ? 8000 : 2600,
    );
  }, []);
  const fail = useCallback(
    (error: unknown) => notify(errorText(error), true),
    [notify],
  );
  const loadStatus = useCallback(
    () => api("Status").then(setStatus).catch(fail),
    [fail],
  );
  const refresh = useCallback(async () => {
    const current = ++request.current;
    try {
      const next = await api("List", query, pinned, 0);
      if (current !== request.current) return;
      setPage(next);
      setSelected((id) =>
        next.entries.some((e) => e.id === id)
          ? id
          : (next.entries[0]?.id ?? null),
      );
    } catch (e) {
      fail(e);
    } finally {
      if (current === request.current) setLoading(false);
    }
  }, [query, pinned, fail]);
  useEffect(() => {
    const timer = setTimeout(refresh, 120);
    return () => clearTimeout(timer);
  }, [refresh]);
  useEffect(() => {
    let active = true;
    setEntry(null);
    if (selected !== null)
      api("Get", selected)
        .then((e) => {
          if (active) setEntry(e);
        })
        .catch(fail);
    return () => {
      active = false;
    };
  }, [selected, page, fail]);
  useEffect(() => {
    let disposed = false;
    const cleanup: Array<() => void> = [];
    const events: [string, (data: unknown) => void][] = [
      ["history:changed", () => void refresh()],
      ["status:changed", () => void loadStatus()],
      [
        "panel:shown",
        () => {
          void refresh();
          void loadStatus();
          setTimeout(() => searchRef.current?.focus(), 0);
        },
      ],
      ["app:error", (data) => notify(String(data), true)],
    ];
    events.forEach(([name, fn]) =>
      listen(name, fn)
        .then((off) => {
          if (disposed) off();
          else cleanup.push(off);
        })
        .catch(fail),
    );
    return () => {
      disposed = true;
      cleanup.forEach((off) => off());
    };
  }, [refresh, loadStatus, notify, fail]);
  useEffect(() => {
    void loadStatus();
    searchRef.current?.focus();
    return () => clearTimeout(noticeTimer.current);
  }, [loadStatus]);
  useEffect(() => {
    const d = dialogRef.current;
    if (settings && !d?.open) d?.showModal();
    else if (!settings && d?.open) d.close();
  }, [settings]);
  useEffect(() => {
    const d = detailRef.current;
    if (details && !d?.open) d?.showModal();
    else if (!details && d?.open) d.close();
  }, [details]);
  useEffect(() => {
    document
      .querySelector(".clip.selected")
      ?.scrollIntoView({ block: "nearest" });
  }, [selected]);
  const perform = useCallback(
    async (action: () => Promise<unknown>, message?: string) => {
      if (busy) return;
      setBusy(true);
      try {
        await action();
        if (message) notify(message);
      } catch (e) {
        fail(e);
      } finally {
        setBusy(false);
      }
    },
    [busy, notify, fail],
  );
  const copy = useCallback(() => {
    if (selected !== null)
      void perform(
        () => api("Copy", selected),
        preview ? "预览：已模拟复制" : "已复制到剪贴板",
      );
  }, [selected, perform]);
  const paste = useCallback(() => {
    if (selected !== null) void perform(() => api("Paste", selected));
  }, [selected, perform]);
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.isComposing || settings || details) return;
      if (e.key === "Escape") {
        e.preventDefault();
        if (query) setQuery("");
        else void api("Hide").catch(fail);
      }
      if (e.ctrlKey && e.key.toLowerCase() === "f") {
        e.preventDefault();
        searchRef.current?.focus();
      }
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const index = page.entries.findIndex((x) => x.id === selected);
        const next = Math.max(
          0,
          Math.min(
            page.entries.length - 1,
            index + (e.key === "ArrowDown" ? 1 : -1),
          ),
        );
        setSelected(page.entries[next]?.id ?? null);
      }
      if (e.key === "Enter") {
        const target = e.target as HTMLElement;
        if (target.tagName === "BUTTON") return;
        e.preventDefault();
        if (e.ctrlKey) copy();
        else paste();
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, [settings, details, query, page, selected, copy, paste, fail]);
  async function more() {
    const current = request.current;
    const result = await api("List", query, pinned, page.entries.length);
    if (current === request.current)
      setPage((p) => ({
        ...result,
        entries: [...p.entries, ...result.entries],
      }));
  }

  return (
    <div className="app-shell">
      <header className="titlebar">
        <div className="drag-handle" title="拖动 PasteWin 窗口">
          <PanelsTopLeft size={18} />
        </div>
        <div className="search-box">
          <Search size={14} />
          <input
            ref={searchRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索剪贴板…"
            aria-label="搜索剪贴板"
          />
          {query && (
            <button aria-label="清空搜索" onClick={() => setQuery("")}>
              <X size={13} />
            </button>
          )}
        </div>
        <button
          className="icon-button"
          aria-label="设置"
          title="设置"
          onClick={() => setSettings(true)}
        >
          <Settings size={16} />
        </button>
        <button
          className="icon-button close-button"
          aria-label="隐藏到托盘"
          title="隐藏到托盘 · Esc"
          onClick={() => void api("Hide").catch(fail)}
        >
          <X size={16} />
        </button>
      </header>
      <nav className="tabs" aria-label="历史分类">
        <button
          className={!pinned ? "active" : ""}
          aria-pressed={!pinned}
          onClick={() => setPinned(false)}
        >
          全部 <span>{page.all}</span>
        </button>
        <button
          className={pinned ? "active" : ""}
          aria-pressed={pinned}
          onClick={() => setPinned(true)}
        >
          <Star size={12} />
          收藏 <span>{page.pinned}</span>
        </button>
        <span className="panel-label">{preview ? "界面预览" : "PasteWin"}</span>
      </nav>
      {status?.hotkeyError && (
        <div className="warning" role="status">
          {status.hotkeyError}
        </div>
      )}
      <main className="clip-list" aria-label="剪贴板历史" aria-busy={loading}>
        {loading ? (
          <div className="empty-list">正在读取历史…</div>
        ) : page.entries.length === 0 ? (
          <div className="empty-list">
            <Clipboard size={28} strokeWidth={1.3} />
            <strong>
              {query
                ? "没有找到相关内容"
                : pinned
                  ? "还没有收藏"
                  : "从第一次复制开始"}
            </strong>
            <p>
              {query
                ? "换个关键词再试试"
                : pinned
                  ? "选中一条记录，点击下方星标。"
                  : "复制文字后，就能在这里随时找回。"}
            </p>
            {!query && !pinned && <kbd>Ctrl + Shift + V</kbd>}
          </div>
        ) : (
          page.entries.map((item) => (
            <button
              key={item.id}
              className={`clip ${selected === item.id ? "selected" : ""}`}
              aria-pressed={selected === item.id}
              onClick={() => setSelected(item.id)}
              onDoubleClick={() => void perform(() => api("Paste", item.id))}
              title="单击选择，双击粘贴到原应用"
            >
              <span className="clip-meta">
                <span className="clip-kind">
                  {isURL(item.preview) ? (
                    <Link2 size={11} />
                  ) : (
                    <FileText size={11} />
                  )}
                  {item.pinned && (
                    <Star size={11} className="starred" fill="currentColor" />
                  )}
                </span>
                <time>{time(item.updatedAt)}</time>
              </span>
              <span
                className={`clip-text ${isURL(item.preview) ? "link-text" : ""}`}
              >
                {item.preview}
              </span>
            </button>
          ))
        )}
        {page.entries.length < page.total && (
          <button
            className="load-more"
            disabled={busy}
            onClick={() => void perform(more)}
          >
            加载更多记录
          </button>
        )}
      </main>
      <div
        className="selection-actions"
        role="toolbar"
        aria-label="所选记录操作"
      >
        <button
          className="icon-button"
          aria-label="查看全文"
          title="查看全文"
          disabled={!entry}
          onClick={() => setDetails(true)}
        >
          <ScanText size={16} />
        </button>
        <button
          className={`icon-button ${entry?.pinned ? "starred" : ""}`}
          aria-label={entry?.pinned ? "取消收藏" : "收藏记录"}
          title={entry?.pinned ? "取消收藏" : "收藏记录"}
          disabled={busy || !entry}
          onClick={() =>
            entry && void perform(() => api("Pin", entry.id, !entry.pinned))
          }
        >
          <Star size={15} fill={entry?.pinned ? "currentColor" : "none"} />
        </button>
        <button
          className="icon-button delete-button"
          aria-label="删除记录"
          title="删除记录"
          disabled={busy || !entry}
          onClick={() =>
            entry && void perform(() => api("Delete", entry.id), "已删除记录")
          }
        >
          <Trash2 size={15} />
        </button>
        <span className="action-spacer" />
        <button
          className="copy-action"
          disabled={busy || !entry}
          onClick={copy}
          title="复制 · Ctrl+Enter"
        >
          <Copy size={14} />
          复制
        </button>
        <button
          className="paste-action"
          disabled={busy || !entry || !status?.canPaste}
          onClick={paste}
          title={
            status?.canPaste
              ? "粘贴到原应用 · Enter"
              : "在目标应用中按 Ctrl+Shift+V 唤起后粘贴"
          }
        >
          <ClipboardPaste size={14} />
          粘贴
          <CornerDownLeft size={12} />
        </button>
      </div>
      <footer className="footer">
        <button
          className={`capture-toggle ${status?.paused ? "paused" : ""}`}
          disabled={busy || !status}
          aria-label={status?.paused ? "恢复记录" : "暂停记录"}
          title={status?.paused ? "已暂停，点击恢复" : "正在记录，点击暂停"}
          onClick={() => void perform(() => api("SetPaused", !status?.paused))}
        >
          <span className="status-dot" />
          {status?.paused ? "已暂停" : "记录中"}
        </button>
        <span aria-live="polite">
          {page.total} 条{query ? "匹配" : "记录"}
        </span>
        <span
          className="shortcut-hint"
          title="方向键选择，Enter 粘贴，Esc 隐藏"
        >
          ↑ ↓ <span>选择</span>
        </span>
      </footer>
      <dialog
        ref={detailRef}
        className="preview-dialog"
        onCancel={() => setDetails(false)}
        onClose={() => setDetails(false)}
        aria-labelledby="preview-title"
      >
        <div className="dialog-heading">
          <h2 id="preview-title">内容预览</h2>
          <button
            className="icon-button"
            aria-label="关闭预览"
            onClick={() => setDetails(false)}
          >
            <X size={18} />
          </button>
        </div>
        <div className="preview-meta">
          {entry
            ? `${entry.characters.toLocaleString()} 字符 · ${time(entry.updatedAt)}`
            : "正在读取…"}
        </div>
        <pre className="content-text" tabIndex={0}>
          {entry?.text}
        </pre>
        <div className="preview-actions">
          <button
            className="secondary"
            disabled={busy || !entry}
            onClick={copy}
          >
            <Copy size={14} />
            复制
          </button>
          <button
            className="primary"
            disabled={busy || !entry || !status?.canPaste}
            onClick={() => {
              setDetails(false);
              paste();
            }}
          >
            <ClipboardPaste size={14} />
            粘贴到原应用
          </button>
        </div>
      </dialog>

      {notice &&
        createPortal(
          <div
            className={`toast ${notice.error ? "error" : ""}`}
            role={notice.error ? "alert" : "status"}
          >
            {notice.error ? <X size={17} /> : <Check size={17} />}
            <span>{notice.text}</span>
            <button aria-label="关闭提示" onClick={() => setNotice(null)}>
              <X size={15} />
            </button>
          </div>,
          (details ? detailRef.current : settings ? dialogRef.current : null) ??
            document.body,
        )}
      <dialog
        ref={dialogRef}
        onCancel={() => setSettings(false)}
        onClose={() => {
          setSettings(false);
          setConfirmClear(false);
        }}
        aria-labelledby="settings-title"
      >
        <div className="dialog-heading">
          <h2 id="settings-title">设置与存储</h2>
          <button aria-label="关闭设置" onClick={() => setSettings(false)}>
            <X size={20} />
          </button>
        </div>
        <div className="setting-row">
          <div>
            <strong>记录剪贴板</strong>
            <p>只记录程序运行期间新复制的文本。</p>
          </div>
          <button
            className="secondary"
            disabled={busy}
            onClick={() =>
              void perform(() => api("SetPaused", !status?.paused))
            }
          >
            {status?.paused ? "恢复记录" : "暂停记录"}
          </button>
        </div>
        <div className="setting-row">
          <div>
            <strong>全局快捷键</strong>
            <p>在当前应用中唤起，选中后粘贴回原处。</p>
          </div>
          <kbd>Ctrl Shift V</kbd>
        </div>
        <div className="storage-note">
          <ShieldCheck size={20} />
          <div>
            <strong>所有历史都保存在本机</strong>
            <p>
              最多保留 500 条，每条不超过 1
              MB。收藏记录不会自动清理。原型使用本地明文数据库，请按需暂停记录敏感内容。
            </p>
            <code>{status?.databasePath}</code>
          </div>
        </div>
        <section className="about-section" aria-labelledby="about-title">
          <div className="about-heading">
            <h3 id="about-title">关于 PasteWin</h3>
            <span className="app-version">
              版本 {buildInfo.fixed.file_version}
            </span>
          </div>
          <p>Windows 本地剪贴板工具</p>
          <p>记录、搜索和收藏复制过的文本，快速粘贴回原应用。</p>
        </section>
        <div className="dialog-footer">
          {confirmClear ? (
            <>
              <span>清除未收藏的记录？</span>
              <button
                className="secondary"
                onClick={() => setConfirmClear(false)}
              >
                取消
              </button>
              <button
                className="danger"
                disabled={busy}
                onClick={() =>
                  void perform(async () => {
                    await api("ClearUnpinned");
                    setConfirmClear(false);
                  }, "已清除未收藏记录")
                }
              >
                确认清除
              </button>
            </>
          ) : (
            <>
              <button
                className="danger-text"
                onClick={() => setConfirmClear(true)}
              >
                <Trash2 size={15} />
                清除未收藏记录
              </button>
              <button
                className="secondary"
                onClick={() => void api("Quit").catch(fail)}
              >
                退出 PasteWin
              </button>
            </>
          )}
        </div>
      </dialog>
    </div>
  );
}
