/**
 * URL state of the data tables (wi-14 S6).
 *
 * The table state has to survive a refresh, be shareable and keep the
 * browser's back button meaningful. These tests pin the serialisation
 * contract that the `DataTable` client component relies on.
 */

import { describe, expect, it } from "vitest";

import {
  DEFAULT_PAGE_SIZE,
  readTableState,
  writeTableState,
  type TableUrlState,
} from "./table-state.ts";

function emptyState(overrides: Partial<TableUrlState> = {}): TableUrlState {
  return {
    search: "",
    filters: {},
    sorting: [],
    pageIndex: 0,
    pageSize: DEFAULT_PAGE_SIZE,
    hiddenColumns: [],
    ...overrides,
  };
}

describe("wi-14 S6 — table state in the query string", () => {
  it("reads the defaults out of an empty query string", () => {
    const state = readTableState(new URLSearchParams(), "patients");
    expect(state).toEqual(emptyState());
  });

  it("round-trips search, filters, sorting, page and hidden columns", () => {
    const original = emptyState({
      search: "ana fernández",
      filters: { estado: ["APROBADO", "PENDIENTE"], clinica: ["Norte"] },
      sorting: [{ id: "hcNumber", desc: false }, { id: "edad", desc: true }],
      pageIndex: 3,
      pageSize: 20,
      hiddenColumns: ["telefono"],
    });

    const query = writeTableState(original, "patients");
    const restored = readTableState(new URLSearchParams(query), "patients");

    expect(restored).toEqual(original);
  });

  it("namespaces every key so two tables can share a route", () => {
    const query = writeTableState(emptyState({ search: "ana" }), "patients");
    expect(query).toContain("patients.q=ana");
    expect(query).not.toContain("other.");
  });

  it("preserves the query parameters that belong to other parts of the page", () => {
    const current = new URLSearchParams("patients.q=ana&view=compact&page=2");
    const query = writeTableState(emptyState({ search: "ana", pageIndex: 1 }), "patients", current);

    const params = new URLSearchParams(query);
    expect(params.get("view")).toBe("compact");
    expect(params.get("patients.q")).toBe("ana");
  });

  it("drops the filters that no longer apply instead of leaving them behind", () => {
    const current = new URLSearchParams("patients.f.estado=APROBADO");
    const query = writeTableState(emptyState({ filters: {} }), "patients", current);

    expect(new URLSearchParams(query).get("patients.f.estado")).toBeNull();
  });

  it("rejects malformed page numbers instead of rendering an empty table", () => {
    const state = readTableState(
      new URLSearchParams("patients.p=-4&patients.ps=not-a-number"),
      "patients",
    );
    expect(state.pageIndex).toBe(0);
    expect(state.pageSize).toBe(DEFAULT_PAGE_SIZE);
  });

  it("ignores empty filter values", () => {
    const state = readTableState(new URLSearchParams("patients.f.estado=,APROBADO,"), "patients");
    expect(state.filters.estado).toEqual(["APROBADO"]);
  });

  it("returns an empty query string when there is nothing to serialise", () => {
    expect(writeTableState(emptyState(), "patients")).toBe("");
  });
});
