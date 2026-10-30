/**
 * PeriodontogramGrid Component (wi-11 S3, RF-04, AGENTS.md §3).
 * 6 probing sites per tooth (MB, B, DB, ML, L, DL), gingival margin, automatic Clinical Attachment Level
 * calculation (NIC = PD - GM), bleeding on probing (BOP), furcation, mobility, and mandatory `tabular-nums`.
 */

import { escapeHtml } from "../../lib/session-idle-guard.ts";

export type SixSiteTuple<T> = [T, T, T, T, T, T];

export const PERIODONTAL_SITES_6 = ["MB", "B", "DB", "ML", "L", "DL"] as const;

export interface PeriodontogramToothDTO {
  fdiTooth: string;
  probingDepthMm: SixSiteTuple<number>; // PD
  gingivalMarginMm: SixSiteTuple<number>; // GM (negative for recession, positive for hyperplasia)
  bleedingOnProbing: SixSiteTuple<boolean>; // BOP
  furcationGrade: 0 | 1 | 2 | 3;
  mobilityGrade: 0 | 1 | 2 | 3;
}

export interface ToothPeriodontalMetrics {
  fdiTooth: string;
  clinicalAttachmentLevelMm: SixSiteTuple<number>; // NIC = PD - GM
  hasPathologicalPocket: boolean; // Any PD >= 4mm
  bopCount: number;
}

/**
 * Calculates Clinical Attachment Level (NIC = Probing Depth - Gingival Margin) across all 6 sites per RF-04 / S3.
 */
export function calculateClinicalAttachmentLevel(
  probingDepthMm: SixSiteTuple<number>,
  gingivalMarginMm: SixSiteTuple<number>
): SixSiteTuple<number> {
  return [
    probingDepthMm[0] - gingivalMarginMm[0],
    probingDepthMm[1] - gingivalMarginMm[1],
    probingDepthMm[2] - gingivalMarginMm[2],
    probingDepthMm[3] - gingivalMarginMm[3],
    probingDepthMm[4] - gingivalMarginMm[4],
    probingDepthMm[5] - gingivalMarginMm[5],
  ];
}

export function computeToothPeriodontalMetrics(
  tooth: PeriodontogramToothDTO
): ToothPeriodontalMetrics {
  const clinicalAttachmentLevelMm = calculateClinicalAttachmentLevel(
    tooth.probingDepthMm,
    tooth.gingivalMarginMm
  );
  const hasPathologicalPocket = tooth.probingDepthMm.some((pd) => pd >= 4);
  const bopCount = tooth.bleedingOnProbing.filter(Boolean).length;

  return {
    fdiTooth: tooth.fdiTooth,
    clinicalAttachmentLevelMm,
    hasPathologicalPocket,
    bopCount,
  };
}

export function renderPeriodontogramGridHtml(props: {
  patientId: string;
  teethData: PeriodontogramToothDTO[];
}): string {
  const rowsHtml = props.teethData
    .map((tooth) => {
      const metrics = computeToothPeriodontalMetrics(tooth);
      const nicText = metrics.clinicalAttachmentLevelMm.join(", ");
      const pdText = tooth.probingDepthMm.join(", ");
      const gmText = tooth.gingivalMarginMm.join(", ");

      return `<tr class="border-b border-slate-200" data-fdi-tooth="${escapeHtml(tooth.fdiTooth)}">
        <td class="px-3 py-2 font-mono font-bold tabular-nums text-slate-900">${escapeHtml(tooth.fdiTooth)}</td>
        <td class="px-3 py-2 font-mono tabular-nums text-slate-800">PD: ${pdText}</td>
        <td class="px-3 py-2 font-mono tabular-nums text-slate-800">GM: ${gmText}</td>
        <td class="px-3 py-2 font-mono font-bold tabular-nums ${metrics.hasPathologicalPocket ? "text-red-600" : "text-slate-900"}">NIC: ${nicText}</td>
        <td class="px-3 py-2 font-mono tabular-nums text-slate-700">BOP: ${metrics.bopCount}/6</td>
        <td class="px-3 py-2 font-mono tabular-nums text-slate-700">F:${tooth.furcationGrade} · M:${tooth.mobilityGrade}</td>
      </tr>`;
    })
    .join("\n");

  return `<section class="bg-white rounded-xl border border-slate-200 p-6 space-y-4 tabular-nums" aria-label="Periodontograma Digital RF-04">
    <header class="border-b border-slate-200 pb-3">
      <h2 class="text-base font-bold text-slate-900">Periodontograma Digital (6 Sitios por Diente: MB, B, DB, ML, L, DL)</h2>
      <p class="text-xs text-slate-600">Cálculo automático en vivo del Nivel de Inserción Clínica (NIC = PD - GM) con tipografía tabular-nums</p>
    </header>
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs tabular-nums">
        <thead class="bg-slate-50 border-b border-slate-200 text-slate-700">
          <tr>
            <th class="px-3 py-2">Pieza (FDI)</th>
            <th class="px-3 py-2">Profundidad Sondaje (PD mm)</th>
            <th class="px-3 py-2">Margen Gingival (GM mm)</th>
            <th class="px-3 py-2">Nivel Inserción (NIC mm)</th>
            <th class="px-3 py-2">Sangrado (BOP)</th>
            <th class="px-3 py-2">Furca / Movilidad</th>
          </tr>
        </thead>
        <tbody>
          ${rowsHtml}
        </tbody>
      </table>
    </div>
  </section>`;
}

export default renderPeriodontogramGridHtml;
