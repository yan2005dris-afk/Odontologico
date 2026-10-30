/**
 * Pure view models for the portal pages (wi-13 S5-S9).
 *
 * Every page fetches through the gateway client and then delegates the data
 * shaping to these pure functions, so the loading / empty / error states are
 * unit-testable without a browser or a running gateway.
 */

import type {
  AgendaFilterState,
  PatientSummaryDTO,
} from "./session-idle-guard.ts";
import type { CalendarAppointmentDTO } from "../components/agenda/MultiChairCalendar.tsx";
import type { OdontogramFindingDTO } from "../components/clinical/OdontogramCanvas.tsx";
import type { PeriodontogramToothDTO } from "../components/clinical/PeriodontogramGrid.tsx";
import type { SoepNoteDTO } from "../components/clinical/SoepEvolutionTimeline.tsx";

export interface PortalErrorState {
  code: string;
  message: string;
  /** True when the operator can retry the same operation. */
  retryable: boolean;
}

export type PortalStatus = "ready" | "empty" | "error";

/** Builds the error state shown when the gateway cannot answer a request. */
export function buildPortalErrorState(error: unknown): PortalErrorState {
  const candidate = (error ?? {}) as { code?: string; status?: number; message?: string };
  const status = candidate.status ?? 0;
  const code = candidate.code ?? "UNKNOWN_ERROR";

  if (code === "SESSION_IDLE_TIMEOUT" || status === 401) {
    return {
      code: "SESSION_IDLE_TIMEOUT",
      message: "La sesión de la estación expiró por inactividad. Vuelva a iniciar sesión.",
      retryable: false,
    };
  }

  if (code === "FORBIDDEN_ROLE" || status === 403) {
    return {
      code,
      message: candidate.message ?? "Su rol no tiene permiso para consultar este recurso.",
      retryable: false,
    };
  }

  if (code === "GATEWAY_UNREACHABLE" || status === 0) {
    return {
      code: "GATEWAY_UNREACHABLE",
      message: "El API Gateway no respondió. Reintente en unos segundos.",
      retryable: true,
    };
  }

  if (status >= 500) {
    return {
      code,
      message: candidate.message ?? "El servicio-clinical no está disponible. Reintente en unos segundos.",
      retryable: true,
    };
  }

  return {
    code,
    message: candidate.message ?? "No se pudo obtener la información solicitada.",
    retryable: false,
  };
}

/**
 * True when the gateway rejected the session: the workstation session expired
 * after 15 minutes of inactivity (RNF-05) or the token is no longer valid.
 *
 * The portal pages redirect to `/login?reason=idle_timeout` instead of showing
 * an error state, because a dead session is not a recoverable page state.
 */
export function isSessionExpiredError(error: unknown): boolean {
  const candidate = (error ?? {}) as { code?: string; status?: number };
  if (!candidate) {
    return false;
  }
  return candidate.code === "SESSION_IDLE_TIMEOUT" || candidate.status === 401;
}

// ---------------------------------------------------------------------------
// /patients
// ---------------------------------------------------------------------------

export interface PatientsListModel {
  status: PortalStatus;
  patients: PatientSummaryDTO[];
  total: number;
  error?: PortalErrorState;
}

export function buildPatientsListModel(input: {
  patients?: PatientSummaryDTO[];
  total?: number;
  error?: unknown;
}): PatientsListModel {
  if (input.error) {
    return { status: "error", patients: [], total: 0, error: buildPortalErrorState(input.error) };
  }

  const patients = input.patients ?? [];
  return {
    status: patients.length === 0 ? "empty" : "ready",
    patients,
    total: typeof input.total === "number" ? input.total : patients.length,
  };
}

// ---------------------------------------------------------------------------
// /patients/[id]
// ---------------------------------------------------------------------------

export type PatientSectionStatus = "ready" | "empty" | "error" | "unavailable";

export interface PatientSection {
  id: "odontogram" | "periodontogram" | "soep" | "budgets" | "consents";
  label: string;
  status: PatientSectionStatus;
  /** Explains an empty, unavailable or failed section. */
  note?: string;
}

export interface PatientDetailModel {
  status: "ready" | "not-found" | "error";
  patient?: PatientSummaryDTO;
  error?: PortalErrorState;
  sections: PatientSection[];
  odontogram: {
    findings: OdontogramFindingDTO[];
    unmappedCount: number;
  };
  periodontogram: {
    teeth: PeriodontogramToothDTO[];
  };
  soep: {
    notes: SoepNoteDTO[];
  };
}

export function buildPatientDetailModel(input: {
  patient?: PatientSummaryDTO | null;
  odontogram?: { findings: OdontogramFindingDTO[]; unmappedCount?: number };
  periodontogram?: { teeth: PeriodontogramToothDTO[] };
  soep?: { notes: SoepNoteDTO[] };
  errors?: {
    patient?: unknown;
    odontogram?: unknown;
    periodontogram?: unknown;
    soep?: unknown;
  };
}): PatientDetailModel {
  const errors = input.errors ?? {};

  if (errors.patient) {
    return {
      status: "error",
      error: buildPortalErrorState(errors.patient),
      sections: [],
      odontogram: { findings: [], unmappedCount: 0 },
      periodontogram: { teeth: [] },
      soep: { notes: [] },
    };
  }

  if (!input.patient) {
    return {
      status: "not-found",
      error: {
        code: "PATIENT_NOT_FOUND",
        message: "No existe una ficha clínica con ese identificador.",
        retryable: false,
      },
      sections: [],
      odontogram: { findings: [], unmappedCount: 0 },
      periodontogram: { teeth: [] },
      soep: { notes: [] },
    };
  }

  const findings = input.odontogram?.findings ?? [];
  const teeth = input.periodontogram?.teeth ?? [];
  const notes = input.soep?.notes ?? [];

  const sections: PatientSection[] = [
    {
      id: "odontogram",
      label: "Odontograma 2D",
      status: errors.odontogram
        ? "error"
        : findings.length === 0
          ? "empty"
          : "ready",
      note: errors.odontogram
        ? buildPortalErrorState(errors.odontogram).message
        : findings.length === 0
          ? "Sin hallazgos odontológicos registrados para este paciente."
          : undefined,
    },
    {
      id: "periodontogram",
      label: "Periodontograma",
      status: errors.periodontogram
        ? "error"
        : teeth.length === 0
          ? "empty"
          : "ready",
      note: errors.periodontogram
        ? buildPortalErrorState(errors.periodontogram).message
        : teeth.length === 0
          ? "Sin periodontalgramas registrados para este paciente."
          : undefined,
    },
    {
      id: "soep",
      label: "Evolución SOEP",
      status: errors.soep ? "error" : notes.length === 0 ? "empty" : "ready",
      note: errors.soep
        ? buildPortalErrorState(errors.soep).message
        : notes.length === 0
          ? "Sin notas de evolución registradas para este paciente."
          : undefined,
    },
    {
      id: "budgets",
      label: "Presupuestos",
      status: "unavailable",
      note: "Sin endpoint de lectura de planes de tratamiento: solo existe POST /api/v1/patients/{id}/treatment-plans.",
    },
    {
      id: "consents",
      label: "Consentimientos",
      status: "unavailable",
      note: "Sin endpoint de lectura de consentimientos: solo existe POST /api/v1/patients/{id}/consents.",
    },
  ];

  return {
    status: "ready",
    patient: input.patient,
    sections,
    odontogram: {
      findings,
      unmappedCount: input.odontogram?.unmappedCount ?? 0,
    },
    periodontogram: { teeth },
    soep: { notes },
  };
}

// ---------------------------------------------------------------------------
// /agenda
// ---------------------------------------------------------------------------

export interface AgendaConflict {
  code: string;
  message: string;
  appointmentId?: string;
  targetStatus?: string;
}

export interface AgendaModel {
  status: PortalStatus;
  appointments: CalendarAppointmentDTO[];
  filter: AgendaFilterState;
  conflict?: AgendaConflict;
  error?: PortalErrorState;
}

export function buildAgendaModel(input: {
  appointments?: CalendarAppointmentDTO[];
  filter: AgendaFilterState;
  error?: unknown;
  conflict?: AgendaConflict;
}): AgendaModel {
  if (input.error) {
    return {
      status: "error",
      appointments: [],
      filter: input.filter,
      conflict: input.conflict,
      error: buildPortalErrorState(input.error),
    };
  }

  const appointments = input.appointments ?? [];
  return {
    status: appointments.length === 0 ? "empty" : "ready",
    appointments,
    filter: input.filter,
    conflict: input.conflict,
  };
}

// ---------------------------------------------------------------------------
// Empty states for the modules without a read endpoint (S9).
// ---------------------------------------------------------------------------

export interface MissingEndpointEmptyState {
  title: string;
  description: string;
  missingEndpoint: string;
  availableOperations: string[];
}

export function buildMissingEndpointEmptyState(
  module: "billing" | "settlements" | "inventory"
): MissingEndpointEmptyState {
  switch (module) {
    case "billing":
      return {
        title: "Caja y facturación",
        description:
          "El servicio billing-stock no expone un endpoint de listado de facturas ni de cierre de caja. No se muestran cifras hasta que exista ese endpoint: el total teórico de caja nunca se expone antes del cierre ciego (RN-02).",
        missingEndpoint: "GET /api/v1/billing/invoices",
        availableOperations: ["POST /api/v1/billing/invoices"],
      };
    case "settlements":
      return {
        title: "Liquidaciones por profesional",
        description:
          "El servicio billing-stock no expone un endpoint de lectura de liquidaciones. No se muestran comisiones ni totales calculados mientras ese endpoint no exista.",
        missingEndpoint: "GET /api/v1/settlements/commissions",
        availableOperations: ["POST /api/v1/settlements/commissions"],
      };
    case "inventory":
      return {
        title: "Inventario de insumos",
        description:
          "El servicio billing-stock no expone un endpoint de listado de insumos clínicos. El stock real se descuenta al completar un procedimiento en el servicio clínico.",
        missingEndpoint: "GET /api/v1/inventory/supplies",
        availableOperations: ["POST /api/v1/inventory/supplies"],
      };
    default:
      return {
        title: "Módulo sin datos",
        description: "Este módulo no expone datos de lectura en la API actual.",
        missingEndpoint: "",
        availableOperations: [],
      };
  }
}
