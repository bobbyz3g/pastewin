import { useI18n } from "./i18n";
import { useEffect, useLayoutEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { Star, Trash2 } from "lucide-react";

export function EntryMenu({
  x,
  y,
  pinned,
  busy,
  onClose,
  onPin,
  onDelete,
}: {
  x: number;
  y: number;
  pinned: boolean;
  busy: boolean;
  onClose: (restoreFocus?: boolean) => void;
  onPin: () => void;
  onDelete: () => void;
}) {
  const { t } = useI18n();
  const menuRef = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const menu = menuRef.current!;
    const { width, height } = menu.getBoundingClientRect();
    menu.style.left = `${Math.max(8, Math.min(x, window.innerWidth - width - 8))}px`;
    menu.style.top = `${Math.max(8, Math.min(y, window.innerHeight - height - 8))}px`;
    menu
      .querySelector<HTMLButtonElement>("button:not(:disabled)")
      ?.focus({ preventScroll: true });
  }, [x, y]);
  useEffect(() => {
    const outside = (e: PointerEvent) => {
      if (!menuRef.current?.contains(e.target as Node)) onClose();
    };
    const dismiss = () => onClose();
    document.addEventListener("pointerdown", outside, true);
    window.addEventListener("wheel", dismiss, true);
    window.addEventListener("resize", dismiss);
    window.addEventListener("blur", dismiss);
    return () => {
      document.removeEventListener("pointerdown", outside, true);
      window.removeEventListener("wheel", dismiss, true);
      window.removeEventListener("resize", dismiss);
      window.removeEventListener("blur", dismiss);
    };
  }, [onClose]);

  return createPortal(
    <div
      ref={menuRef}
      className="entry-menu"
      role="menu"
      aria-label={t("记录操作")}
      onContextMenu={(e) => e.preventDefault()}
      onKeyDown={(e) => {
        // Menu keys must not also delete, navigate or paste in the history list.
        e.stopPropagation();
        if (e.nativeEvent.isComposing || e.ctrlKey || e.altKey || e.metaKey) {
          e.preventDefault();
          return;
        }
        if (e.key === "Escape" || e.key === "Tab") {
          e.preventDefault();
          onClose(true);
        } else if (["ArrowDown", "ArrowUp", "Home", "End"].includes(e.key)) {
          e.preventDefault();
          const items = Array.from(
            menuRef.current!.querySelectorAll<HTMLButtonElement>(
              "button:not(:disabled)",
            ),
          );
          const index = items.indexOf(
            document.activeElement as HTMLButtonElement,
          );
          const next =
            e.key === "Home"
              ? 0
              : e.key === "End"
                ? items.length - 1
                : (index + (e.key === "ArrowDown" ? 1 : -1) + items.length) %
                  items.length;
          items[next]?.focus();
        }
      }}
      onBlur={(e) => {
        if (
          e.relatedTarget &&
          !e.currentTarget.contains(e.relatedTarget as Node)
        )
          onClose();
      }}
    >
      <button role="menuitem" disabled={busy} onClick={onPin}>
        <Star size={14} fill={pinned ? "currentColor" : "none"} />
        {pinned ? t("取消收藏") : t("收藏")}
      </button>
      <button
        role="menuitem"
        className="entry-menu-delete"
        disabled={busy}
        onClick={onDelete}
      >
        <Trash2 size={14} />
        {t("删除")}
      </button>
    </div>,
    document.body,
  );
}
