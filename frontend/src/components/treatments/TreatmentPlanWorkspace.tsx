/**
 * TreatmentPlanWorkspace Component (wi-11 S5, RF-07, RF-08, AGENTS.md §3).
 * Multi-phase treatment plan builder and budget workspace where completed phases auto-collapse
 * to emphasize what's next, supporting partial or total approval and transition to schedulable tasks.
 */

import { escapeHtml, formatCentsCurrency } from "../../lib/session-idle-guard.ts";

export interface TreatmentPlanItemDTO {
  id: string;
  procedureCode: string;
  description: string;
  toothFdi?: string;
  priceCents: number;
  status: "PENDIENTE" | "APROBADO" | "REALIZADO";
}

export interface TreatmentPhaseDTO {
  id: string;
  name: string;
  items: TreatmentPlanItemDTO[];
}

export interface TreatmentPlanDTO {
  id: string;
  patientId: string;
  title: string;
  phases: TreatmentPhaseDTO[];
}

export interface TreatmentPhaseViewModel {
  id: string;
  name: string;
  isCompleted: boolean;
  isCollapsedByDefault: boolean;
  phaseSubtotalCents: number;
  items: TreatmentPlanItemDTO[];
}

export interface TreatmentPlanViewModel {
  planId: string;
  totalPlanCents: number;
  pendingSubtotalCents: number;
  selectedSubtotalCents: number;
  approvalMode: "TOTAL" | "PARTIAL" | "NONE";
  schedulableTaskIds: string[];
  phases: TreatmentPhaseViewModel[];
}

export interface TreatmentPlanWorkspaceProps {
  patientId: string;
  plan: TreatmentPlanDTO;
  selectedItemIds?: string[];
  onApprove?: (mode: "TOTAL" | "PARTIAL", itemIds: string[]) => Promise<void>;
}

/**
 * Builds the progressive-disclosure ViewModel for a multi-phase treatment plan (S5, RF-07, RF-08).
 * Completed phases (where all items are REALIZADO) collapse automatically per AGENTS.md §3.
 */
export function buildTreatmentPlanViewModel(
  plan: TreatmentPlanDTO,
  selectedItemIds: string[] = []
): TreatmentPlanViewModel {
  const selectedSet = new Set(selectedItemIds);
  let totalPlanCents = 0;
  let pendingSubtotalCents = 0;
  let selectedSubtotalCents = 0;
  const pendingItemIds: string[] = [];
  const schedulableTaskIds: string[] = [];

  const phases: TreatmentPhaseViewModel[] = plan.phases.map((phase) => {
    const isCompleted =
      phase.items.length > 0 && phase.items.every((item) => item.status === "REALIZADO");
    let phaseSubtotalCents = 0;

    for (const item of phase.items) {
      totalPlanCents += item.priceCents;
      phaseSubtotalCents += item.priceCents;

      if (item.status !== "REALIZADO") {
        pendingSubtotalCents += item.priceCents;
        pendingItemIds.push(item.id);
        if (selectedSet.has(item.id)) {
          selectedSubtotalCents += item.priceCents;
          schedulableTaskIds.push(item.id);
        }
      }
    }

    return {
      id: phase.id,
      name: phase.name,
      isCompleted,
      isCollapsedByDefault: isCompleted,
      phaseSubtotalCents,
      items: phase.items,
    };
  });

  let approvalMode: "TOTAL" | "PARTIAL" | "NONE" = "NONE";
  if (schedulableTaskIds.length > 0) {
    approvalMode =
      schedulableTaskIds.length === pendingItemIds.length ? "TOTAL" : "PARTIAL";
  }

  return {
    planId: plan.id,
    totalPlanCents,
    pendingSubtotalCents,
    selectedSubtotalCents,
    approvalMode,
    schedulableTaskIds,
    phases,
  };
}

export function renderTreatmentPlanWorkspaceHtml(props: TreatmentPlanWorkspaceProps): string {
  const vm = buildTreatmentPlanViewModel(props.plan, props.selectedItemIds ?? []);

  const phasesHtml = vm.phases
    .map((phase) => {
      if (phase.isCollapsedByDefault) {
        return `<details class="rounded-lg border border-slate-200 bg-slate-50 p-4" data-phase-id="${escapeHtml(phase.id)}">
          <summary class="cursor-pointer flex items-center justify-between text-xs font-semibold text-slate-700">
            <span>${escapeHtml(phase.name)} — Fase Completada (Colapsada)</span>
            <span class="font-mono tabular-nums">${formatCentsCurrency(phase.phaseSubtotalCents)}</span>
          </summary>
        </details>`;
      }

      const itemsRows = phase.items
        .map(
          (item) => `<tr class="border-b border-slate-100 text-xs">
            <td class="py-2 px-3 font-mono tabular-nums">${escapeHtml(item.procedureCode)}</td>
            <td class="py-2 px-3 text-slate-900">${escapeHtml(item.description)}</td>
            <td class="py-2 px-3 font-mono tabular-nums">${escapeHtml(item.toothFdi ?? "General")}</td>
            <td class="py-2 px-3 font-mono font-semibold tabular-nums text-right">${formatCentsCurrency(item.priceCents)}</td>
            <td class="py-2 px-3 text-right"><span class="px-2 py-0.5 rounded bg-slate-100 text-slate-800 font-medium">${item.status}</span></td>
          </tr>`
        )
        .join("");

      return `<div class="rounded-lg border border-slate-200 bg-white p-4 space-y-3" data-phase-id="${escapeHtml(phase.id)}">
        <div class="flex items-center justify-between border-b border-slate-200 pb-2">
          <h3 class="text-sm font-bold text-slate-900">${escapeHtml(phase.name)}</h3>
          <span class="text-xs font-mono font-bold tabular-nums text-slate-800">${formatCentsCurrency(phase.phaseSubtotalCents)}</span>
        </div>
        <table class="w-full text-left tabular-nums">
          <thead>
            <tr class="text-[11px] text-slate-500 border-b border-slate-200">
              <th class="py-1.5 px-3">Código</th>
              <th class="py-1.5 px-3">Procedimiento</th>
              <th class="py-1.5 px-3">Pieza (FDI)</th>
              <th class="py-1.5 px-3 text-right">Importe</th>
              <th class="py-1.5 px-3 text-right">Estado</th>
            </tr>
          </thead>
          <tbody>${itemsRows}</tbody>
        </table>
      </div>`;
    })
    .join("\n");

  const approvalLabel =
    vm.approvalMode === "TOTAL"
      ? "Aprobar Selección (TOTAL)"
      : "Aprobar Selección (PARCIAL)";

  return `<section class="bg-white rounded-xl border border-slate-200 p-6 space-y-5 tabular-nums" aria-label="Presupuesto y Plan de Tratamiento Multinivel">
    <header class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 pb-4">
      <div>
        <h2 class="text-base font-bold text-slate-900">${escapeHtml(props.plan.title)}</h2>
        <p class="text-xs text-slate-600">Total Presupuesto: <span class="font-mono font-bold tabular-nums">${formatCentsCurrency(vm.totalPlanCents)}</span> · Selección a aprobar: <span class="font-mono font-bold tabular-nums text-blue-700">${formatCentsCurrency(vm.selectedSubtotalCents)}</span></p>
      </div>
      <button type="button" ${
        vm.approvalMode === "NONE" ? "disabled" : ""
      } class="min-h-[44px] px-4 py-2 rounded bg-blue-600 text-white text-xs font-semibold disabled:opacity-50">
        ${approvalLabel} → Generar ${vm.schedulableTaskIds.length} Tarea(s) Agendable(s)
      </button>
    </header>
    <div class="space-y-3">
      ${phasesHtml}
    </div>
  </section>`;
}

export default renderTreatmentPlanWorkspaceHtml;
