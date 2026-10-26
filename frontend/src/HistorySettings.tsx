import { useCallback, useEffect, useRef, useState } from "react";
import {
  api,
  listen,
  type HistorySettings as HistorySettingsData,
} from "./api";
import { useI18n } from "./i18n";

function formatSpace(bytes: number, language: string) {
  const unit = bytes >= 1e9 ? "GB" : bytes >= 1e6 ? "MB" : "KB";
  const divisor = unit === "GB" ? 1e9 : unit === "MB" ? 1e6 : 1e3;
  return `${(bytes / divisor).toLocaleString(language, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })} ${unit}`;
}

export function HistorySettings({ disabled }: { disabled: boolean }) {
  const { t, language, translateError } = useI18n();
  const [data, setData] = useState<HistorySettingsData | null>(null);
  const [confirmation, setConfirmation] = useState<{
    days: number;
    count: number;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const active = useRef(false);
  const pending = useRef(false);
  const revision = useRef(0);
  const selectRef = useRef<HTMLSelectElement>(null);
  const cancelRef = useRef<HTMLButtonElement>(null);
  const hadConfirmation = useRef(false);
  useEffect(() => {
    if (busy) return;
    if (confirmation) cancelRef.current?.focus();
    else if (hadConfirmation.current) selectRef.current?.focus();
    hadConfirmation.current = confirmation !== null;
  }, [confirmation, busy]);
  const refresh = useCallback(async () => {
    const request = ++revision.current;
    try {
      const value = await api("HistorySettings");
      if (active.current && request === revision.current) {
        setData(value);
        setError("");
      }
    } catch (cause) {
      if (active.current && request === revision.current)
        setError(cause instanceof Error ? cause.message : String(cause));
    }
  }, []);

  useEffect(() => {
    active.current = true;
    let disposed = false;
    let off: (() => void) | undefined;
    void listen("history:changed", () => void refresh())
      .then((unsubscribe) => {
        if (disposed) unsubscribe();
        else {
          off = unsubscribe;
          void refresh();
        }
      })
      .catch((cause) => {
        if (!disposed) setError(String(cause));
      });
    void refresh();
    return () => {
      disposed = true;
      active.current = false;
      revision.current++;
      off?.();
    };
  }, [refresh]);

  async function change(days: number, confirmed = false) {
    if (pending.current || !data) return;
    pending.current = true;
    setBusy(true);
    setError("");
    try {
      if (!confirmed && days < data.retentionDays) {
        const count = await api("ExpiredCount", days);
        if (!active.current) return;
        if (count > 0) {
          setConfirmation({ days, count });
          return;
        }
      }
      await api("SetRetentionDays", days);
      if (active.current) {
        setConfirmation(null);
        await refresh();
      }
    } catch (cause) {
      if (active.current)
        setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }

  return (
    <div className="history-settings">
      <div className="setting-row">
        <div>
          <label htmlFor="retention-select">
            <strong>{t("保留期限")}</strong>
          </label>
          <p>{t("收藏不自动清理")}</p>
        </div>
        <select
          ref={selectRef}
          id="retention-select"
          value={data?.retentionDays ?? 30}
          disabled={disabled || busy || !data || confirmation !== null}
          onChange={(event) => void change(Number(event.target.value))}
          onKeyDown={(event) => {
            if (
              (event.key !== "Home" && event.key !== "End") ||
              event.currentTarget.matches(":open")
            )
              return;
            event.preventDefault();
            void change(event.key === "Home" ? 7 : 90);
          }}
        >
          {[7, 30, 90].map((days) => (
            <option key={days} value={days}>
              {t("{days} 天", { days })}
            </option>
          ))}
        </select>
      </div>
      {confirmation && (
        <div className="retention-confirmation">
          <p>
            {t("将删除 {count} 条过期记录，收藏不受影响。", {
              count: confirmation.count,
            })}
          </p>
          <div>
            <button
              ref={cancelRef}
              className="secondary"
              disabled={busy || disabled}
              onClick={() => setConfirmation(null)}
            >
              {t("取消")}
            </button>
            <button
              className="danger"
              disabled={busy || disabled}
              onClick={() => void change(confirmation.days, true)}
            >
              {t("修改并清理")}
            </button>
          </div>
        </div>
      )}
      <div className="setting-row">
        <strong>{t("已用空间")}</strong>
        <span className="history-space" aria-live="polite">
          {data ? formatSpace(data.usedBytes, language) : t("读取中")}
        </span>
      </div>
      {error && (
        <p className="setting-error" role="alert">
          {translateError(error)}{" "}
          <button
            className="secondary"
            disabled={busy || disabled}
            onClick={() => void refresh()}
          >
            {t("重试")}
          </button>
        </p>
      )}
    </div>
  );
}
