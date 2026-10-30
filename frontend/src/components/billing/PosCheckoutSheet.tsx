/**
 * PosCheckoutSheet Component (wi-12 S1, RF-12, AGENTS.md §3).
 * Slide-over Sheet and checkout view for electronic fiscal invoicing with mixed-payment builder
 * (CASH, CREDIT_CARD, DEBIT_CARD, BANK_TRANSFER, DENTAL_INSURANCE), integer-cents exact balance validation,
 * and mandatory `tabular-nums`.
 */

import { escapeHtml, formatCentsCurrency } from "../../lib/session-idle-guard.ts";
import { renderSlideOverSheetHtml } from "../shell/SlideOverSheet.tsx";

export type PaymentTenderMethod =
  | "CASH"
  | "CREDIT_CARD"
  | "DEBIT_CARD"
  | "BANK_TRANSFER"
  | "DENTAL_INSURANCE";

export const PAYMENT_TENDER_LABELS: Record<PaymentTenderMethod, string> = {
  CASH: "Efectivo",
  CREDIT_CARD: "Tarjeta Crédito",
  DEBIT_CARD: "Tarjeta Débito",
  BANK_TRANSFER: "Transferencia Bancaria",
  DENTAL_INSURANCE: "Seguro / Convenio",
};

export interface InvoiceLineItemInput {
  procedureCode: string;
  description: string;
  quantity: number;
  unitPriceCents: number;
}

export interface PaymentTenderLineInput {
  method: PaymentTenderMethod;
  amountCents: number;
  referenceCode?: string;
}

export interface CreateInvoiceRequest {
  patientId: string;
  cashRegisterId: string;
  items: InvoiceLineItemInput[];
  payments: PaymentTenderLineInput[];
}

export interface InvoiceResponse {
  invoiceId: string;
  fiscalNumber: string;
  issuedAtIso: string;
  totalCents: number;
}

export interface PosCheckoutBalanceEvaluation {
  invoiceTotalCents: number;
  paidTotalCents: number;
  remainingCents: number;
  canIssueFiscalReceipt: boolean;
}

export interface PosCheckoutSheetProps {
  isOpen: boolean;
  patientId: string;
  cashRegisterId: string;
  items: InvoiceLineItemInput[];
  payments: PaymentTenderLineInput[];
  onSubmitInvoice?: (payload: CreateInvoiceRequest) => Promise<InvoiceResponse>;
}

/**
 * Evaluates exact mixed-payment sum in integer cents (S1, RF-12).
 * Fiscal receipt emission is strictly enabled only when invoiceTotalCents > 0 and sum(payments) === invoiceTotalCents.
 */
export function evaluatePosCheckoutBalance(
  items: InvoiceLineItemInput[],
  payments: PaymentTenderLineInput[]
): PosCheckoutBalanceEvaluation {
  const invoiceTotalCents = items.reduce(
    (acc, item) => acc + Math.trunc(item.quantity) * Math.trunc(item.unitPriceCents),
    0
  );
  const paidTotalCents = payments.reduce((acc, p) => acc + Math.trunc(p.amountCents), 0);
  const remainingCents = invoiceTotalCents - paidTotalCents;
  const canIssueFiscalReceipt =
    invoiceTotalCents > 0 &&
    payments.length > 0 &&
    payments.every((p) => p.amountCents > 0) &&
    remainingCents === 0;

  return {
    invoiceTotalCents,
    paidTotalCents,
    remainingCents,
    canIssueFiscalReceipt,
  };
}

export function renderPosCheckoutSheetHtml(props: PosCheckoutSheetProps): string {
  const balance = evaluatePosCheckoutBalance(props.items, props.payments);

  const itemsHtml = props.items
    .map(
      (item) => `<tr class="border-b border-slate-100 text-xs">
        <td class="py-2 px-3 font-mono tabular-nums">${escapeHtml(item.procedureCode)}</td>
        <td class="py-2 px-3 text-slate-900">${escapeHtml(item.description)}</td>
        <td class="py-2 px-3 font-mono tabular-nums text-right">${item.quantity}</td>
        <td class="py-2 px-3 font-mono font-semibold tabular-nums text-right">${formatCentsCurrency(
          item.quantity * item.unitPriceCents
        )}</td>
      </tr>`
    )
    .join("");

  const paymentsHtml = props.payments
    .map(
      (pay) => `<li class="flex items-center justify-between py-2 px-3 rounded bg-slate-50 border border-slate-200 text-xs">
        <span class="font-medium text-slate-800">${escapeHtml(PAYMENT_TENDER_LABELS[pay.method])}${
          pay.referenceCode ? ` · Ref: ${escapeHtml(pay.referenceCode)}` : ""
        }</span>
        <span class="font-mono font-bold tabular-nums text-slate-900">${formatCentsCurrency(pay.amountCents)}</span>
      </li>`
    )
    .join("");

  const bodyHtml = `<div class="space-y-5 tabular-nums">
    <div>
      <h3 class="text-xs font-bold uppercase tracking-wider text-slate-500 mb-2">Conceptos a Facturar</h3>
      <table class="w-full text-left tabular-nums border border-slate-200 rounded-lg overflow-hidden">
        <thead class="bg-slate-50 text-[11px] text-slate-600 border-b border-slate-200">
          <tr>
            <th class="py-1.5 px-3">Código</th>
            <th class="py-1.5 px-3">Descripción</th>
            <th class="py-1.5 px-3 text-right">Cant.</th>
            <th class="py-1.5 px-3 text-right">Subtotal</th>
          </tr>
        </thead>
        <tbody>${itemsHtml}</tbody>
      </table>
    </div>

    <div>
      <h3 class="text-xs font-bold uppercase tracking-wider text-slate-500 mb-2">Pagos Mixtos Registrados (RF-12)</h3>
      <ul class="space-y-2">${paymentsHtml}</ul>
    </div>

    <div class="p-4 rounded-lg bg-slate-50 border border-slate-200 space-y-1.5 text-xs">
      <div class="flex justify-between">
        <span class="text-slate-600">Total Comprobante:</span>
        <span class="font-mono font-bold tabular-nums text-slate-900">${formatCentsCurrency(balance.invoiceTotalCents)}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-slate-600">Suma de Pagos Mixtos:</span>
        <span class="font-mono font-bold tabular-nums text-blue-700">${formatCentsCurrency(balance.paidTotalCents)}</span>
      </div>
      <div class="flex justify-between border-t border-slate-200 pt-1.5">
        <span class="font-semibold text-slate-800">Remanente por Cubrir:</span>
        <span class="font-mono font-bold tabular-nums ${
          balance.remainingCents === 0 ? "text-emerald-700" : "text-red-600"
        }">${formatCentsCurrency(balance.remainingCents)}</span>
      </div>
    </div>
  </div>`;

  const footerHtml = `<button type="submit" ${
    !balance.canIssueFiscalReceipt ? "disabled" : ""
  } class="min-h-[44px] px-4 py-2 rounded bg-slate-900 text-white text-xs font-semibold disabled:opacity-50">
    Emitir Comprobante Fiscal
  </button>`;

  return renderSlideOverSheetHtml({
    id: `pos-checkout-${props.cashRegisterId}`,
    title: "Punto de Venta (POS) y Facturación Electrónica (RF-12)",
    description: "Constructor de pagos mixtos con validación exacta en centavos.",
    isOpen: props.isOpen,
    widthClass: "max-w-xl",
    bodyHtml,
    footerHtml,
  });
}

export default renderPosCheckoutSheetHtml;
