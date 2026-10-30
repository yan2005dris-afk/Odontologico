/**
 * InventoryWorkspace Component (wi-12 S4, RF-14, AGENTS.md §3).
 * Consultorio clinical supplies catalog, critical low-stock alert badges (`currentStock <= minStockThreshold`),
 * and standard consumption recipes per clinical procedure code.
 */

import { escapeHtml, formatCentsCurrency } from "../../lib/session-idle-guard.ts";

export interface InventorySupplyItemDTO {
  id: string;
  sku: string;
  name: string;
  consultorioId: string;
  unit: string;
  currentStock: number;
  minStockThreshold: number;
  unitCostCents: number;
}

export interface EvaluatedInventorySupplyItem extends InventorySupplyItemDTO {
  lowStockAlert: boolean;
}

export interface ProcedureSupplyRecipeLineDTO {
  supplyId: string;
  quantityPerProcedure: number;
}

export interface ProcedureSupplyRecipeDTO {
  procedureCode: string;
  procedureName: string;
  lines: ProcedureSupplyRecipeLineDTO[];
}

export interface InventoryWorkspaceProps {
  items: InventorySupplyItemDTO[];
  activeRecipe?: ProcedureSupplyRecipeDTO;
}

/**
 * Evaluates critical low-stock alerts where currentStock <= minStockThreshold (S4, RF-14).
 */
export function evaluateInventoryStockAlerts(
  items: InventorySupplyItemDTO[]
): EvaluatedInventorySupplyItem[] {
  return items.map((item) => ({
    ...item,
    lowStockAlert: item.currentStock <= item.minStockThreshold,
  }));
}

/**
 * Simulates automatic supply deduction and calculates material cost in cents upon procedure completion (RF-14).
 */
export function simulateProcedureRecipeDeduction(
  items: InventorySupplyItemDTO[],
  recipe: ProcedureSupplyRecipeDTO
): {
  totalRecipeMaterialCostCents: number;
  remainingStockBySupplyId: Record<string, number>;
} {
  const byId = new Map(items.map((item) => [item.id, item]));
  let totalRecipeMaterialCostCents = 0;
  const remainingStockBySupplyId: Record<string, number> = {};

  for (const item of items) {
    remainingStockBySupplyId[item.id] = item.currentStock;
  }

  for (const line of recipe.lines) {
    const supply = byId.get(line.supplyId);
    if (supply) {
      totalRecipeMaterialCostCents += supply.unitCostCents * line.quantityPerProcedure;
      remainingStockBySupplyId[supply.id] = Math.max(
        0,
        supply.currentStock - line.quantityPerProcedure
      );
    }
  }

  return {
    totalRecipeMaterialCostCents,
    remainingStockBySupplyId,
  };
}

export function renderInventoryWorkspaceHtml(props: InventoryWorkspaceProps): string {
  const evaluatedItems = evaluateInventoryStockAlerts(props.items);

  const rowsHtml = evaluatedItems
    .map(
      (item) => `<tr class="border-b border-slate-100 text-xs">
        <td class="py-2 px-3 font-mono tabular-nums">${escapeHtml(item.sku)}</td>
        <td class="py-2 px-3 text-slate-900 font-medium">${escapeHtml(item.name)}</td>
        <td class="py-2 px-3 font-mono tabular-nums">${escapeHtml(item.consultorioId)}</td>
        <td class="py-2 px-3 font-mono font-bold tabular-nums text-right">${item.currentStock} ${escapeHtml(
          item.unit
        )}</td>
        <td class="py-2 px-3 font-mono tabular-nums text-right">${item.minStockThreshold}</td>
        <td class="py-2 px-3 font-mono tabular-nums text-right">${formatCentsCurrency(
          item.unitCostCents
        )}</td>
        <td class="py-2 px-3 text-right">
          ${
            item.lowStockAlert
              ? `<span role="alert" class="px-2 py-0.5 rounded bg-red-600 text-white font-semibold text-[11px]">Stock Crítico (RF-14)</span>`
              : `<span class="px-2 py-0.5 rounded bg-emerald-50 text-emerald-800 font-medium text-[11px]">Óptimo</span>`
          }
        </td>
      </tr>`
    )
    .join("");

  const recipeHtml = props.activeRecipe
    ? `<div class="p-4 rounded-lg bg-slate-50 border border-slate-200 space-y-2">
        <h3 class="text-xs font-bold text-slate-800">Receta Estándar de Consumo Automático: ${escapeHtml(
          props.activeRecipe.procedureCode
        )} — ${escapeHtml(props.activeRecipe.procedureName)}</h3>
        <p class="text-[11px] text-slate-600">Descuento automático de inventario al marcar procedimiento como REALIZADO (RF-14).</p>
      </div>`
    : "";

  return `<section class="bg-white rounded-xl border border-slate-200 p-6 space-y-5 tabular-nums" aria-label="Inventario de Insumos Clínicos y Recetas RF-14">
    <header class="border-b border-slate-200 pb-3">
      <h2 class="text-base font-bold text-slate-900">Inventario de Insumos Clínicos por Consultorio (RF-14)</h2>
      <p class="text-xs text-slate-600">Control de stock crítico y recetas de descargo automático por procedimiento realizado</p>
    </header>

    <div class="overflow-x-auto">
      <table class="w-full text-left tabular-nums">
        <thead class="bg-slate-50 text-[11px] text-slate-600 border-b border-slate-200">
          <tr>
            <th class="py-2 px-3">SKU</th>
            <th class="py-2 px-3">Insumo Clínico</th>
            <th class="py-2 px-3">Consultorio</th>
            <th class="py-2 px-3 text-right">Stock Actual</th>
            <th class="py-2 px-3 text-right">Mínimo</th>
            <th class="py-2 px-3 text-right">Costo Unit.</th>
            <th class="py-2 px-3 text-right">Estado</th>
          </tr>
        </thead>
        <tbody>${rowsHtml}</tbody>
      </table>
    </div>

    ${recipeHtml}
  </section>`;
}

export default renderInventoryWorkspaceHtml;
