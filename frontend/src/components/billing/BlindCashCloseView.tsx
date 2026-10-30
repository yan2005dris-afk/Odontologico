/**
 * BlindCashCloseView Component (wi-12 S2, RN-02, AGENTS.md §3 & §4).
 * Daily cash reconciliation strictly enforcing RN-02:
 * Receptionists NEVER receive or see the system's theoretical cash total before submitting
 * the physical cash count. Only `auditor` and `admin` roles can view the post-close audit discrepancy report.
 */

import {
  escapeHtml,
  formatCentsCurrency,
  type UserRole,
} from "../../lib/session-idle-guard.ts";

export interface BlindCashRegisterSessionDTO {
  id: string;
  openedBy: string;
  openedAt: string;
  status: "OPEN" | "CLOSED";
  // Intentionally excludes theoreticalCashCents per RN-02 contract
}

export interface CashCloseAuditReportDTO {
  theoreticalCashCents: number;
  declaredCashCents: number;
  discrepancyCents: number;
}

export interface BlindCashCloseViewProps {
  cashRegister: BlindCashRegisterSessionDTO;
  userRole: UserRole;
  denominationsCount: Record<string, number>; // key = denomination value in cents (e.g. "10000" for $100.00), value = bill/coin count
  directDeclaredCents?: number;
  confirmedByCashier: boolean;
  submitting?: boolean;
  auditReport?: CashCloseAuditReportDTO;
  onSubmitBlindClose?: (
    declaredCashCents: number,
    denominations: Record<string, number>
  ) => Promise<void>;
}

/**
 * Calculates total physical cash in integer cents from denomination counts (S2, RN-02).
 */
export function calculatePhysicalDenominationTotalCents(
  denominationsCount: Record<string, number>
): number {
  let totalCents = 0;
  for (const [denomCentsStr, count] of Object.entries(denominationsCount)) {
    const denomCents = Number.parseInt(denomCentsStr, 10);
    const qty = Math.trunc(count);
    if (!Number.isNaN(denomCents) && denomCents > 0 && qty > 0) {
      totalCents += denomCents * qty;
    }
  }
  return totalCents;
}

export function validateBlindCashCloseSubmission(input: {
  registerStatus: "OPEN" | "CLOSED";
  declaredCashCents: number;
  confirmedByCashier: boolean;
  submitting?: boolean;
}): {
  valid: boolean;
  errorCode?:
    | "REGISTER_ALREADY_CLOSED"
    | "SUBMISSION_IN_PROGRESS"
    | "NEGATIVE_DECLARED_AMOUNT"
    | "CASHIER_CONFIRMATION_REQUIRED";
} {
  if (input.registerStatus === "CLOSED") {
    return { valid: false, errorCode: "REGISTER_ALREADY_CLOSED" };
  }
  if (input.submitting) {
    return { valid: false, errorCode: "SUBMISSION_IN_PROGRESS" };
  }
  if (input.declaredCashCents < 0) {
    return { valid: false, errorCode: "NEGATIVE_DECLARED_AMOUNT" };
  }
  if (!input.confirmedByCashier) {
    return { valid: false, errorCode: "CASHIER_CONFIRMATION_REQUIRED" };
  }
  return { valid: true };
}

export function renderBlindCashCloseViewHtml(props: BlindCashCloseViewProps): string {
  const denomTotalCents = calculatePhysicalDenominationTotalCents(props.denominationsCount);
  const declaredCashCents =
    props.directDeclaredCents !== undefined ? props.directDeclaredCents : denomTotalCents;

  const validation = validateBlindCashCloseSubmission({
    registerStatus: props.cashRegister.status,
    declaredCashCents,
    confirmedByCashier: props.confirmedByCashier,
    submitting: props.submitting,
  });

  const canViewAuditReport =
    (props.userRole === "auditor" || props.userRole === "admin") &&
    props.cashRegister.status === "CLOSED" &&
    props.auditReport !== undefined;

  const denomRowsHtml = Object.entries(props.denominationsCount)
    .map(([denomCentsStr, qty]) => {
      const denomCents = Number.parseInt(denomCentsStr, 10);
      const lineCents = denomCents * qty;
      return `<tr class="border-b border-slate-100 text-xs">
        <td class="py-2 px-3 font-mono tabular-nums">${formatCentsCurrency(denomCents)}</td>
        <td class="py-2 px-3 font-mono tabular-nums text-right">${qty}</td>
        <td class="py-2 px-3 font-mono font-semibold tabular-nums text-right">${formatCentsCurrency(lineCents)}</td>
      </tr>`;
    })
    .join("");

  const auditPanelHtml =
    canViewAuditReport && props.auditReport
      ? `<section class="mt-6 p-4 rounded-lg bg-slate-900 text-white space-y-2 tabular-nums" aria-label="Reporte de Auditoría de Corte de Caja">
          <h3 class="text-xs font-bold uppercase tracking-wider text-slate-300">Reporte de Auditoría de Corte de Caja (Exclusivo Admin / Auditor)</h3>
          <div class="grid grid-cols-3 gap-4 text-xs pt-1">
            <div>
              <span class="text-slate-400 block">Saldo Teórico del Sistema:</span>
              <span class="font-mono font-bold text-sm tabular-nums">${formatCentsCurrency(
                props.auditReport.theoreticalCashCents
              )}</span>
            </div>
            <div>
              <span class="text-slate-400 block">Efectivo Físico Declarado:</span>
              <span class="font-mono font-bold text-sm tabular-nums">${formatCentsCurrency(
                props.auditReport.declaredCashCents
              )}</span>
            </div>
            <div>
              <span class="text-slate-400 block">Diferencia / Descuadre:</span>
              <span class="font-mono font-bold text-sm tabular-nums ${
                props.auditReport.discrepancyCents === 0 ? "text-emerald-400" : "text-red-400"
              }">${formatCentsCurrency(props.auditReport.discrepancyCents)}</span>
            </div>
          </div>
        </section>`
      : "";

  return `<section class="bg-white rounded-xl border border-slate-200 p-6 space-y-5 tabular-nums" aria-label="Arqueo de Caja Ciego (RN-02)">
    <header class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 pb-4">
      <div>
        <h2 class="text-base font-bold text-slate-900">Arqueo de Caja Ciego (RN-02)</h2>
        <p class="text-xs text-slate-600">Caja <span class="font-mono">${escapeHtml(
          props.cashRegister.id
        )}</span> · Apertura: <span class="font-mono tabular-nums">${escapeHtml(
          props.cashRegister.openedAt
        )}</span> · Estado: <span class="font-semibold">${props.cashRegister.status}</span></p>
      </div>
      <span class="text-xs px-2.5 py-1 rounded bg-slate-100 text-slate-800 font-semibold">
        Protección RN-02 Activa: Saldo esperado oculto durante conteo físico
      </span>
    </header>

    <div class="space-y-3">
      <h3 class="text-xs font-bold uppercase tracking-wider text-slate-500">Desglose de Conteo Físico de Billetes y Monedas</h3>
      <table class="w-full text-left tabular-nums border border-slate-200 rounded-lg overflow-hidden">
        <thead class="bg-slate-50 text-[11px] text-slate-600 border-b border-slate-200">
          <tr>
            <th class="py-1.5 px-3">Denominación</th>
            <th class="py-1.5 px-3 text-right">Cantidad Contada</th>
            <th class="py-1.5 px-3 text-right">Subtotal Físico</th>
          </tr>
        </thead>
        <tbody>${denomRowsHtml}</tbody>
      </table>
    </div>

    <div class="p-4 rounded-lg bg-slate-50 border border-slate-200 flex items-center justify-between">
      <span class="text-xs font-bold text-slate-800">Total Efectivo Físico Declarado por Cajero:</span>
      <span class="text-base font-mono font-bold tabular-nums text-slate-900">${formatCentsCurrency(
        declaredCashCents
      )}</span>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-4 pt-2">
      <label class="flex items-center gap-2 text-xs text-slate-800">
        <input type="checkbox" ${props.confirmedByCashier ? "checked" : ""} ${
          props.cashRegister.status === "CLOSED" ? "disabled" : ""
        } />
        <span>Confirmo que el monto ingresado corresponde al recuento físico real en gaveta (RN-02)</span>
      </label>
      <button type="submit" ${
        !validation.valid ? "disabled" : ""
      } class="min-h-[44px] px-4 py-2 rounded bg-slate-900 text-white text-xs font-semibold disabled:opacity-50">
        Confirmar y Enviar Cierre Ciego
      </button>
    </div>

    ${auditPanelHtml}
  </section>`;
}

export default renderBlindCashCloseViewHtml;
