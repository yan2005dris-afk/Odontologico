import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  calculateDentistFeeSettlement,
  renderDentistSettlementPanelHtml,
} from "./DentistSettlementPanel.tsx";

describe("wi-12 S3 — Dentist Fee & Commission Settlement Simulator (RF-13, RF-14, AGENTS.md §3)", () => {
  it("calculates FIXED_PERCENTAGE, FIXED_AMOUNT, and NET_PERCENTAGE_AFTER_COSTS (deducting lab & RF-14 materials) accurately (S3)", () => {
    const fixedPct = calculateDentistFeeSettlement({
      ruleType: "FIXED_PERCENTAGE",
      percentageBps: 4000, // 40.00%
      fixedAmountCents: 0,
      grossProductionCents: 100000_00,
      labCostCents: 15000_00,
      materialsCostCents: 5000_00,
    });
    assert.equal(fixedPct.netPayableCents, 40000_00);

    const fixedAmt = calculateDentistFeeSettlement({
      ruleType: "FIXED_AMOUNT",
      percentageBps: 0,
      fixedAmountCents: 25000_00,
      grossProductionCents: 100000_00,
      labCostCents: 15000_00,
      materialsCostCents: 5000_00,
    });
    assert.equal(fixedAmt.netPayableCents, 25000_00);

    const netAfterCosts = calculateDentistFeeSettlement({
      ruleType: "NET_PERCENTAGE_AFTER_COSTS",
      percentageBps: 5000, // 50.00% after deducting lab + materials
      fixedAmountCents: 0,
      grossProductionCents: 100000_00,
      labCostCents: 15000_00,
      materialsCostCents: 5000_00,
    });
    assert.equal(netAfterCosts.netBaseCents, 80000_00);
    assert.equal(netAfterCosts.netPayableCents, 40000_00);
  });

  it("renders settlement breakdown with tabular-nums (S3)", () => {
    const html = renderDentistSettlementPanelHtml({
      dentistName: "Dra. Valeria Méndez",
      periodLabel: "2026-10",
      input: {
        ruleType: "NET_PERCENTAGE_AFTER_COSTS",
        percentageBps: 4500,
        fixedAmountCents: 0,
        grossProductionCents: 200000_00,
        labCostCents: 30000_00,
        materialsCostCents: 10000_00,
      },
    });
    assert.match(html, /tabular-nums/);
    assert.match(html, /NET_PERCENTAGE_AFTER_COSTS/);
    assert.match(html, /\$72,000\.00/);
  });
});
