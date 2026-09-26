import * as service from "../bindings/pastewin/clipboardservice";
import { Events } from "@wailsio/runtime";

export interface Entry {
  id: number;
  text: string;
  preview: string;
  characters: number;
  createdAt: number;
  updatedAt: number;
  pinned: boolean;
}
export interface Page {
  entries: Entry[];
  total: number;
  all: number;
  pinned: number;
}
export interface Status {
  paused: boolean;
  hotkey: string;
  hotkeyError: string;
  databasePath: string;
  canPaste: boolean;
}
export const preview =
  import.meta.env.DEV &&
  new URLSearchParams(location.search).get("preview") === "1";

interface API {
  List(query: string, pinned: boolean, offset: number): Promise<Page>;
  Get(id: number): Promise<Entry>;
  Pin(id: number, pinned: boolean): Promise<void>;
  Delete(id: number): Promise<void>;
  ClearUnpinned(): Promise<void>;
  Copy(id: number): Promise<void>;
  Paste(id: number): Promise<void>;
  SetPaused(paused: boolean): Promise<void>;
  AutoStart(): Promise<boolean>;
  SetAutoStart(enabled: boolean): Promise<boolean>;
  Status(): Promise<Status>;
  Hide(): Promise<void>;
  Quit(): Promise<void>;
}
// Keep the Wails runtime and generated bindings in the same static module graph.
// Splitting both into dynamic entry chunks creates a runtime initialisation cycle.
const backend: Promise<API> = preview
  ? import("./preview").then((m) => m.demo)
  : Promise.resolve({
      ...service,
      async List(query: string, pinned: boolean, offset: number) {
        const result = await service.List(query, pinned, offset);
        return { ...result, entries: result.entries ?? [] };
      },
    });
export async function api<K extends keyof API>(
  method: K,
  ...args: Parameters<API[K]>
): Promise<Awaited<ReturnType<API[K]>>> {
  const service = await backend;
  return (service[method] as (...a: Parameters<API[K]>) => ReturnType<API[K]>)(
    ...args,
  ) as Awaited<ReturnType<API[K]>>;
}
export async function listen(
  name: string,
  callback: (data: unknown) => void,
): Promise<() => void> {
  if (preview) {
    const fn = (e: Event) => callback((e as CustomEvent).detail);
    window.addEventListener(name, fn);
    return () => window.removeEventListener(name, fn);
  }
  return Events.On(name, (e) => callback(e.data));
}
