/**
 * View-model contract of the portal data pages (wi-13 S5-S9).
 *
 * The page components only translate these pure models into JSX, so the
 * ready / empty / error states of every module are asserted here without a
 * browser, a running gateway or a Next.js runtime.
 */

import { describe, it } from "vitest";
import assert from "node:assert/strict";
import {
  buildAgendaModel,
  buildMissingEndpointEmptyState,
  buildPatientDetailModel,
  buildPatientsListModel,
  buildPortalErrorState,
} from "./portal-views.ts";
import type { AgendaFilterState, PatientSummaryDTO } from "./session-idle-guard.ts";
import type { CalendarAppointmentDTO } from "../components/agenda/MultiChairCalendar.tsx";
import type { OdontogramFindingDTO } from "../components/clinical/OdontogramCanvas.tsx";
import type { PeriodontogramToothDTO } from "../components/clinical/PeriodontogramGrid.tsx";
import type { SoepNoteDTO } from "../components/clinical/SoepEvolutionTimeline.tsx";

const patient: PatientSummaryDTO = {
  id: "pat-001",
  hcNumber: "HC-2026-000001",
  fullName: "María Fernanda Gómez",
  age: 64,
  isMinor: false,
  criticalAlerts: [],
};

const minorPatient: PatientSummaryDTO = {
  id: "pat-002",
  hcNumber: "HC-2026-000002",
  fullName: "Lucas Mateo Díaz",
  age: 12,
  isMinor: true,
  criticalAlerts: [
    { code: "PENICILLIN_ALLERGY", label: "Alergia Severa a Penicilina", severity: "CRITICAL" },
  ],
};

const finding: OdontogramFindingDTO = {
  id: "f-1",
  fdiTooth: "16",
  surface: "OCLUSAL_INCISAL",
  findingCode: "CARIES",
  status: "PLANIFICADO",
};

const tooth: PeriodontogramToothDTO = {
  fdiTooth: "16",
  probingDepthMm: [3, 4, 3, 2, 3, 2],
  gingivalMarginMm: [0, -1, 0, 0, 0, 0],
  bleedingOnProbing: [true, false, false, false, false, true],
  furcationGrade: 0,
  mobilityGrade: 0,
};

const note: SoepNoteDTO = {
  id: "n-1",
  patientId: "pat-001",
  authorName: "Dr. Alejandro Vega",
  createdAtIso: "2026-10-30T10:00:00Z",
  isLocked: false,
  subjective: "S",
  objective: "O",
  assessment: "A",
  plan: "P",
  addenda: [],
};

const appointment: CalendarAppointmentDTO = {
  id: "apt-101",
  patientId: "pat-001",
  patientName: "María Fernanda Gómez",
  chairId: "chair-1",
  chairLabel: "Sillón 1 - Cirugía",
  dentistId: "dent-1",
  dentistName: "Dr. Alejandro Vega",
  date: "2026-10-30",
  startTime: "09:00",
  endTime: "10:00",
  status: "CONFIRMADO",
  procedureSummary: "Implante Pieza 16",
};

const dayFilter: AgendaFilterState = { view: "day", date: "2026-10-30" };

/** Asserts that an empty state never leaks a monetary figure (RN-02). */
function assertNoCurrencyFigures(text: string): void {
  assert.equal(text.includes("$"), false, `unexpected currency symbol in: ${text}`);
  assert.equal(text.includes("USD"), false, `unexpected currency code in: ${text}`);
  assert.equal(text.includes("ARS"), false, `unexpected currency code in: ${text}`);
  assert.equal(/\d{1,3}(?:\.\d{3})+,\d{2}/.test(text), false, `unexpected amount in: ${text}`);
  assert.equal(/\b\d+[.,]\d{2}\b/.test(text), false, `unexpected amount in: ${text}`);
}

describe("wi-13 S6 — /patients list view model", () => {
  it("reports the ready state with the patients and the server total", () => {
    const model = buildPatientsListModel({ patients: [patient, minorPatient], total: 2 });

    assert.equal(model.status, "ready");
    assert.equal(model.patients.length, 2);
    assert.equal(model.total, 2);
    assert.equal(model.patients[1].isMinor, true);
    assert.equal(model.error, undefined);
  });

  it("reports the empty state when the clinic has no patients yet", () => {
    const model = buildPatientsListModel({ patients: [], total: 0 });

    assert.equal(model.status, "empty");
    assert.deepEqual(model.patients, []);
    assert.equal(model.total, 0);
  });

  it("reports the error state with a recoverable code when the gateway fails", () => {
    const model = buildPatientsListModel({
      error: { code: "GATEWAY_UNREACHABLE", status: 0, message: "boom" },
    });

    assert.equal(model.status, "error");
    assert.equal(model.error?.code, "GATEWAY_UNREACHABLE");
    assert.equal(model.error?.retryable, true);
    assert.deepEqual(model.patients, []);
  });

  it("maps an expired session to a non retryable SESSION_IDLE_TIMEOUT", () => {
    const model = buildPatientsListModel({
      error: { code: "SESSION_IDLE_TIMEOUT", status: 401 },
    });

    assert.equal(model.status, "error");
    assert.equal(model.error?.code, "SESSION_IDLE_TIMEOUT");
    assert.equal(model.error?.retryable, false);
  });

  it("keeps the RBAC denial non retryable", () => {
    const state = buildPortalErrorState({ code: "FORBIDDEN_ROLE", status: 403 });

    assert.equal(state.code, "FORBIDDEN_ROLE");
    assert.equal(state.retryable, false);
  });
});

describe("wi-13 S7 — /patients/[id] clinical record view model", () => {
  it("reports the ready state with every clinical section populated", () => {
    const model = buildPatientDetailModel({
      patient,
      odontogram: { findings: [finding], unmappedCount: 0 },
      periodontogram: { teeth: [tooth] },
      soep: { notes: [note] },
    });

    assert.equal(model.status, "ready");
    assert.equal(model.patient?.id, "pat-001");

    const odontogram = model.sections.find((section) => section.id === "odontogram");
    const periodontogram = model.sections.find((section) => section.id === "periodontogram");
    const soep = model.sections.find((section) => section.id === "soep");

    assert.equal(odontogram?.status, "ready");
    assert.equal(periodontogram?.status, "ready");
    assert.equal(soep?.status, "ready");
    assert.equal(model.odontogram.findings.length, 1);
    assert.equal(model.periodontogram.teeth.length, 1);
    assert.equal(model.soep.notes.length, 1);
  });

  it("reports a per-section empty state instead of an empty canvas", () => {
    const model = buildPatientDetailModel({
      patient,
      odontogram: { findings: [], unmappedCount: 0 },
      periodontogram: { teeth: [] },
      soep: { notes: [] },
    });

    const odontogram = model.sections.find((section) => section.id === "odontogram");
    const periodontogram = model.sections.find((section) => section.id === "periodontogram");
    const soep = model.sections.find((section) => section.id === "soep");

    assert.equal(odontogram?.status, "empty");
    assert.match(odontogram?.note ?? "", /hallazgos odontológicos/i);
    assert.equal(periodontogram?.status, "empty");
    assert.match(periodontogram?.note ?? "", /periodontalgramas/i);
    assert.equal(soep?.status, "empty");
    assert.match(soep?.note ?? "", /notas de evolución/i);
  });

  it("isolates a failing clinical section without breaking the whole record", () => {
    const model = buildPatientDetailModel({
      patient,
      odontogram: { findings: [], unmappedCount: 0 },
      periodontogram: { teeth: [tooth] },
      soep: { notes: [note] },
      errors: { odontogram: { code: "HTTP_500", status: 500 } },
    });

    assert.equal(model.status, "ready");
    const odontogram = model.sections.find((section) => section.id === "odontogram");
    assert.equal(odontogram?.status, "error");
    assert.match(odontogram?.note ?? "", /no está disponible/i);
    assert.equal(model.sections.find((section) => section.id === "soep")?.status, "ready");
  });

  it("declares the missing read endpoint of PRESUPUESTOS and CONSENTIMIENTOS", () => {
    const model = buildPatientDetailModel({ patient });

    const budgets = model.sections.find((section) => section.id === "budgets");
    const consents = model.sections.find((section) => section.id === "consents");

    assert.equal(budgets?.status, "unavailable");
    assert.match(budgets?.note ?? "", /treatment-plans/);
    assert.equal(consents?.status, "unavailable");
    assert.match(consents?.note ?? "", /consents/);
  });

  it("reports the not-found state instead of crashing on an unknown patient", () => {
    const model = buildPatientDetailModel({ patient: null });

    assert.equal(model.status, "not-found");
    assert.equal(model.error?.code, "PATIENT_NOT_FOUND");
    assert.deepEqual(model.sections, []);
  });

  it("reports the error state when the clinical header cannot be loaded", () => {
    const model = buildPatientDetailModel({
      patient: null,
      errors: { patient: { code: "GATEWAY_UNREACHABLE", status: 0 } },
    });

    assert.equal(model.status, "error");
    assert.equal(model.error?.code, "GATEWAY_UNREACHABLE");
    assert.equal(model.error?.retryable, true);
  });

  it("reports the findings whose clinical catalog is unknown", () => {
    const model = buildPatientDetailModel({
      patient,
      odontogram: { findings: [finding], unmappedCount: 3 },
    });

    assert.equal(model.odontogram.unmappedCount, 3);
    assert.equal(model.odontogram.findings.length, 1);
  });
});

describe("wi-13 S8 — /agenda view model", () => {
  it("reports the ready state with the appointments of the active date", () => {
    const model = buildAgendaModel({ appointments: [appointment], filter: dayFilter });

    assert.equal(model.status, "ready");
    assert.equal(model.appointments.length, 1);
    assert.equal(model.filter.date, "2026-10-30");
    assert.equal(model.conflict, undefined);
  });

  it("reports the empty state when the agenda has no appointments", () => {
    const model = buildAgendaModel({ appointments: [], filter: dayFilter });

    assert.equal(model.status, "empty");
    assert.deepEqual(model.appointments, []);
  });

  it("reports the error state without losing the requested filter", () => {
    const model = buildAgendaModel({
      appointments: [],
      filter: dayFilter,
      error: { code: "GATEWAY_UNREACHABLE", status: 0 },
    });

    assert.equal(model.status, "error");
    assert.equal(model.error?.retryable, true);
    assert.equal(model.filter.date, "2026-10-30");
  });

  it("preserves the chair overlap conflict so the transition form is not lost (RN-03)", () => {
    const conflict = {
      code: "CHAIR_OVERLAP_CONFLICT",
      message: "El sillón físico ya está reservado.",
      appointmentId: "apt-101",
      targetStatus: "EN_SALA_DE_ESPERA",
    };

    const model = buildAgendaModel({
      appointments: [appointment],
      filter: dayFilter,
      conflict,
    });

    assert.equal(model.status, "ready");
    assert.equal(model.conflict?.code, "CHAIR_OVERLAP_CONFLICT");
    assert.equal(model.conflict?.appointmentId, "apt-101");
    assert.equal(model.conflict?.targetStatus, "EN_SALA_DE_ESPERA");
  });

  it("keeps the conflict visible even when the agenda failed to load", () => {
    const model = buildAgendaModel({
      appointments: [],
      filter: dayFilter,
      error: { code: "HTTP_503", status: 503 },
      conflict: { code: "CHAIR_OVERLAP_CONFLICT", message: "Sillón ocupado." },
    });

    assert.equal(model.status, "error");
    assert.equal(model.conflict?.code, "CHAIR_OVERLAP_CONFLICT");
  });
});

describe("wi-13 S9 — empty states of the modules without a read endpoint", () => {
  it("names the missing read endpoint and the available write operations of billing", () => {
    const state = buildMissingEndpointEmptyState("billing");

    assert.equal(state.title, "Caja y facturación");
    assert.equal(state.missingEndpoint, "GET /api/v1/billing/invoices");
    assert.deepEqual(state.availableOperations, ["POST /api/v1/billing/invoices"]);
    assert.match(state.description, /RN-02/);
    assertNoCurrencyFigures(
      `${state.title} ${state.description} ${state.missingEndpoint} ${state.availableOperations.join(" ")}`
    );
  });

  it("names the missing read endpoint and the available write operations of settlements", () => {
    const state = buildMissingEndpointEmptyState("settlements");

    assert.equal(state.title, "Liquidaciones por profesional");
    assert.equal(state.missingEndpoint, "GET /api/v1/settlements/commissions");
    assert.deepEqual(state.availableOperations, ["POST /api/v1/settlements/commissions"]);
    assertNoCurrencyFigures(
      `${state.title} ${state.description} ${state.missingEndpoint} ${state.availableOperations.join(" ")}`
    );
  });

  it("names the missing read endpoint and the available write operations of inventory", () => {
    const state = buildMissingEndpointEmptyState("inventory");

    assert.equal(state.title, "Inventario de insumos");
    assert.equal(state.missingEndpoint, "GET /api/v1/inventory/supplies");
    assert.deepEqual(state.availableOperations, ["POST /api/v1/inventory/supplies"]);
    assertNoCurrencyFigures(
      `${state.title} ${state.description} ${state.missingEndpoint} ${state.availableOperations.join(" ")}`
    );
  });

  it("never exposes a theoretical cash total before the blind close (RN-02)", () => {
    const billing = buildMissingEndpointEmptyState("billing");

    assertNoCurrencyFigures(billing.description);
    assert.match(billing.description, /nunca se expone antes del cierre ciego/i);
  });
});
