import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  calculatePhysicalDenominationTotalCents,
  validateBlindCashCloseSubmission,
  renderBlindCashCloseViewHtml,
} from "./BlindCashCloseView.tsx";

describe("wi-12 S2 — Blind Cash Close UI Strictly Hiding System Theoretical Balance (RN-02)", () => {
  it("calculates declared physical cash strictly from bill/coin denominations in integer cents (S2)", () => {
    const declaredCents = calculatePhysicalDenominationTotalCents({
      "10000": 5, // 5 x $100.00 = $500.00 (50000 cents)
      "5000": 4,  // 4 x $50.00 = $200.00 (20000 cents)
      "1000": 3,  // 3 x $10.00 = $30.00 (3000 cents)
    });
    assert.equal(declaredCents, 73000);
  });

  it("NEVER renders or exposes any theoretical system balance in the DOM for receptionist when register is OPEN (RN-02 adversarial test)", () => {
    const html = renderBlindCashCloseViewHtml({
      cashRegister: {
        id: "reg-morning-01",
        openedBy: "Recepcionista Carla",
        openedAt: "2026-10-30T08:00:00Z",
        status: "OPEN",
        // Even if an attacker or buggy caller passes an unexpected property at runtime, RN-02 must strip/ignore it
        ...({ theoreticalCashCents: 999999_00 } as Record<string, unknown>),
      },
      userRole: "receptionist",
      denominationsCount: { "10000": 12 },
      confirmedByCashier: true,
    });

    assert.doesNotMatch(html, /9,999\.99/);
    assert.doesNotMatch(html, /999999/);
    assert.doesNotMatch(html, /Saldo Teórico del Sistema:/);
    assert.match(html, /Arqueo de Caja Ciego \(RN-02\)/);
    assert.match(html, /tabular-nums/);
  });

  it("requires explicit cashier confirmation before submitting blind close and allows audit discrepancy view ONLY for auditor or admin (S2, RN-02)", () => {
    const unconfirmed = validateBlindCashCloseSubmission({
      registerStatus: "OPEN",
      declaredCashCents: 12000_00,
      confirmedByCashier: false,
      submitting: false,
    });
    assert.equal(unconfirmed.valid, false);
    assert.equal(unconfirmed.errorCode, "CASHIER_CONFIRMATION_REQUIRED");

    const auditorHtml = renderBlindCashCloseViewHtml({
      cashRegister: {
        id: "reg-morning-01",
        openedBy: "Recepcionista Carla",
        openedAt: "2026-10-30T08:00:00Z",
        status: "CLOSED",
      },
      userRole: "auditor",
      denominationsCount: { "10000": 12 },
      confirmedByCashier: true,
      auditReport: {
        theoreticalCashCents: 125000,
        declaredCashCents: 120000,
        discrepancyCents: -5000,
      },
    });
    assert.match(auditorHtml, /Reporte de Auditoría de Corte de Caja/);
    assert.match(auditorHtml, /-\$50\.00/);
  });
});
