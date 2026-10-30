/**
 * Generic data table of the SIGO-Dental portal (wi-14 S6).
 *
 * This is the `shadcn-admin` list pattern: one screen per entity, the list on
 * the page and the create/edit actions in a `Sheet`. The table is a client
 * island; the data arrives already loaded by the server component that renders
 * it, so the browser never calls the API Gateway directly (wi-13 S4).
 *
 * The table state (search, filters, sorting, page, visible columns) lives in
 * the query string through `src/lib/table-state.ts`, which makes a filtered
 * view shareable and keeps the browser's back button meaningful.
 */

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  flexRender,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type ColumnFiltersState,
  type SortingState,
  type VisibilityState,
} from "@tanstack/react-table";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { createMultiSelectFilter } from "./column-helpers.ts";
import {
  DEFAULT_PAGE_SIZE,
  readTableState,
  writeTableState,
  type TableUrlState,
} from "@/lib/table-state";
import { DataTablePagination } from "./data-table-pagination.tsx";
import { DataTableToolbar } from "./data-table-toolbar.tsx";
import type { DataTableFacet } from "./data-table-faceted-filter.tsx";

export interface DataTableProps<TData> {
  /** Namespaces the table state inside the query string. */
  tableId: string;
  columns: ColumnDef<TData, unknown>[];
  data: TData[];
  facets?: DataTableFacet[];
  searchPlaceholder?: string;
  emptyMessage?: string;
  pageSize?: number;
  /** Labels the unit of the pagination summary. */
  rowLabel?: string;
}

function toColumnFiltersState(filters: Record<string, string[]>): ColumnFiltersState {
  return Object.entries(filters).map(([id, value]) => ({ id, value }));
}

function toVisibilityState(hiddenColumns: string[]): VisibilityState {
  const visibility: VisibilityState = {};
  for (const id of hiddenColumns) {
    visibility[id] = false;
  }
  return visibility;
}

export function DataTable<TData>({
  tableId,
  columns,
  data,
  facets = [],
  searchPlaceholder,
  emptyMessage = "No hay registros para mostrar.",
  pageSize = DEFAULT_PAGE_SIZE,
  rowLabel = "filas",
}: DataTableProps<TData>) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const initialState = useMemo(
    () => readTableState(searchParams, tableId, pageSize),
    // The URL is the source of truth only on the first render: afterwards the
    // component owns the state and writes it back.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  const [globalFilter, setGlobalFilter] = useState(initialState.search);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>(() =>
    toColumnFiltersState(initialState.filters),
  );
  const [sorting, setSorting] = useState<SortingState>(initialState.sorting);
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>(() =>
    toVisibilityState(initialState.hiddenColumns),
  );
  const [pagination, setPagination] = useState({
    pageIndex: initialState.pageIndex,
    pageSize: initialState.pageSize,
  });

  // A facet stores an array of selected values, so every faceted column gets
  // the multi-select filter. Declaring the facet is enough: the screen does
  // not have to repeat the filter semantics in its column definition.
  const tableColumns = useMemo(() => {
    const facetIds = new Set(facets.map((facet) => facet.id));
    if (facetIds.size === 0) {
      return columns;
    }
    return columns.map((column) => {
      const columnId =
        "id" in column && typeof column.id === "string"
          ? column.id
          : "accessorKey" in column && typeof column.accessorKey === "string"
            ? column.accessorKey
            : null;
      if (!columnId || !facetIds.has(columnId)) {
        return column;
      }
      return { ...column, filterFn: createMultiSelectFilter<TData>() };
    });
  }, [columns, facets]);

  const table = useReactTable({
    data,
    columns: tableColumns,
    state: { globalFilter, columnFilters, sorting, columnVisibility, pagination },
    onGlobalFilterChange: setGlobalFilter,
    onColumnFiltersChange: setColumnFilters,
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    globalFilterFn: "includesString",
    getRowId: (row, index) => String((row as { id?: string }).id ?? index),
  });

  // The table state is written back to the URL so that a filtered view can be
  // shared and survives a refresh. `replace` keeps the browser history clean.
  const syncToUrl = useCallback(() => {
    const state: TableUrlState = {
      search: globalFilter,
      filters: Object.fromEntries(
        columnFilters
          .filter((filter) => Array.isArray(filter.value))
          .map((filter) => [filter.id, filter.value as string[]]),
      ),
      sorting: sorting.map((entry) => ({ id: entry.id, desc: entry.desc })),
      pageIndex: pagination.pageIndex,
      pageSize: pagination.pageSize,
      hiddenColumns: Object.entries(columnVisibility)
        .filter(([, visible]) => !visible)
        .map(([id]) => id),
    };
    const query = writeTableState(state, tableId, searchParams, pageSize);
    const target = query === "" ? pathname : `${pathname}${query}`;

    // The effect depends on `searchParams`, which changes after every
    // replacement. Without this guard the component would keep replacing the
    // URL with an identical one, and each replacement would schedule the
    // effect again.
    const current = searchParams.toString() === "" ? pathname : `${pathname}?${searchParams.toString()}`;
    if (target === current) {
      return;
    }

    router.replace(target, { scroll: false });
  }, [
    columnFilters,
    columnVisibility,
    globalFilter,
    pagination.pageIndex,
    pagination.pageSize,
    pageSize,
    pathname,
    router,
    searchParams,
    sorting,
    tableId,
  ]);

  // A short debounce keeps the query string from being rewritten on every
  // keystroke of the search box.
  useEffect(() => {
    const timer = setTimeout(syncToUrl, 250);
    return () => clearTimeout(timer);
  }, [syncToUrl]);

  const rows = table.getRowModel().rows;

  return (
    <div className="rounded-lg border border-slate-200 bg-white">
      <DataTableToolbar
        table={table}
        facets={facets}
        searchPlaceholder={searchPlaceholder}
      />
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="bg-slate-50 hover:bg-slate-50">
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id} className="text-slate-700">
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={table.getVisibleLeafColumns().length}
                  className="h-24 text-center text-sm text-slate-500"
                >
                  {emptyMessage}
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row) => (
                <TableRow key={row.id} data-state={row.getIsSelected() && "selected"}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
      <DataTablePagination table={table} label={rowLabel} />
    </div>
  );
}
