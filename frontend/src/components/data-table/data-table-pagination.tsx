/**
 * Pagination of the portal data tables (wi-14 S6).
 *
 * The pagination is client-side over the set the server already delivered:
 * `wi-15` adds the read endpoints and `wi-16` wires the tables to them, but
 * the API contract exposes `limit`/`offset` rather than a cursor, so the
 * table keeps the whole result set in memory and paginates locally.
 */

"use client";

import type { Table } from "@tanstack/react-table";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { PAGE_SIZE_OPTIONS } from "@/lib/table-state";

export interface DataTablePaginationProps<TData> {
  table: Table<TData>;
  /** Text template for the range summary. */
  label?: string;
}

export function DataTablePagination<TData>({
  table,
  label = "filas",
}: DataTablePaginationProps<TData>) {
  const { pageIndex, pageSize } = table.getState().pagination;
  const rowCount = table.getFilteredRowModel().rows.length;
  const first = rowCount === 0 ? 0 : pageIndex * pageSize + 1;
  const last = Math.min(pageIndex * pageSize + pageSize, rowCount);

  return (
    <div className="flex flex-col gap-3 border-t border-slate-200 px-2 py-3 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-sm text-slate-600" data-testid="data-table-pagination-summary">
        {rowCount === 0 ? (
          <>Sin {label}</>
        ) : (
          <>
            <span className="tabular-nums font-medium text-slate-900">
              {first}–{last}
            </span>{" "}
            de <span className="tabular-nums font-medium text-slate-900">{rowCount}</span>{" "}
            {label}
          </>
        )}
      </p>

      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2">
          <label htmlFor="data-table-page-size" className="text-sm text-slate-600">
            Filas por página
          </label>
          <Select
            value={String(pageSize)}
            onValueChange={(value) => table.setPageSize(Number(value))}
          >
            <SelectTrigger id="data-table-page-size" className="h-8 w-20 border-slate-200">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PAGE_SIZE_OPTIONS.map((option) => (
                <SelectItem key={option} value={String(option)}>
                  {option}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex items-center gap-1">
          <Button
            variant="outline"
            size="icon"
            className="size-8 border-slate-200"
            aria-label="Página anterior"
            disabled={!table.getCanPreviousPage()}
            onClick={() => table.previousPage()}
          >
            <ChevronLeft className="size-4" aria-hidden="true" />
          </Button>
          <span className="px-2 text-sm text-slate-600 tabular-nums">
            {pageIndex + 1} / {Math.max(1, table.getPageCount())}
          </span>
          <Button
            variant="outline"
            size="icon"
            className="size-8 border-slate-200"
            aria-label="Página siguiente"
            disabled={!table.getCanNextPage()}
            onClick={() => table.nextPage()}
          >
            <ChevronRight className="size-4" aria-hidden="true" />
          </Button>
        </div>
      </div>
    </div>
  );
}
