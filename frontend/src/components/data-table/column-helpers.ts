/**
 * Shared helpers of the portal data tables (wi-14 S6, AGENTS.md §3).
 *
 * `tabular-nums` is not cosmetic here: the AGENTS.md §3 typography contract
 * requires it for periodontal probing depths, tooth numbers (FDI/Universal)
 * and every money column, so that digits align vertically down a table and
 * can be compared at a glance on a chairside tablet.
 */

import type { FilterFn } from "@tanstack/react-table";

export const NUMERIC_CELL_CLASS = "text-right tabular-nums";

/** Class for a numeric cell that also has to carry a currency symbol. */
export const MONEY_CELL_CLASS = `text-right tabular-nums font-medium`;

/**
 * Filter of a faceted column: keeps the rows whose value is one of the
 * selected options.
 *
 * A facet stores an **array** of values, so the stock `equalsString` filter
 * (which expects a single string) would throw `filterValue.toLowerCase is not
 * a function`. `DataTable` installs this filter on every column that has a
 * declared facet, so a screen only has to describe the facet and not repeat
 * the filter semantics.
 */
export function createMultiSelectFilter<TData>(): FilterFn<TData> {
  return (row, columnId, filterValue) => {
    const selected = Array.isArray(filterValue) ? filterValue : [filterValue];
    if (selected.length === 0) {
      return true;
    }
    const value = row.getValue(columnId);
    return selected.some((entry) => String(entry) === String(value));
  };
}

/**
 * Formats a monetary amount held in cents.
 *
 * The amount is formatted with the Ecuadorian convention (thousands separated
 * by a dot, decimals by a comma) and without floating point arithmetic, so a
 * cash total shown in the portal always agrees with the server's
 * `theoretical_cents` (`RN-02`).
 */
export function formatCents(cents: number, currencySymbol = "$"): string {
  const safe = Number.isFinite(cents) ? cents : 0;
  const sign = safe < 0 ? "-" : "";
  const absolute = Math.abs(safe);
  const units = Math.floor(absolute / 100);
  const remainder = String(absolute % 100).padStart(2, "0");
  return `${sign}${currencySymbol}${units.toLocaleString("es-EC")},${remainder}`;
}
