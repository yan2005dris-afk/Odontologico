/**
 * DentistSettlementPanel Component (wi-12 S3, RF-13, RF-14, AGENTS.md §3).
 * Commission rule configurator and fee settlement calculator supporting
 * FIXED_PERCENTAGE, FIXED_AMOUNT, and NET_PERCENTAGE_AFTER_COSTS (deducting lab and RF-14 material costs).
 */

import { escapeHtml, formatCentsCurrency } from "../../lib/session-idle-guard.ts";

export type CommissionRuleType =
  | "FIXED_PERCENTAGE"
  | "FIXED_AMOUNT"
  | "NET_PERCENTAGE_AFTER_COSTS";

export interface DentistSettlementCalculationInput {
  ruleType: CommissionRuleType;
  percentageBps: number; // Basis points: 4000 = 40.00%
  fixedAmountCents: number;
  grossProductionCents: number;
  labCostCents: number;
  materialsCostCents: number; // Automatic deduction from RF-14 recipes
}

export interface DentistSettlementBreakdown {
  ruleType: CommissionRuleType;
  grossProductionCents: number;
  labCostCents: number;
  materialsCostCents: number;
  netBaseCents: number;
  netPayableCents: number;
}

export interface DentistSettlementPanelProps {
  dentistName: string;
  periodLabel: string;
  input: DentistSettlementCalculationInput;
}

/**
 * Calculates dentist fee settlement in integer cents per RF-13 / S3.
 */
export function calculateDentistFeeSettlement(
  input: DentistSettlementCalculationInput
): DentistSettlementBreakdown {
  const gross = Math.max(0, Math.trunc(input.grossProductionCents));
  const lab = Math.max(0, Math.trunc(input.labCostCents));
  const materials = Math.max(0, Math.trunc(input.materialsCostCents));
  const netBaseCents = Math.max(0, gross - lab - materials);

  let netPayableCents = 0;
  if (input.ruleType === "FIXED_PERCENTAGE") {
    netPayableCents = Math.round((gross * input.percentageBps) / 10_000);
  } else if (input.ruleType === "FIXED_AMOUNT") {
    netPayableCents = Math.max(0, Math.trunc(input.fixedAmountCents));
  } else {
    netPayableCents = Math.round((netBaseCents * input.percentageBps) / 10_000);
  }

  return {
    ruleType: input.ruleType,
    grossProductionCents: gross,
    labCostCents: lab,
    materialsCostCents: materials,
    netBaseCents,
    netPayableCents,
  };
}

export function renderDentistSettlementPanelHtml(props: DentistSettlementPanelProps): string {
  const breakdown = calculateDentistFeeSettlement(props.input);

  return `<section class="bg-white rounded-xl border border-slate-200 p-6 space-y-4 tabular-nums" aria-label="Liquidación de Honorarios a Profesionales RF-13">
    <header class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 pb-3">
      <div>
        <h2 class="text-base font-bold text-slate-900">Liquidación de Honorarios Profesionales (RF-13)</h2>
        <p class="text-xs text-slate-600">Profesional: <span class="font-semibold">${escapeHtml(
          props.dentistName
        )}</span> · Período: <span class="font-mono tabular-nums">${escapeHtml(
          props.periodLabel
        )}</span> · Regla: <span class="font-mono font-semibold">${breakdown.ruleType}</span></p>
      </div>
    </header>

    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3 text-xs tabular-nums">
      <div class="p-3 rounded-lg bg-slate-50 border border-slate-200">
        <span class="text-slate-500 block">Producción Bruta</span>
        <span class="font-mono font-bold text-sm tabular-nums text-slate-900">${formatCentsCurrency(
          breakdown.grossProductionCents
        )}</span>
      </div>
      <div class="p-3 rounded-lg bg-slate-50 border border-slate-200">
        <span class="text-slate-500 block">Deducción Laboratorio</span>
        <span class="font-mono font-bold text-sm tabular-nums text-slate-800">-${formatCentsCurrency(
          breakdown.labCostCents
        )}</span>
      </div>
      <div class="p-3 rounded-lg bg-slate-50 border border-slate-200">
        <span class="text-slate-500 block">Deducción Materiales (RF-14)</span>
        <span class="font-mono font-bold text-sm tabular-nums text-slate-800">-${formatCentsCurrency(
          breakdown.materialsCostCents
        )}</span>
      </div>
      <div class="p-3 rounded-lg bg-slate-50 border border-slate-200">
        <span class="text-slate-500 block">Base Neta Liquidable</span>
        <span class="font-mono font-bold text-sm tabular-nums text-slate-900">${formatCentsCurrency(
          breakdown.netBaseCents
        )}</span>
      </div>
      <div class="p-3 rounded-lg bg-blue-50 border border-blue-200">
        <span class="text-blue-800 font-semibold block">Honorario Neto a Liquidar</span>
        <span class="font-mono font-bold text-base tabular-nums text-blue-900">${formatCentsCurrency(
          breakdown.netPayableCents
        )}</span>
      </div>
    </div>
  </section>`;
}

export default renderDentistSettlementPanelHtml;
