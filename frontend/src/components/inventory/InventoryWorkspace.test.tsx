import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  evaluateInventoryStockAlerts,
  simulateProcedureRecipeDeduction,
  renderInventoryWorkspaceHtml,
  type InventorySupplyItemDTO,
} from "./InventoryWorkspace.tsx";

describe("wi-12 S4 — Clinical Supplies Inventory, Low-Stock Alerts & Procedure Recipes (RF-14)", () => {
  const sampleItems: InventorySupplyItemDTO[] = [
    {
      id: "sup-01",
      sku: "ANEST-ART-4",
      name: "Carpules Articaína 4% con Epinefrina 1:100.000",
      consultorioId: "SILLON-1",
      unit: "unidades",
      currentStock: 8,
      minStockThreshold: 10,
      unitCostCents: 120_00,
    },
    {
      id: "sup-02",
      sku: "RES-A2-3M",
      name: "Jeringa Resina Compuesta Fotopolimerizable A2",
      consultorioId: "SILLON-1",
      unit: "gramos",
      currentStock: 25,
      minStockThreshold: 5,
      unitCostCents: 850_00,
    },
  ];

  it("flags low_stock_alert whenever currentStock <= minStockThreshold (S4, RF-14)", () => {
    const evaluated = evaluateInventoryStockAlerts(sampleItems);
    assert.equal(evaluated[0].lowStockAlert, true);
    assert.equal(evaluated[1].lowStockAlert, false);
  });

  it("calculates automatic stock deduction and total material cost for a procedure recipe (RF-14)", () => {
    const deduction = simulateProcedureRecipeDeduction(sampleItems, {
      procedureCode: "RES-16",
      procedureName: "Restauración Resina Compuesta",
      lines: [
        { supplyId: "sup-01", quantityPerProcedure: 2 },
        { supplyId: "sup-02", quantityPerProcedure: 1 },
      ],
    });
    assert.equal(deduction.totalRecipeMaterialCostCents, 1090_00);
    assert.equal(deduction.remainingStockBySupplyId["sup-01"], 6);
    assert.equal(deduction.remainingStockBySupplyId["sup-02"], 24);
  });

  it("renders low-stock warning badge and recipe editor with tabular-nums (S4)", () => {
    const html = renderInventoryWorkspaceHtml({
      items: sampleItems,
      activeRecipe: {
        procedureCode: "RES-16",
        procedureName: "Restauración Resina Compuesta",
        lines: [{ supplyId: "sup-01", quantityPerProcedure: 2 }],
      },
    });
    assert.match(html, /Stock Crítico \(RF-14\)/);
    assert.match(html, /tabular-nums/);
    assert.match(html, /ANEST-ART-4/);
  });
});
