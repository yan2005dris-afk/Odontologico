/**
 * Zero-dependency Node built-in test runner suite for SIGO-Dental Frontend
 * covering all pure business & UI invariants across wi-10, wi-11, and wi-12
 * (RF-01..RF-14, RN-01..RN-03, RNF-01, RNF-05 15-min timeout, RNF-07, AGENTS.md §3).
 *
 * Execute directly with:
 *   node --test frontend/tests/domain-invariants.test.mjs
 */

import { registerHooks, stripTypeScriptTypes } from "node:module";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, it } from "node:test";
import assert from "node:assert/strict";

registerHooks({
  load(url, context, nextLoad) {
    if (url.endsWith(".tsx")) {
      const source = readFileSync(fileURLToPath(url), "utf8");
      const stripped = stripTypeScriptTypes(source, { mode: "strip", sourceUrl: url });
      return {
        format: "module",
        shortCircuit: true,
        source: stripped,
      };
    }
    return nextLoad(url, context);
  },
});

const {
  DEFAULT_IDLE_TIMEOUT_MS,
  createSessionIdleGuard,
  validateTotpCode,
  getAllowedNavigationModules,
} = await import("../src/lib/session-idle-guard.ts");

const {
  validatePatientRegistration,
} = await import("../src/components/patients/PatientRegistrationForm.tsx");

const {
  renderStickyPatientHeaderHtml,
} = await import("../src/components/patients/StickyPatientHeader.tsx");

const {
  filterAppointments,
  detectChairOverlapConflict,
  canTransitionAppointmentStatus,
} = await import("../src/components/agenda/MultiChairCalendar.tsx");

const {
  ALL_52_FDI_TEETH,
  FINDING_CATALOG_COLORS,
  fdiToUniversal,
  universalToFdi,
  canMutateOdontogramFinding,
  renderOdontogramCanvasHtml,
} = await import("../src/components/clinical/OdontogramCanvas.tsx");

const {
  calculateClinicalAttachmentLevel,
  renderPeriodontogramGridHtml,
} = await import("../src/components/clinical/PeriodontogramGrid.tsx");

const {
  isSoepNoteLockedByWindow,
  validateSoepAddendumInput,
  renderSoepEvolutionTimelineHtml,
} = await import("../src/components/clinical/SoepEvolutionTimeline.tsx");

const {
  validateInformedConsentSubmission,
} = await import("../src/components/clinical/InformedConsentSheet.tsx");

const {
  buildTreatmentPlanViewModel,
} = await import("../src/components/treatments/TreatmentPlanWorkspace.tsx");

const {
  evaluatePosCheckoutBalance,
} = await import("../src/components/billing/PosCheckoutSheet.tsx");

const {
  calculatePhysicalDenominationTotalCents,
  validateBlindCashCloseSubmission,
  renderBlindCashCloseViewHtml,
} = await import("../src/components/billing/BlindCashCloseView.tsx");

const {
  calculateDentistFeeSettlement,
} = await import("../src/components/billing/DentistSettlementPanel.tsx");

const {
  evaluateInventoryStockAlerts,
  simulateProcedureRecipeDeduction,
} = await import("../src/components/inventory/InventoryWorkspace.tsx");

describe("SIGO-Dental Frontend Domain & UI Invariants (wi-10, wi-11, wi-12)", () => {
  it("wi-10 S1 / RNF-05: enforces 15-minute (900,000 ms) idle session timeout, TOTP MFA, and RBAC sidebar filtering", () => {
    assert.equal(DEFAULT_IDLE_TIMEOUT_MS, 900_000);
    let now = 0;
    let lockedCount = 0;
    const guard = createSessionIdleGuard({
      nowFn: () => now,
      onIdleTimeout: () => {
        lockedCount += 1;
      },
    });
    guard.start();
    now = 899_999;
    assert.equal(guard.checkIdle(), false);
    now = 900_000;
    assert.equal(guard.checkIdle(), true);
    assert.equal(lockedCount, 1);

    assert.equal(validateTotpCode("123456").valid, true);
    assert.equal(validateTotpCode("12345").valid, false);
    assert.deepEqual(
      getAllowedNavigationModules("dentist").map((m) => m.id),
      ["agenda", "patients", "reports"]
    );
  });

  it("wi-10 S2 & S3 / RF-01 & RF-02: enforces Legal Guardian on minors (<18) and renders StickyPatientHeader critical alerts", () => {
    const minorMissingGuardian = validatePatientRegistration(
      {
        documentType: "DNI",
        documentNumber: "55111222",
        fullName: "Lucas Gómez",
        birthDate: "2015-05-10",
        phone: "+5491100001111",
        criticalAlerts: [],
      },
      "2026-10-30"
    );
    assert.equal(minorMissingGuardian.valid, false);
    assert.equal(minorMissingGuardian.errorCode, "LEGAL_GUARDIAN_REQUIRED_FOR_MINOR");

    const headerHtml = renderStickyPatientHeaderHtml({
      id: "pat-01",
      hcNumber: "HC-001",
      fullName: "Ana María Torres",
      age: 62,
      isMinor: false,
      criticalAlerts: [{ code: "PENICILLIN", label: "Alergia a Penicilina", severity: "CRITICAL" }],
    });
    assert.match(headerHtml, /sticky top-0/);
    assert.match(headerHtml, /role="alert"/);
    assert.match(headerHtml, /Alergia a Penicilina/);
  });

  it("wi-10 S4 & S5 / RF-09, RF-10 & RN-03: prevents dental chair overlap even across different dentists and enforces 7-state graph", () => {
    const existing = [
      {
        id: "apt-1",
        patientId: "pat-01",
        patientName: "Juan Pérez",
        chairId: "SILLON-1",
        chairLabel: "Sillón 1",
        dentistId: "DENT-1",
        dentistName: "Dr. Vega",
        date: "2026-10-30",
        startTime: "09:00",
        endTime: "10:00",
        status: "CONFIRMADO",
        procedureSummary: "Endodoncia",
      },
    ];
    assert.equal(
      filterAppointments(existing, {
        view: "day",
        date: "2026-10-30",
        chairId: "SILLON-1",
      }).length,
      1
    );

    const overlap = detectChairOverlapConflict(existing, {
      chairId: "SILLON-1",
      dentistId: "DENT-2",
      date: "2026-10-30",
      startTime: "09:30",
      endTime: "10:30",
    });
    assert.equal(overlap.hasConflict, true);
    assert.equal(overlap.errorCode, "CHAIR_OVERLAP_CONFLICT");

    assert.equal(canTransitionAppointmentStatus("PROGRAMADO", "CONFIRMADO"), true);
    assert.equal(canTransitionAppointmentStatus("ATENDIDO", "CANCELADO"), false);
  });

  it("wi-11 S1 & S2 / RF-03, RN-01 & RNF-01: renders 52 teeth in <100ms, toggles FDI<->Universal, and locks REALIZADO findings & 24h SOEP notes", () => {
    assert.equal(ALL_52_FDI_TEETH.length, 52);
    assert.equal(fdiToUniversal("16"), "3");
    assert.equal(universalToFdi("A"), "55");
    assert.equal(FINDING_CATALOG_COLORS.CARIES, "#DC2626");
    assert.equal(FINDING_CATALOG_COLORS.EXISTING_RESTORATION, "#2563EB");
    assert.equal(
      canMutateOdontogramFinding({
        id: "f-1",
        fdiTooth: "16",
        surface: "OCLUSAL_INCISAL",
        findingCode: "CARIES",
        status: "REALIZADO",
      }),
      false
    );

    const startMs = performance.now();
    const odontogramHtml = renderOdontogramCanvasHtml({
      patientId: "pat-01",
      notation: "FDI",
      findings: [],
    });
    const elapsedMs = performance.now() - startMs;
    assert.ok(elapsedMs < 100, `Expected render <100ms, got ${elapsedMs}ms`);
    assert.match(odontogramHtml, /#FFFFFF/);

    assert.equal(
      isSoepNoteLockedByWindow("2026-10-28T10:00:00Z", "2026-10-29T10:00:01Z"),
      true
    );
    assert.equal(
      validateSoepAddendumInput({
        noteId: "n-1",
        content: "Adenda firmada por omisión involuntaria",
        confirmSignature: true,
      }).valid,
      true
    );
    const soepHtml = renderSoepEvolutionTimelineHtml({
      patientId: "pat-01",
      notes: [
        {
          id: "n-1",
          patientId: "pat-01",
          authorName: "Dr. Vega",
          createdAtIso: "2026-10-28T08:00:00Z",
          isLocked: true,
          subjective: "S",
          objective: "O",
          assessment: "E",
          plan: "P",
          addenda: [],
        },
      ],
    });
    assert.match(soepHtml, /Agregar Adenda Firmada/);
  });

  it("wi-11 S3, S4 & S5 / RF-04, RF-06, RF-07 & RF-08: calculates 6-site NIC = PD - GM, validates biometric/OTP consent, and auto-collapses completed phases", () => {
    const nic = calculateClinicalAttachmentLevel([3, 2, 4, 3, 2, 3], [0, -1, -1, 0, 0, -1]);
    assert.deepEqual(nic, [3, 3, 5, 3, 2, 4]);
    const perioHtml = renderPeriodontogramGridHtml({
      patientId: "pat-01",
      teethData: [
        {
          fdiTooth: "16",
          probingDepthMm: [3, 2, 4, 3, 2, 3],
          gingivalMarginMm: [0, -1, -1, 0, 0, -1],
          bleedingOnProbing: [false, false, true, false, false, false],
          furcationGrade: 0,
          mobilityGrade: 0,
        },
      ],
    });
    assert.match(perioHtml, /tabular-nums/);
    assert.match(perioHtml, /NIC: 3, 3, 5, 3, 2, 4/);

    assert.equal(
      validateInformedConsentSubmission({
        consentId: "c-1",
        mode: "BIOMETRIC_PAD",
        svgStrokes: [],
        otpCode: "",
      }).valid,
      false
    );
    assert.equal(
      validateInformedConsentSubmission({
        consentId: "c-1",
        mode: "OTP_CODE",
        svgStrokes: [],
        otpCode: "987654",
      }).valid,
      true
    );

    const planVm = buildTreatmentPlanViewModel(
      {
        id: "plan-1",
        patientId: "pat-01",
        title: "Plan Integral",
        phases: [
          {
            id: "ph-1",
            name: "Fase Higiénica",
            items: [
              {
                id: "i-1",
                procedureCode: "PER-01",
                description: "Profilaxis",
                priceCents: 4000_00,
                status: "REALIZADO",
              },
            ],
          },
          {
            id: "ph-2",
            name: "Fase Rehabilitadora",
            items: [
              {
                id: "i-2",
                procedureCode: "RES-16",
                description: "Resina 16",
                priceCents: 8000_00,
                status: "PENDIENTE",
              },
              {
                id: "i-3",
                procedureCode: "END-14",
                description: "Endodoncia 14",
                priceCents: 15000_00,
                status: "PENDIENTE",
              },
            ],
          },
        ],
      },
      ["i-2"]
    );
    assert.equal(planVm.phases[0].isCollapsedByDefault, true);
    assert.equal(planVm.approvalMode, "PARTIAL");
    assert.equal(planVm.selectedSubtotalCents, 8000_00);
  });

  it("wi-12 S1, S2, S3 & S4 / RF-12, RN-02, RF-13 & RF-14: enforces exact mixed-payment POS sum, Blind Cash Close hiding theoretical balance, dentist settlements, and inventory alerts", () => {
    const posEval = evaluatePosCheckoutBalance(
      [{ procedureCode: "RES-16", description: "Resina", quantity: 1, unitPriceCents: 10000_00 }],
      [
        { method: "CASH", amountCents: 4000_00 },
        { method: "CREDIT_CARD", amountCents: 6000_00 },
      ]
    );
    assert.equal(posEval.canIssueFiscalReceipt, true);

    const declaredCents = calculatePhysicalDenominationTotalCents({ "10000": 8 });
    assert.equal(declaredCents, 800_00);
    assert.equal(
      validateBlindCashCloseSubmission({
        registerStatus: "OPEN",
        declaredCashCents: declaredCents,
        confirmedByCashier: true,
      }).valid,
      true
    );

    const receptionistBlindHtml = renderBlindCashCloseViewHtml({
      cashRegister: {
        id: "reg-1",
        openedBy: "Carla",
        openedAt: "2026-10-30T08:00:00Z",
        status: "OPEN",
        theoreticalCashCents: 777777_00,
      },
      userRole: "receptionist",
      denominationsCount: { "10000": 8 },
      confirmedByCashier: true,
    });
    assert.doesNotMatch(receptionistBlindHtml, /7,777\.77/);
    assert.doesNotMatch(receptionistBlindHtml, /Saldo Teórico del Sistema:/);

    const settlement = calculateDentistFeeSettlement({
      ruleType: "NET_PERCENTAGE_AFTER_COSTS",
      percentageBps: 5000,
      fixedAmountCents: 0,
      grossProductionCents: 50000_00,
      labCostCents: 8000_00,
      materialsCostCents: 2000_00,
    });
    assert.equal(settlement.netPayableCents, 20000_00);

    const supplies = [
      {
        id: "s-1",
        sku: "ART-4",
        name: "Articaína",
        consultorioId: "SILLON-1",
        unit: "carpules",
        currentStock: 4,
        minStockThreshold: 10,
        unitCostCents: 150_00,
      },
    ];
    assert.equal(evaluateInventoryStockAlerts(supplies)[0].lowStockAlert, true);
    const deduction = simulateProcedureRecipeDeduction(supplies, {
      procedureCode: "EXO-18",
      procedureName: "Exodoncia",
      lines: [{ supplyId: "s-1", quantityPerProcedure: 2 }],
    });
    assert.equal(deduction.totalRecipeMaterialCostCents, 300_00);
    assert.equal(deduction.remainingStockBySupplyId["s-1"], 2);
  });
});
