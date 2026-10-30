/**
 * Data table contract (wi-14 S6).
 *
 * The table is the list primitive of the whole portal: every entity screen
 * of `wi-16` and `wi-17` is built on it. These tests pin the behaviour the
 * screens depend on — filtering, faceting, pagination and the numeric
 * typography required by AGENTS.md §3.
 */

import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ColumnDef } from "@tanstack/react-table";

const replaceMock = vi.fn();
let currentQuery = "";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock, push: vi.fn(), refresh: vi.fn() }),
  usePathname: () => "/patients",
  useSearchParams: () => new URLSearchParams(currentQuery),
}));

import { DataTable } from "./data-table.tsx";
import { MONEY_CELL_CLASS, NUMERIC_CELL_CLASS, formatCents } from "./column-helpers.ts";

interface Paciente {
  id: string;
  hcNumber: string;
  fullName: string;
  estado: "APROBADO" | "PENDIENTE";
  totalCents: number;
}

const PACIENTES: Paciente[] = [
  { id: "p1", hcNumber: "HC-2026-0001", fullName: "Ana Lucía Fernández", estado: "APROBADO", totalCents: 150000 },
  { id: "p2", hcNumber: "HC-2026-0002", fullName: "Carlos Mendoza", estado: "PENDIENTE", totalCents: 85000 },
  { id: "p3", hcNumber: "HC-2026-0003", fullName: "Sofía Ríos", estado: "APROBADO", totalCents: 320000 },
];

const COLUMNS: ColumnDef<Paciente, unknown>[] = [
  { accessorKey: "hcNumber", header: "HC" },
  { accessorKey: "fullName", header: "Paciente" },
  {
    accessorKey: "estado",
    header: "Estado",
    filterFn: "equalsString",
  },
  {
    accessorKey: "totalCents",
    header: "Total",
    cell: ({ getValue }) => (
      <span className={MONEY_CELL_CLASS}>{formatCents(getValue<Paciente["totalCents"]>())}</span>
    ),
  },
];

function renderTabla(overrides: Partial<React.ComponentProps<typeof DataTable<Paciente>>> = {}) {
  return render(
    <DataTable<Paciente>
      tableId="patients"
      columns={COLUMNS}
      data={PACIENTES}
      searchPlaceholder="Buscar paciente"
      facets={[
        {
          id: "estado",
          label: "Estado",
          options: [
            { value: "APROBADO", label: "Aprobado" },
            { value: "PENDIENTE", label: "Pendiente" },
          ],
        },
      ]}
      {...overrides}
    />,
  );
}

describe("wi-14 S6 — DataTable", () => {
  beforeEach(() => {
    replaceMock.mockClear();
    currentQuery = "";
  });

  it("renders the headers and one row per record", () => {
    renderTabla();

    expect(screen.getByRole("columnheader", { name: "HC" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Paciente" })).toBeInTheDocument();
    expect(screen.getAllByRole("row")).toHaveLength(PACIENTES.length + 1);
    expect(screen.getByText("Ana Lucía Fernández")).toBeInTheDocument();
  });

  it("filters the rows with the instant search", async () => {
    const user = userEvent.setup();
    renderTabla();

    await user.type(screen.getByRole("searchbox", { name: "Buscar paciente" }), "Sofía");

    expect(screen.getByText("Sofía Ríos")).toBeInTheDocument();
    expect(screen.queryByText("Ana Lucía Fernández")).not.toBeInTheDocument();
  });

  it("narrows the rows with a faceted filter", async () => {
    const user = userEvent.setup();
    renderTabla();

    await user.click(screen.getByRole("button", { name: /Estado/ }));
    await user.click(screen.getByRole("option", { name: "Pendiente" }));

    expect(screen.getByText("Carlos Mendoza")).toBeInTheDocument();
    expect(screen.queryByText("Sofía Ríos")).not.toBeInTheDocument();
  });

  it("shows an explicit empty state when nothing matches", async () => {
    const user = userEvent.setup();
    renderTabla();

    await user.type(screen.getByRole("searchbox", { name: "Buscar paciente" }), "zzzzz");

    expect(screen.getByText("No hay registros para mostrar.")).toBeInTheDocument();
  });

  it("summarises the visible range with tabular numerals", () => {
    renderTabla();

    const summary = screen.getByTestId("data-table-pagination-summary");
    expect(summary).toHaveTextContent("1\u20133");
    expect(summary).toHaveTextContent("de 3 filas");
  });

  it("keeps the money columns in tabular numerals", () => {
    const { container } = renderTabla();

    const moneyCell = container.querySelector(`.${MONEY_CELL_CLASS.replace(/ /g, ".")}`);
    expect(moneyCell).not.toBeNull();
    expect(moneyCell?.textContent).toContain("1.500,00");
  });

  it("exposes the numeric cell class so the columns stay aligned", () => {
    expect(NUMERIC_CELL_CLASS).toContain("tabular-nums");
    expect(MONEY_CELL_CLASS).toContain("tabular-nums");
  });

  it("writes the table state back into the query string", async () => {
    const user = userEvent.setup();
    renderTabla();

    await user.type(screen.getByRole("searchbox", { name: "Buscar paciente" }), "Ana");

    await vi.waitFor(() => {
      expect(replaceMock).toHaveBeenCalled();
    });
    const target = String(replaceMock.mock.calls.at(-1)?.[0] ?? "");
    expect(target).toContain("patients.q=Ana");
  });

  it("does not rewrite the URL when the state did not change", async () => {
    renderTabla();

    await vi.waitFor(() => {
      expect(replaceMock).not.toHaveBeenCalled();
    });
  });

  it("keeps the pagination controls reachable by keyboard", async () => {
    const user = userEvent.setup();
    renderTabla({ pageSize: 1 });

    const next = screen.getByRole("button", { name: "Página siguiente" });
    expect(next).toBeEnabled();

    await user.click(next);

    const summary = screen.getByText(/\d+ \/ \d+/);
    expect(summary).toHaveTextContent("2 / 3");
  });

  it("labels the rows of the table for screen readers", () => {
    renderTabla();

    const table = screen.getByRole("table");
    expect(within(table).getAllByRole("row")).toHaveLength(PACIENTES.length + 1);
  });
});

describe("wi-14 S6 — money formatting", () => {
  it("formats cents without losing precision", () => {
    expect(formatCents(150000)).toBe("$1.500,00");
    expect(formatCents(85)).toBe("$0,85");
    expect(formatCents(-5000)).toBe("-$50,00");
  });

  it("never renders NaN for a corrupted amount", () => {
    expect(formatCents(Number.NaN)).toBe("$0,00");
  });
});
