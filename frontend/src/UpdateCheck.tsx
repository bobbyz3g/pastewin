import { useEffect, useRef, useState } from "react";
import { api, type UpdateResult } from "./api";
import { useI18n } from "./i18n";

export function UpdateCheck({ version }: { version: string }) {
  const { t, translateError } = useI18n();
  const [result, setResult] = useState<UpdateResult | null>(null);
  const [checking, setChecking] = useState(false);
  const [opening, setOpening] = useState(false);
  const [error, setError] = useState("");
  const active = useRef(false);
  const pending = useRef(false);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);

  async function check() {
    if (pending.current) return;
    pending.current = true;
    setChecking(true);
    setError("");
    setResult(null);
    try {
      const next = await api("CheckForUpdates");
      if (active.current) setResult(next);
    } catch (cause) {
      if (active.current)
        setError(
          cause instanceof Error ? cause.message : "无法检查更新，请稍后重试",
        );
    } finally {
      pending.current = false;
      if (active.current) setChecking(false);
    }
  }

  async function openRelease() {
    if (!result || pending.current) return;
    pending.current = true;
    setOpening(true);
    setError("");
    try {
      await api("OpenUpdateRelease", result.tag);
    } catch {
      if (active.current) setError("无法打开发布页，请重试");
    } finally {
      pending.current = false;
      if (active.current) setOpening(false);
    }
  }

  const available = result?.status === "available";
  return (
    <div className="setting-row update-check-row" aria-live="polite">
      <div className="update-info">
        <strong className="app-version">
          {t(available ? "当前 {version}" : "版本 {version}", { version })}
        </strong>
        <div className={`update-status ${error ? "error" : ""}`} role="status">
          {error ? (
            translateError(error)
          ) : available ? (
            <>
              {t("新版")}{" "}
              <span className="update-version">{result.version}</span>
            </>
          ) : null}
        </div>
      </div>
      <button
        className={`${available && !error ? "primary" : "secondary"} update-button`}
        disabled={checking || opening}
        onClick={() => void (available ? openRelease() : check())}
      >
        {checking
          ? t("检查中…")
          : error
            ? t("重试")
            : available
              ? t("查看详情")
              : result?.status === "current"
                ? t("已是最新")
                : result?.status === "ahead"
                  ? t("暂无更新")
                  : t("检查更新")}
      </button>
    </div>
  );
}
