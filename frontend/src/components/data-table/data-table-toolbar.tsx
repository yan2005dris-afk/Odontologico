/**
 * Toolbar of the portal data tables (wi-14 S6).
 *
 * Holds the instant search, the faceted filters and the column visibility
 * selector. The toolbar never renders a creation form: creating and editing
 * happen in a `Sheet`, so the operator stays anchored in the list
 * (AGENTS.md §3.1).
 */

"use client";

import type { Table } from "@tanstack/react-table";
import { Search } from "lucide-react";

import { Input } from "@/components/ui/input";
import {
  DataTableFacetedFilter,
  type DataTableFacet,
} from "./data-table-faceted-filter.tsx";
import { DataTableViewOptions } from "./data-table-view-options.tsx";

export interface DataTableToolbarProps<TData> {
  table: Table<TData>;
  facets?: DataTableFacet[];
  searchPlaceholder?: string;
  enableColumnVisibility?: boolean;
}

export function DataTableToolbar<TData>({
  table,
  facets = [],
  searchPlaceholder = "Buscar…",
  enableColumnVisibility = true,
}: DataTableToolbarProps<TData>) {
  return (
    <div className="flex flex-col gap-3 border-b border-slate-200 px-2 py-3 lg:flex-row lg:items-center lg:justify-between">
      <div className="flex flex-1 flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative w-full sm:max-w-xs">
          <Search
            className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-slate-400"
            aria-hidden="true"
          />
          <Input
            type="search"
            aria-label={searchPlaceholder}
            placeholder={searchPlaceholder}
            value={(table.getState().globalFilter as string) ?? ""}
            onChange={(event) => table.setGlobalFilter(event.target.value)}
            className="h-9 border-slate-200 pl-8"
          />
        </div>

        {facets.map((facet) => {
          const column = table.getColumn(facet.id);
          if (!column) {
            return null;
          }
          return (
            <DataTableFacetedFilter
              key={facet.id}
              column={column}
              title={facet.label}
              options={facet.options}
            />
          );
        })}
      </div>

      <div className="flex items-center justify-end gap-2">
        {enableColumnVisibility ? <DataTableViewOptions table={table} /> : null}
      </div>
    </div>
  );
}
