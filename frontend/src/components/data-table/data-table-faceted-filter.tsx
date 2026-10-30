/**
 * Faceted filter of the portal data tables (wi-14 S6).
 *
 * A facet narrows a column to a set of values. The selected values are part
 * of the table's URL state, so a filtered view is shareable.
 */

"use client";

import { useMemo } from "react";
import type { Column } from "@tanstack/react-table";
import { Check, ChevronsUpDown } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

export interface DataTableFacetOption {
  value: string;
  label: string;
}

export interface DataTableFacet {
  /** Column id the facet applies to. */
  id: string;
  label: string;
  options: DataTableFacetOption[];
}

export interface DataTableFacetedFilterProps<TData> {
  column: Column<TData, unknown>;
  title: string;
  options: DataTableFacetOption[];
}

export function DataTableFacetedFilter<TData>({
  column,
  title,
  options,
}: DataTableFacetedFilterProps<TData>) {
  const selected = useMemo(
    () => (column.getFilterValue() as string[] | undefined) ?? [],
    [column],
  );

  const toggle = (value: string): void => {
    const next = selected.includes(value)
      ? selected.filter((entry) => entry !== value)
      : [...selected, value];
    column.setFilterValue(next.length === 0 ? undefined : next);
  };

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className={cn(
            "h-9 border-slate-200 bg-white",
            selected.length > 0 && "border-slate-900 bg-slate-50",
          )}
        >
          <span>{title}</span>
          {selected.length > 0 ? (
            <Badge variant="secondary" className="ml-1 bg-slate-900 text-white">
              {selected.length}
            </Badge>
          ) : null}
          <ChevronsUpDown className="ml-1 size-3.5 text-slate-500" aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-56 p-0">
        <Command>
          <CommandList>
            <CommandEmpty>Sin resultados.</CommandEmpty>
            <CommandGroup>
              {options.map((option) => {
                const isSelected = selected.includes(option.value);
                return (
                  <CommandItem
                    key={option.value}
                    value={option.value}
                    onSelect={() => toggle(option.value)}
                    className="gap-2"
                  >
                    <div
                      className={cn(
                        "flex size-4 items-center justify-center rounded-sm border border-slate-300",
                        isSelected && "border-slate-900 bg-slate-900 text-white",
                      )}
                      aria-hidden="true"
                    >
                      {isSelected ? <Check className="size-3" /> : null}
                    </div>
                    <span>{option.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </CommandList>
        </Command>
        {selected.length > 0 ? (
          <>
            <Separator className="bg-slate-200" />
            <div className="p-1">
              <Button
                variant="ghost"
                size="sm"
                className="w-full text-slate-700"
                onClick={() => column.setFilterValue(undefined)}
              >
                Limpiar filtro
              </Button>
            </div>
          </>
        ) : null}
      </PopoverContent>
    </Popover>
  );
}
