import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  evaluatePosCheckoutBalance,
  renderPosCheckoutSheetHtml,
  type InvoiceLineItemInput,
  type PaymentTenderLineInput,
} from "./PosCheckoutSheet.tsx";

describe("wi-12 S1 — Mixed-Payment POS Checkout & Exact Sum Validation (RF-12, AGENTS.md §3)", () => {
  const sampleItems: InvoiceLineItemInput[] = [
    {
      procedureCode: "END-14",
      description: "Endodoncia Birradicular Pieza 14",
      quantity: 1,
      unitPriceCents: 18000_00,
    },
    {
      procedureCode: "RES-16",
      description: "Restauración Resina Compuesta Pieza 16",
      quantity: 1,
      unitPriceCents: 9500_00,
    },
  ];

  it("supports mixed payment methods (CASH, CREDIT_CARD, DEBIT_CARD, BANK_TRANSFER, DENTAL_INSURANCE) and disables issue button until sum === invoiceTotal (S1)", () => {
    const partialPayments: PaymentTenderLineInput[] = [
      { method: "CASH", amountCents: 10000_00 },
      { method: "CREDIT_CARD", amountCents: 10000_00 },
    ];
    const partialEval = evaluatePosCheckoutBalance(sampleItems, partialPayments);
    assert.equal(partialEval.invoiceTotalCents, 27500_00);
    assert.equal(partialEval.paidTotalCents, 20000_00);
    assert.equal(partialEval.remainingCents, 7500_00);
    assert.equal(partialEval.canIssueFiscalReceipt, false);

    const exactMixedPayments: PaymentTenderLineInput[] = [
      { method: "CASH", amountCents: 5000_00 },
      { method: "CREDIT_CARD", amountCents: 10000_00 },
      { method: "DEBIT_CARD", amountCents: 2500_00 },
      { method: "BANK_TRANSFER", amountCents: 4000_00 },
      { method: "DENTAL_INSURANCE", amountCents: 6000_00, referenceCode: "OSDE-8821" },
    ];
    const exactEval = evaluatePosCheckoutBalance(sampleItems, exactMixedPayments);
    assert.equal(exactEval.invoiceTotalCents, 27500_00);
    assert.equal(exactEval.paidTotalCents, 27500_00);
    assert.equal(exactEval.remainingCents, 0);
    assert.equal(exactEval.canIssueFiscalReceipt, true);

    const overpaidEval = evaluatePosCheckoutBalance(sampleItems, [
      { method: "CASH", amountCents: 30000_00 },
    ]);
    assert.equal(overpaidEval.remainingCents, -2500_00);
    assert.equal(overpaidEval.canIssueFiscalReceipt, false);
  });

  it("renders PosCheckoutSheet with tabular-nums and disables 'Emitir Comprobante Fiscal' when balance != 0 (S1)", () => {
    const htmlDisabled = renderPosCheckoutSheetHtml({
      isOpen: true,
      patientId: "pat-001",
      cashRegisterId: "reg-01",
      items: sampleItems,
      payments: [{ method: "CASH", amountCents: 15000_00 }],
    });
    assert.match(htmlDisabled, /tabular-nums/);
    assert.match(htmlDisabled, /Emitir Comprobante Fiscal/);
    assert.match(htmlDisabled, /disabled/);
  });
});
