/**
 * URL-serialised state of the portal data tables (wi-14 S6).
 *
 * The table state (search, filters, sort, page and visible columns) lives in
 * the query string, so a filtered view is shareable, survives a refresh and
 * keeps the browser's back button meaningful. The functions here are pure so
 * the contract can be unit-tested without a DOM.
 *
 * Every key is namespaced by the table id, because a page may host more than
 * one table and the portal's query string is shared by the whole route:
 *
 *   ?patients.q=ana&patients.f.estado=APROVED&patients.s=hcNumber:asc&patients.p=2
 */

export interface TableSortState {
  id: string;
  desc: boolean;
}

export interface TableUrlState {
  search: string;
  /** Faceted filter values keyed by column id. */
  filters: Record<string, string[]>;
  sorting: TableSortState[];
  pageIndex: number;
  pageSize: number;
  /** Column ids that are hidden. */
  hiddenColumns: string[];
}

export const DEFAULT_PAGE_SIZE = 10;
export const PAGE_SIZE_OPTIONS = [10, 20, 30, 50] as const;

const KEY = {
  search: "q",
  filter: "f",
  sort: "s",
  page: "p",
  pageSize: "ps",
  hidden: "c",
} as const;

function namespaced(tableId: string, key: string): string {
  return `${tableId}.${key}`;
}

/** Reads the table state out of a query string; missing keys mean defaults. */
export function readTableState(
  params: URLSearchParams,
  tableId: string,
  defaultPageSize: number = DEFAULT_PAGE_SIZE,
): TableUrlState {
  const filters: Record<string, string[]> = {};
  const filterPrefix = `${namespaced(tableId, KEY.filter)}.`;
  params.forEach((value, key) => {
    if (!key.startsWith(filterPrefix)) {
      return;
    }
    const columnId = key.slice(filterPrefix.length);
    if (columnId === "") {
      return;
    }
    filters[columnId] = value
      .split(",")
      .map((entry) => entry.trim())
      .filter((entry) => entry !== "");
  });

  // `?.` short-circuits the whole chain, so the `?? []` is what keeps `sorting`
  // an array instead of `undefined` when the parameter is absent.
  const sorting =
    params
      .get(namespaced(tableId, KEY.sort))
      ?.split(",")
      .map((entry) => entry.trim())
      .filter((entry) => entry !== "")
      .map((entry) => {
        const separator = entry.lastIndexOf(":");
        if (separator === -1) {
          return { id: entry, desc: false };
        }
        return { id: entry.slice(0, separator), desc: entry.slice(separator + 1) === "desc" };
      })
      .filter((entry) => entry.id !== "") ?? [];

  const hiddenColumns = (params.get(namespaced(tableId, KEY.hidden)) ?? "")
    .split(",")
    .map((entry) => entry.trim())
    .filter((entry) => entry !== "");

  const pageIndex = Number.parseInt(params.get(namespaced(tableId, KEY.page)) ?? "0", 10);
  const pageSize = Number.parseInt(
    params.get(namespaced(tableId, KEY.pageSize)) ?? String(defaultPageSize),
    10,
  );

  return {
    search: params.get(namespaced(tableId, KEY.search)) ?? "",
    filters,
    sorting,
    pageIndex: Number.isFinite(pageIndex) && pageIndex > 0 ? pageIndex : 0,
    pageSize: Number.isFinite(pageSize) && pageSize > 0 ? pageSize : defaultPageSize,
    hiddenColumns,
  };
}

/**
 * Writes the table state back into a query string, preserving the parameters
 * that belong to other parts of the page.
 */
export function writeTableState(
  state: TableUrlState,
  tableId: string,
  current?: URLSearchParams,
  defaultPageSize: number = DEFAULT_PAGE_SIZE,
): string {
  const params = new URLSearchParams(current ?? undefined);

  const setOrDelete = (key: string, value: string): void => {
    if (value === "") {
      params.delete(key);
      return;
    }
    params.set(key, value);
  };

  setOrDelete(namespaced(tableId, KEY.search), state.search);

  const filterPrefix = `${namespaced(tableId, KEY.filter)}.`;
  for (const key of [...params.keys()]) {
    if (key.startsWith(filterPrefix)) {
      params.delete(key);
    }
  }
  for (const [columnId, values] of Object.entries(state.filters)) {
    if (values.length === 0) {
      continue;
    }
    params.set(`${filterPrefix}${columnId}`, values.join(","));
  }

  setOrDelete(
    namespaced(tableId, KEY.sort),
    state.sorting.map((entry) => `${entry.id}:${entry.desc ? "desc" : "asc"}`).join(","),
  );
  setOrDelete(namespaced(tableId, KEY.page), state.pageIndex > 0 ? String(state.pageIndex) : "");
  // The page size is only written when it differs from the default, so a
  // pristine table leaves no trace in the URL.
  setOrDelete(
    namespaced(tableId, KEY.pageSize),
    state.pageSize !== defaultPageSize ? String(state.pageSize) : "",
  );
  setOrDelete(namespaced(tableId, KEY.hidden), state.hiddenColumns.join(","));

  const query = params.toString();
  return query === "" ? "" : `?${query}`;
}
