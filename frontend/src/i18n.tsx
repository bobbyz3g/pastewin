import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { api } from "./api";
import { english } from "./messages";
import { englishErrors } from "./messages-errors";

type Language = "zh-CN" | "en";
type Message = keyof typeof english;
type Params = Record<string, string | number>;
type I18n = {
  language: Language;
  setLanguage: (value: string) => Promise<void>;
  t: (key: Message, params?: Params) => string;
  translateError: (value: string) => string;
};
const Context = createContext<I18n | null>(null);
const englishOne: Partial<Record<Message, string>> = {
  "{count} 字符": "{count} character",
  "{count} 条匹配": "{count} match",
  "{count} 条记录": "{count} record",
};

export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, updateLanguage] = useState<Language>("zh-CN");
  const [ready, setReady] = useState(false);
  const [error, setError] = useState("");
  const [attempt, retry] = useState(0);
  useEffect(() => {
    let active = true;
    setError("");
    api("Language")
      .then((value) => {
        if (!active) return;
        updateLanguage(value === "en" ? "en" : "zh-CN");
        setReady(true);
      })
      .catch((error) => {
        if (active) setError(String(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);
  useEffect(() => {
    document.documentElement.lang = language;
  }, [language]);
  const setLanguage = useCallback(async (value: string) => {
    if (value !== "en" && value !== "zh-CN")
      throw new Error("Unsupported language");
    // Commit the preference before changing the UI, so failures cannot look saved.
    await api("SetLanguage", value);
    updateLanguage(value);
  }, []);
  const t = useCallback(
    (key: Message, params: Params = {}) => {
      const text =
        language === "en"
          ? ((Number(params.count) === 1 ? englishOne[key] : undefined) ??
            english[key])
          : key;
      return text.replace(/\{(\w+)\}/g, (token, name) =>
        String(params[name] ?? token),
      );
    },
    [language],
  );
  const translateError = useCallback(
    (value: string): string => {
      if (language !== "en") return value;
      // Translate known application messages only. Keep OS diagnostics and data intact.
      for (const [source, translated] of Object.entries(englishErrors)) {
        if (value === source) return translated;
        if (
          value.startsWith(source + "：") ||
          value.startsWith(source + ": ")
        ) {
          return (
            translated +
            ": " +
            translateError(
              value.slice(
                source.length + (value[source.length] === "：" ? 1 : 2),
              ),
            )
          );
        }
      }
      return value;
    },
    [language],
  );
  if (!ready)
    return (
      <div className="empty-list" role="status">
        {error ? (
          <>
            <p>无法读取语言设置 / Unable to load language settings</p>
            <p>{error}</p>
            <button onClick={() => retry((value) => value + 1)}>
              重试 / Retry
            </button>
          </>
        ) : (
          "PasteWin…"
        )}
      </div>
    );
  return (
    <Context.Provider value={{ language, setLanguage, t, translateError }}>
      {children}
    </Context.Provider>
  );
}

export function useI18n() {
  const context = useContext(Context);
  if (!context) throw new Error("I18nProvider is required");
  return context;
}
