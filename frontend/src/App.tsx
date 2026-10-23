import { useI18n } from "./i18n";
import { createPortal } from "react-dom";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import {
  Check,
  Clipboard,
  ClipboardPaste,
  Copy,
  FileText,
  Image as ImageIcon,
  Link2,
  ScanText,
  Search,
  Settings,
  ShieldCheck,
  Star,
  Trash2,
  X,
} from "lucide-react";
import { api, imageURL, listen, preview, type Entry, type Status } from "./api";
import { useHistory } from "./useHistory";
import { EntryMenu } from "./EntryMenu";
import buildInfo from "../../build/windows/info.json";
import "./style.css";
import "./liquid-glass.css";

const time = (value: number, language: string) =>
  new Date(value).toLocaleTimeString(language, {
    hour: "2-digit",
    minute: "2-digit",
  });
const isURL = (text: string) => /^https?:\/\/\S+$/.test(text.trim());
const errorText = (error: unknown) =>
  error instanceof Error ? error.message : String(error);

const imageInfo = (entry: Entry) =>
  `${entry.imageWidth} × ${entry.imageHeight} · ${entry.imageBytes < 1048576 ? `${Math.max(1, Math.round(entry.imageBytes / 1024))} KiB` : `${(entry.imageBytes / 1048576).toFixed(1)} MiB`}`;

function Picture({
  entry,
  thumbnail = false,
}: {
  entry: Entry;
  thumbnail?: boolean;
}) {
  const { t } = useI18n();
  const [failed, setFailed] = useState(false);
  const src = imageURL(entry, thumbnail);
  useEffect(() => setFailed(false), [src]);
  return failed ? (
    <span className="image-unavailable" role="status">
      {t("图片无法读取，请刷新或重新选择")}
    </span>
  ) : (
    <img
      className={thumbnail ? "clip-thumbnail" : "preview-image checkerboard"}
      src={src}
      alt={t("图片，{width} × {height} 像素", {
        width: entry.imageWidth,
        height: entry.imageHeight,
      })}
      loading={thumbnail ? "lazy" : "eager"}
      decoding="async"
      draggable={false}
      onError={() => setFailed(true)}
    />
  );
}

export default function App() {
  const { t, language, setLanguage, translateError } = useI18n();
  const [contextMenu, setContextMenu] = useState<{
    id: number;
    x: number;
    y: number;
    scrollTop: number;
  } | null>(null);
  const [entry, setEntry] = useState<Entry | null>(null);
  const [status, setStatus] = useState<Status | null>(null),
    [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ text: string; error: boolean } | null>(
      null,
    ),
    [settings, setSettings] = useState(false),
    [details, setDetails] = useState(false),
    [confirmClear, setConfirmClear] = useState(false);
  const [autoStart, setAutoStart] = useState<boolean | null>(null),
    [autoStartError, setAutoStartError] = useState("");
  const searchRef = useRef<HTMLInputElement>(null),
    listRef = useRef<HTMLElement>(null),
    dialogRef = useRef<HTMLDialogElement>(null),
    detailRef = useRef<HTMLDialogElement>(null),
    previewContentRef = useRef<HTMLDivElement>(null),
    noticeTimer = useRef<ReturnType<typeof setTimeout>>(),
    scrollTimer = useRef<ReturnType<typeof setTimeout>>();
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
  const {
    page,
    selected,
    query,
    pinned,
    loading,
    following,
    stopFollowing,
    refresh,
    reopen,
    more,
    setQuery,
    setPinned,
    setSelected,
  } = useHistory(fail);
  const selectedItem = page.entries.find((item) => item.id === selected);
  const menuEntry =
    contextMenu && selected === contextMenu.id
      ? page.entries.find((item) => item.id === contextMenu.id)
      : undefined;
  const closeMenu = useCallback((restoreFocus = false) => {
    setContextMenu(null);
    if (restoreFocus) searchRef.current?.focus({ preventScroll: true });
  }, []);
  useEffect(() => {
    closeMenu();
  }, [query, pinned, settings, details, closeMenu]);
  useEffect(() => {
    if (contextMenu && !menuEntry) closeMenu(true);
  }, [contextMenu, menuEntry, closeMenu]);
  const loadStatus = useCallback(
    () => api("Status").then(setStatus).catch(fail),
    [fail],
  );
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
        (reset) => {
          void loadStatus();
          if (reset !== true) return;
          setContextMenu(null);
          // Close the modal before focusing search; dialog.close restores focus.
          dialogRef.current?.close();
          detailRef.current?.close();
          setSettings(false);
          setDetails(false);
          setConfirmClear(false);
          setEntry(null);
          reopen();
          searchRef.current?.focus();
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
  }, [refresh, reopen, loadStatus, notify, fail]);
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
    if (!settings) return;
    let active = true;
    setAutoStart(null);
    setAutoStartError("");
    api("AutoStart")
      .then((value) => {
        if (active) setAutoStart(value);
      })
      .catch((error) => {
        if (active) setAutoStartError(errorText(error));
      });
    return () => {
      active = false;
    };
  }, [settings]);
  useLayoutEffect(() => {
    const d = detailRef.current;
    if (details && d && !d.open) {
      d.showModal();
      previewContentRef.current?.focus({ preventScroll: true });
    } else if (!details && d?.open) d.close();
  }, [details]);
  useLayoutEffect(() => {
    if (following.current) listRef.current?.scrollTo({ top: 0 });
  }, [page, following]);
  useEffect(() => {
    if (contextMenu) return;
    listRef.current
      ?.querySelector(".clip.selected")
      ?.scrollIntoView({ block: "nearest" });
  }, [selected]);
  const perform = useCallback(
    async (action: () => Promise<unknown>, message?: string) => {
      stopFollowing();
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
    [busy, notify, fail, stopFollowing],
  );
  const copy = useCallback(() => {
    if (selected !== null)
      void perform(
        () => api("Copy", selected),
        preview ? t("预览：已模拟复制") : t("已复制到剪贴板"),
      );
  }, [selected, perform, t]);
  const paste = useCallback(() => {
    if (selected !== null) void perform(() => api("Paste", selected));
  }, [selected, perform, t]);
  const deleteEntry = useCallback(
    (id: number) => {
      void perform(() => api("Delete", id), t("已删除记录"));
    },
    [perform, t],
  );
  const togglePin = useCallback(
    (item: Pick<Entry, "id" | "pinned">) => {
      void perform(() => api("Pin", item.id, !item.pinned));
    },
    [perform, t],
  );
  const deleteSelected = useCallback(() => {
    if (selected !== null) deleteEntry(selected);
  }, [selected, deleteEntry]);
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.isComposing || settings || contextMenu) return;
      if (e.key === " ") {
        const target = e.target;
        if (
          e.ctrlKey ||
          e.altKey ||
          e.shiftKey ||
          e.metaKey ||
          !(target instanceof HTMLElement) ||
          target.isContentEditable ||
          target.closest("input, textarea, select")
        )
          return;
        if (details) {
          e.preventDefault();
          if (!e.repeat) {
            detailRef.current?.close();
            setDetails(false);
            listRef.current?.focus({ preventScroll: true });
          }
        } else if (listRef.current?.contains(target) && selectedItem) {
          e.preventDefault();
          if (!e.repeat) setDetails(true);
        }
        return;
      }
      if (details) return;
      if (e.key === "Delete") {
        const target = e.target;
        const editing =
          target instanceof HTMLElement &&
          (target.isContentEditable ||
            target.closest("input, textarea, select"));
        // The panel focuses search on opening; an empty search has no text to delete.
        if (
          (editing &&
            !(target === searchRef.current && !searchRef.current.value)) ||
          e.ctrlKey ||
          e.altKey ||
          e.shiftKey ||
          e.metaKey
        )
          return;
        e.preventDefault();
        if (!e.repeat) deleteSelected();
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        if (e.repeat) return;
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
        if (page.entries.length > 0)
          listRef.current?.focus({ preventScroll: true });
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
  }, [
    contextMenu,
    settings,
    details,
    query,
    page,
    selected,
    selectedItem,
    copy,
    paste,
    deleteSelected,
    fail,
  ]);
  return (
    <div
      className="app-shell"
      onPointerDownCapture={stopFollowing}
      onWheelCapture={stopFollowing}
      onTouchMoveCapture={stopFollowing}
      onKeyDownCapture={(e) => {
        if (
          !["Control", "Shift", "Alt", "Meta"].includes(e.key) &&
          !(e.ctrlKey && e.shiftKey && e.key.toLowerCase() === "v")
        )
          stopFollowing();
      }}
    >
      <header className="titlebar">
        <div className="drag-handle" title={t("拖动 PasteWin 窗口")}>
          <Clipboard size={20} strokeWidth={1.6} />
        </div>
        <div className="search-box">
          <Search size={14} />
          <input
            ref={searchRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("搜索剪贴板…")}
            aria-label={t("搜索剪贴板")}
          />
          {query && (
            <button aria-label={t("清空搜索")} onClick={() => setQuery("")}>
              <X size={13} />
            </button>
          )}
        </div>
        <button
          className="icon-button"
          aria-label={t("设置")}
          title={t("设置")}
          onClick={() => setSettings(true)}
        >
          <Settings size={16} />
        </button>
        <button
          className="icon-button close-button"
          aria-label={t("隐藏到托盘")}
          title={t("隐藏到托盘 · Esc")}
          onClick={() => void api("Hide").catch(fail)}
        >
          <X size={16} />
        </button>
      </header>
      <nav className="tabs" aria-label={t("历史分类")}>
        <div className={`tab-segment ${pinned ? "show-pinned" : ""}`}>
          <button
            className={!pinned ? "active" : ""}
            aria-pressed={!pinned}
            onClick={() => setPinned(false)}
          >
            {t("全部")} <span>{page.all}</span>
          </button>
          <button
            className={pinned ? "active" : ""}
            aria-pressed={pinned}
            onClick={() => setPinned(true)}
          >
            <Star size={12} />
            {t("收藏")} <span>{page.pinned}</span>
          </button>
        </div>
        <button
          className={`capture-toggle ${status?.paused ? "paused" : ""}`}
          disabled={busy || !status}
          aria-label={status?.paused ? t("恢复记录") : t("暂停记录")}
          title={
            status?.paused ? t("已暂停，点击恢复") : t("正在记录，点击暂停")
          }
          onClick={() => void perform(() => api("SetPaused", !status?.paused))}
        >
          <span className="status-dot" />
          {status?.paused ? t("已暂停") : t("记录中")}
        </button>
      </nav>
      {status?.hotkeyError && (
        <div className="warning" role="status">
          {translateError(status.hotkeyError)}
        </div>
      )}
      {query && (
        <div className="search-summary" role="status">
          {loading
            ? t("正在读取历史…")
            : t("{count} 条匹配", {
                count: page.total.toLocaleString(language),
              })}
        </div>
      )}
      <main
        ref={listRef}
        tabIndex={-1}
        className="clip-list"
        aria-label={t("剪贴板历史")}
        aria-busy={loading}
        onScroll={() => {
          // A queued scroll event from revealing the card can arrive after opening.
          if (
            contextMenu &&
            listRef.current?.scrollTop !== contextMenu.scrollTop
          )
            closeMenu();

          // macOS-style scrollbar: show on scroll, hide after idle
          const list = listRef.current;
          if (!list) return;

          list.classList.add("scrolling");
          clearTimeout(scrollTimer.current);
          scrollTimer.current = setTimeout(() => {
            list.classList.remove("scrolling");
          }, 1000);
        }}
      >
        {loading ? (
          <div className="empty-list">{t("正在读取历史…")}</div>
        ) : page.entries.length === 0 ? (
          <div className="empty-list">
            <Clipboard size={28} strokeWidth={1.3} />
            <strong>
              {query
                ? t("没有找到相关内容")
                : pinned
                  ? t("还没有收藏")
                  : t("从第一次复制开始")}
            </strong>
            <p>
              {query
                ? t("换个关键词再试试")
                : pinned
                  ? t("选中一条记录，点击下方星标。")
                  : t("复制文字或图片后，就能在这里随时找回。")}
            </p>
            {!query && !pinned && <kbd>Ctrl + Shift + V</kbd>}
          </div>
        ) : (
          page.entries.map((item) => (
            <button
              key={item.id}
              className={`clip ${item.hasImage ? "has-image" : ""} ${selected === item.id ? "selected" : ""}`}
              aria-pressed={selected === item.id}
              aria-haspopup="menu"
              onContextMenu={(e) => {
                e.preventDefault();
                if (busy) return;
                setSelected(item.id);
                const rect = e.currentTarget.getBoundingClientRect();
                setContextMenu({
                  id: item.id,
                  scrollTop: listRef.current?.scrollTop ?? 0,
                  x: e.clientX || rect.left + 12,
                  y: e.clientY || rect.top + 24,
                });
              }}
              onKeyDown={(e) => {
                if (e.nativeEvent.isComposing || busy) return;
                if (
                  e.key === "ContextMenu" ||
                  (e.shiftKey && e.key === "F10")
                ) {
                  e.preventDefault();
                  e.stopPropagation();
                  setSelected(item.id);
                  const rect = e.currentTarget.getBoundingClientRect();
                  setContextMenu({
                    id: item.id,
                    scrollTop: listRef.current?.scrollTop ?? 0,
                    x: rect.left + 12,
                    y: rect.top + 24,
                  });
                }
              }}
              onClick={() => setSelected(item.id)}
              onDoubleClick={() => void perform(() => api("Paste", item.id))}
              title={t("单击选择，双击粘贴，右键收藏或删除")}
            >
              <span className="clip-meta">
                <span className="clip-kind">
                  {item.hasImage ? (
                    <ImageIcon size={22} strokeWidth={1.5} />
                  ) : isURL(item.preview) ? (
                    <Link2 size={22} strokeWidth={1.5} />
                  ) : (
                    <FileText size={22} strokeWidth={1.5} />
                  )}
                  {item.pinned && (
                    <Star size={11} className="starred" fill="currentColor" />
                  )}
                </span>
                <time>{time(item.updatedAt, language)}</time>
              </span>
              {item.hasImage && (
                <>
                  <Picture entry={item} thumbnail />
                  <span className="image-info">
                    {imageInfo(item)}
                    {item.characters > 0 ? t(" · 含文字") : ""}
                  </span>
                </>
              )}
              {item.characters > 0 && (
                <span
                  className={`clip-text ${isURL(item.preview) ? "link-text" : ""}`}
                >
                  {item.preview}
                </span>
              )}
            </button>
          ))
        )}
        {page.entries.length < page.total && (
          <button
            className="load-more"
            disabled={busy}
            onClick={() => void perform(more)}
          >
            {t("加载更多记录")}
          </button>
        )}
      </main>
      {contextMenu && menuEntry && !settings && !details && (
        <EntryMenu
          key={contextMenu.id}
          x={contextMenu.x}
          y={contextMenu.y}
          pinned={menuEntry.pinned}
          busy={busy}
          onClose={closeMenu}
          onPin={() => {
            closeMenu(true);
            togglePin(menuEntry);
          }}
          onDelete={() => {
            closeMenu(true);
            deleteEntry(menuEntry.id);
          }}
        />
      )}
      <div
        className="selection-actions"
        role="toolbar"
        aria-label={t("所选记录操作")}
      >
        <button
          className="icon-button"
          aria-label={t("查看内容")}
          title={t("查看内容")}
          disabled={!selectedItem}
          onClick={() => setDetails(true)}
        >
          <ScanText size={16} />
        </button>
        <button
          className={`icon-button ${selectedItem?.pinned ? "starred" : ""}`}
          aria-label={selectedItem?.pinned ? t("取消收藏") : t("收藏记录")}
          title={selectedItem?.pinned ? t("取消收藏") : t("收藏记录")}
          disabled={busy || !selectedItem}
          onClick={() => selectedItem && togglePin(selectedItem)}
        >
          <Star
            size={15}
            fill={selectedItem?.pinned ? "currentColor" : "none"}
          />
        </button>
        <button
          className="icon-button"
          aria-label={t("复制")}
          title={t("复制 · Ctrl+Enter")}
          disabled={busy || !selectedItem}
          onClick={copy}
        >
          <Copy size={15} />
        </button>
        <button
          className="icon-button delete-button"
          aria-label={t("删除记录")}
          title={t("删除记录 · Delete")}
          aria-keyshortcuts="Delete"
          disabled={busy || !selectedItem}
          onClick={deleteSelected}
        >
          <Trash2 size={15} />
        </button>
        <span className="action-spacer" />
        <button
          className="paste-action"
          disabled={busy || !selectedItem || !status?.canPaste}
          onClick={paste}
          title={
            status?.canPaste
              ? t("粘贴到原应用 · Enter")
              : t("在目标应用中按 Ctrl+Shift+V 唤起后粘贴")
          }
        >
          <ClipboardPaste size={14} />
          {t("粘贴")}
        </button>
      </div>

      <dialog
        ref={detailRef}
        className="preview-dialog"
        onKeyDown={(e) => {
          if (e.key !== "Escape") return;
          e.preventDefault();
          e.stopPropagation();
          if (!e.repeat && !e.nativeEvent.isComposing) setDetails(false);
        }}
        onCancel={(e) => {
          e.preventDefault();
          setDetails(false);
        }}
        onClose={() => setDetails(false)}
        aria-labelledby="preview-title"
      >
        <div className="dialog-heading">
          <h2 id="preview-title">{t("内容预览")}</h2>
          <button
            className="icon-button"
            aria-label={t("关闭预览")}
            onClick={() => setDetails(false)}
          >
            <X size={18} />
          </button>
        </div>
        <div className="preview-meta">
          {entry
            ? `${entry.hasImage ? imageInfo(entry) : t("{count} 字符", { count: entry.characters.toLocaleString(language) })} · ${time(entry.updatedAt, language)}`
            : t("正在读取…")}
        </div>
        <div className="preview-content" ref={previewContentRef} tabIndex={-1}>
          {details && !entry && <p role="status">{t("读取中")}</p>}
          {details && entry?.hasImage && <Picture entry={entry} />}
          {entry && entry.characters > 0 && (
            <pre className="content-text" tabIndex={0}>
              {entry?.text}
            </pre>
          )}
        </div>
        <div className="preview-actions">
          <button
            className="secondary"
            disabled={busy || !entry}
            onClick={copy}
          >
            <Copy size={14} />
            {entry?.hasImage && entry.characters > 0
              ? t("复制文字")
              : t("复制")}
          </button>
          {entry?.hasImage && entry.characters > 0 && (
            <button
              className="secondary"
              disabled={busy}
              onClick={() =>
                void perform(
                  () => api("CopyImage", entry.id),
                  preview ? t("预览：已模拟复制图片") : t("已复制图片"),
                )
              }
            >
              <Copy size={14} />
              {t("复制图片")}
            </button>
          )}
          <button
            className="primary"
            disabled={busy || !entry || !status?.canPaste}
            onClick={() => {
              setDetails(false);
              paste();
            }}
          >
            <ClipboardPaste size={14} />
            {entry?.hasImage && entry.characters > 0
              ? t("粘贴文字")
              : t("粘贴")}
          </button>
          {entry?.hasImage && entry.characters > 0 && (
            <button
              className="primary"
              disabled={busy || !status?.canPaste}
              onClick={() => {
                setDetails(false);
                void perform(() => api("PasteImage", entry.id));
              }}
            >
              <ClipboardPaste size={14} />
              {t("粘贴图片")}
            </button>
          )}
        </div>
      </dialog>

      {notice &&
        createPortal(
          <div
            className={`toast ${notice.error ? "error" : ""}`}
            role={notice.error ? "alert" : "status"}
          >
            {notice.error ? <X size={17} /> : <Check size={17} />}
            <span>
              {notice.error ? translateError(notice.text) : notice.text}
            </span>
            <button aria-label={t("关闭提示")} onClick={() => setNotice(null)}>
              <X size={15} />
            </button>
          </div>,
          (details ? detailRef.current : settings ? dialogRef.current : null) ??
            document.body,
        )}
      <dialog
        ref={dialogRef}
        className="settings-dialog"
        onKeyDown={(e) => {
          if (e.key !== "Escape") return;
          // Let an open language picker consume Escape before its dialog.
          if (
            e.target instanceof HTMLSelectElement &&
            e.target.matches(":open")
          )
            return;
          e.preventDefault();
          e.stopPropagation();
          if (!e.repeat && !e.nativeEvent.isComposing) setSettings(false);
        }}
        onCancel={(e) => {
          e.preventDefault();
          setSettings(false);
        }}
        onClose={() => {
          setSettings(false);
          setConfirmClear(false);
        }}
        aria-labelledby="settings-title"
      >
        <div className="dialog-heading">
          <h2 id="settings-title">{t("设置")}</h2>
          <button aria-label={t("关闭设置")} onClick={() => setSettings(false)}>
            <X size={20} />
          </button>
        </div>
        <div className="setting-row">
          <div>
            <label htmlFor="language-select">
              <strong>{t("语言")}</strong>
            </label>
          </div>
          <select
            id="language-select"
            value={language}
            disabled={busy}
            onKeyDown={(event) => {
              // Customizable select pickers don't apply Home/End while closed.
              // Preserve the native select shortcut used by keyboard users.
              if (
                (event.key !== "Home" && event.key !== "End") ||
                event.currentTarget.matches(":open")
              )
                return;
              event.preventDefault();
              const value = event.key === "Home" ? "zh-CN" : "en";
              void perform(async () => {
                await setLanguage(value);
                setNotice(null);
              });
            }}
            onChange={(event) => {
              const value = event.target.value;
              void perform(async () => {
                await setLanguage(value);
                setNotice(null);
              });
            }}
          >
            <option value="zh-CN">简体中文</option>
            <option value="en">English</option>
          </select>
        </div>
        <div className="setting-row">
          <div>
            <strong id="capture-label">{t("自动记录")}</strong>
          </div>
          <button
            className="startup-switch"
            role="switch"
            aria-checked={status != null && !status.paused}
            aria-labelledby="capture-label"
            disabled={busy || !status}
            onClick={() =>
              void perform(() => api("SetPaused", !status?.paused))
            }
          >
            <span className="switch-track" aria-hidden="true">
              <span />
            </span>
          </button>
        </div>
        <div className="setting-row">
          <div>
            <strong>{t("打开面板")}</strong>
          </div>
          <kbd>Ctrl + Shift + V</kbd>
        </div>
        <div className="setting-row">
          <div>
            <strong id="autostart-label">{t("开机启动")}</strong>
            {autoStartError && (
              <p className="setting-error" role="alert">
                {t("{error}。请关闭设置后重试。", {
                  error: translateError(autoStartError),
                })}
              </p>
            )}
          </div>
          <button
            className="startup-switch"
            role="switch"
            aria-checked={autoStart === true}
            aria-labelledby="autostart-label"
            disabled={busy || autoStart === null}
            onClick={() =>
              void perform(async () => {
                const enabled = await api("SetAutoStart", !autoStart);
                setAutoStart(enabled);
                notify(
                  preview
                    ? t("仅供预览，未更改开机启动。")
                    : enabled
                      ? t("已开启开机启动")
                      : t("已关闭开机启动"),
                );
              })
            }
          >
            <span className="switch-track" aria-hidden="true">
              <span />
            </span>
          </button>
        </div>
        <div className="storage-note">
          <ShieldCheck size={20} />
          <div>
            <strong>{t("本机保存")}</strong>
            <p>{t("最多保存500条，收藏不自动删除。")}</p>
            <p>{t("内容未加密，复制隐私信息前请暂停。")}</p>
            <code>
              {status?.databasePath &&
                (preview ? t("界面预览") : status.databasePath)}
            </code>
          </div>
        </div>
        <section className="about-section" aria-labelledby="about-title">
          <div className="about-heading">
            <h3 id="about-title">{t("关于")}</h3>
            <span className="app-version">
              {t("版本 {version}", { version: buildInfo.fixed.file_version })}
            </span>
          </div>
          <p>{t("保存复制过的文字和图片。")}</p>
        </section>
        <div className={`dialog-footer ${confirmClear ? "confirm-clear" : ""}`}>
          {confirmClear ? (
            <>
              <span>{t("删除所有未收藏记录？无法恢复。")}</span>
              <button
                className="secondary"
                disabled={busy}
                onClick={() => setConfirmClear(false)}
              >
                {t("取消")}
              </button>
              <button
                className="danger"
                disabled={busy}
                onClick={() =>
                  void perform(async () => {
                    await api("ClearUnpinned");
                    setConfirmClear(false);
                  }, t("已删除未收藏记录"))
                }
              >
                {t("确认删除")}
              </button>
            </>
          ) : (
            <>
              <button
                className="danger-text"
                disabled={busy}
                onClick={() => setConfirmClear(true)}
              >
                <Trash2 size={15} />
                {t("清理记录")}
              </button>
              <button
                className="secondary"
                onClick={() => void api("Quit").catch(fail)}
              >
                {t("退出应用")}
              </button>
            </>
          )}
        </div>
      </dialog>
    </div>
  );
}
