import { useCallback, useEffect, useRef, useState } from "react";
import { api, type Page } from "./api";

const empty: Page = { entries: [], total: 0, all: 0, pinned: 0 };

export function useHistory(fail: (error: unknown) => void) {
  const [view, setView] = useState({
    page: empty,
    selected: null as number | null,
    query: "",
    pinned: false,
    loading: true,
  });
  const current = useRef(view);
  const request = useRef(0);
  const following = useRef(true);
  const update = useCallback((patch: Partial<typeof view>) => {
    current.current = { ...current.current, ...patch };
    setView(current.current);
  }, []);
  const stopFollowing = useCallback(() => {
    following.current = false;
  }, []);

  const refresh = useCallback(async () => {
    const version = ++request.current;
    const { query, pinned, page } = current.current;
    try {
      const next = await api("List", query, pinned, 0);
      // Keep loaded pages so an update cannot discard a selection past item 50.
      const offsets: number[] = [];
      for (
        let offset = 50;
        offset < Math.min(page.entries.length, next.total);
        offset += 50
      )
        offsets.push(offset);
      const rest = await Promise.all(
        offsets.map((offset) => api("List", query, pinned, offset)),
      );
      if (version !== request.current) return;
      next.entries = [...next.entries, ...rest.flatMap((p) => p.entries)];
      // New captures can push the selected last item beyond the loaded page.
      while (
        !following.current &&
        current.current.selected !== null &&
        !next.entries.some((e) => e.id === current.current.selected) &&
        next.entries.length < next.total
      ) {
        const extra = await api("List", query, pinned, next.entries.length);
        if (version !== request.current) return;
        if (extra.entries.length === 0) break;
        next.entries.push(...extra.entries);
      }
      const previous = current.current;
      let selected = previous.selected;
      if (following.current) selected = next.entries[0]?.id ?? null;
      else if (!next.entries.some((e) => e.id === selected)) {
        const index = previous.page.entries.findIndex((e) => e.id === selected);
        const neighbours = [
          ...previous.page.entries.slice(index + 1),
          ...previous.page.entries.slice(0, Math.max(0, index)).reverse(),
        ];
        selected =
          neighbours.find((e) => next.entries.some((n) => n.id === e.id))?.id ??
          next.entries[0]?.id ??
          null;
      }
      update({ page: next, selected, loading: false });
    } catch (error) {
      if (version === request.current) {
        update({ loading: false });
        fail(error);
      }
    }
  }, [fail, update]);

  const setFilter = useCallback(
    (patch: { query?: string; pinned?: boolean }) => {
      stopFollowing();
      if (
        (patch.query === undefined || patch.query === current.current.query) &&
        (patch.pinned === undefined || patch.pinned === current.current.pinned)
      )
        return;
      // Invalidate immediately, before the debounced request starts.
      ++request.current;
      update({
        ...patch,
        // Global counts remain valid while the filtered entries are loading.
        page: { ...current.current.page, entries: [], total: 0 },
        selected: null,
        loading: true,
      });
    },
    [stopFollowing, update],
  );
  const setQuery = useCallback(
    (query: string) => setFilter({ query }),
    [setFilter],
  );
  const setPinned = useCallback(
    (pinned: boolean) => setFilter({ pinned }),
    [setFilter],
  );
  const setSelected = useCallback(
    (selected: number | null) => {
      stopFollowing();
      update({ selected });
    },
    [stopFollowing, update],
  );

  const reopen = useCallback(() => {
    following.current = true;
    ++request.current;
    // Clear the old action target while the first unfiltered page is loading.
    update({
      query: "",
      pinned: false,
      page: { ...current.current.page, entries: [], total: 0 },
      selected: null,
      loading: true,
    });
    void refresh();
  }, [refresh, update]);

  const more = useCallback(async () => {
    stopFollowing();
    const version = request.current;
    const { query, pinned, page } = current.current;
    const next = await api("List", query, pinned, page.entries.length);
    if (version === request.current && current.current.page === page)
      update({
        page: { ...next, entries: [...page.entries, ...next.entries] },
      });
  }, [stopFollowing, update]);

  useEffect(() => {
    const timer = setTimeout(() => void refresh(), 120);
    return () => clearTimeout(timer);
  }, [view.query, view.pinned, refresh]);
  useEffect(
    () => () => {
      ++request.current;
    },
    [],
  );

  return {
    ...view,
    following,
    stopFollowing,
    refresh,
    reopen,
    more,
    setQuery,
    setPinned,
    setSelected,
  };
}
