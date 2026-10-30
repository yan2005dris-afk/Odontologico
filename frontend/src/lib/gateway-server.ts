/**
 * Server-side API Gateway client for the SIGO-Dental portal (wi-13 S4/S5).
 *
 * Data flow (S4): the browser only ever talks to Next (same origin). Server
 * components, server actions and route handlers call the gateway over the
 * internal compose network through `GATEWAY_INTERNAL_URL`, so no CORS
 * configuration is needed and the gateway is never exposed to the browser.
 *
 * The wire format of the Go services is snake_case; the portal view layer
 * consumes the camelCase DTOs declared in `session-idle-guard.ts` and in the
 * component modules. The mapping functions below are pure and therefore
 * unit-testable without any network access.
 */

import type {
  AppointmentStatus,
  CriticalAlertDTO,
  PatientSummaryDTO,
} from "./session-idle-guard.ts";
import type { CalendarAppointmentDTO } from "../components/agenda/MultiChairCalendar.tsx";
import type {
  DentalSurface,
  OdontogramFindingCode,
  OdontogramFindingDTO,
} from "../components/clinical/OdontogramCanvas.tsx";
import type {
  PeriodontogramToothDTO,
  SixSiteTuple,
} from "../components/clinical/PeriodontogramGrid.tsx";
import type { SoepNoteDTO } from "../components/clinical/SoepEvolutionTimeline.tsx";

export const DEFAULT_GATEWAY_INTERNAL_URL = "http://api-gateway:8080";
export const DEFAULT_GATEWAY_TIMEOUT_MS = 5_000;

/** Resolves the internal gateway URL used by every server-side call. */
export function getGatewayInternalUrl(
  env: Record<string, string | undefined> = process.env
): string {
  const raw = (env.GATEWAY_INTERNAL_URL ?? "").trim();
  const base = raw === "" ? DEFAULT_GATEWAY_INTERNAL_URL : raw;
  return base.replace(/\/+$/, "");
}

/** Error raised by the gateway client; `code` mirrors the upstream error code. */
export class GatewayError extends Error {
  public readonly code: string;
  public readonly status: number;

  constructor(code: string, message: string, status: number) {
    super(message);
    this.name = "GatewayError";
    this.code = code;
    this.status = status;
  }

  /** True when the gateway rejected the session (idle timeout or invalid token). */
  get isUnauthorized(): boolean {
    return this.status === 401;
  }
}

export interface GatewayServerOptions {
  /** Overrides `GATEWAY_INTERNAL_URL`; defaults to the compose-internal gateway. */
  baseUrl?: string;
  /** Bearer token of the current portal session; absent for public auth calls. */
  accessToken?: string | null;
  /** Injectable fetch, used by the unit tests. */
  fetchImpl?: typeof fetch;
  /** Upstream timeout in milliseconds (plan §6). */
  timeoutMs?: number;
  env?: Record<string, string | undefined>;
}

// ---------------------------------------------------------------------------
// Wire shapes (snake_case, exactly as the Go services encode them).
// ---------------------------------------------------------------------------

export interface WireCriticalAlert {
  code?: string;
  label?: string;
  severity?: string;
  category?: string;
}

export interface WireInsurancePlan {
  provider_name?: string;
  plan_name?: string;
  member_number?: string;
}

export interface WirePatient {
  id?: string;
  hc_number?: string;
  first_name?: string;
  last_name?: string;
  full_name?: string;
  age?: number;
  is_minor?: boolean;
  insurance_plan?: WireInsurancePlan | null;
  critical_alerts?: WireCriticalAlert[] | null;
}

export interface WireOdontogramFinding {
  id?: string;
  tooth_fdi?: string;
  surface?: string;
  finding_type?: string;
  status?: string;
}

export interface WirePeriodontogramTooth {
  tooth_fdi?: string;
  probing_depth_mm?: number[];
  gingival_margin_mm?: number[];
  bleeding_on_probing?: boolean[];
  furcation_grade?: number;
  mobility_grade?: number;
}

export interface WireSoepAddendum {
  id?: string;
  author_dentist_id?: string;
  correction_text?: string;
  created_at?: string;
}

export interface WireSoepNote {
  id?: string;
  patient_id?: string;
  author_dentist_id?: string;
  subjective?: string;
  objective?: string;
  assessment?: string;
  plan?: string;
  created_at?: string;
  is_locked?: boolean;
  addenda?: WireSoepAddendum[] | null;
}

export interface WireAppointment {
  id?: string;
  patient_id?: string;
  dentist_id?: string;
  chair_id?: string;
  clinic_id?: string;
  start_at?: string;
  end_at?: string;
  status?: string;
  reason?: string;
  notes?: string;
}

export interface LoginWireResponse {
  mfa_required?: boolean;
  challenge_token?: string;
  expires_in_seconds?: number;
}

export interface VerifyMfaWireResponse {
  access_token?: string;
  session_id?: string;
  user?: {
    id?: string;
    email?: string;
    full_name?: string;
    role?: string;
    mfa_verified?: boolean;
  };
  idle_timeout_seconds?: number;
}

// ---------------------------------------------------------------------------
// Pure mapping helpers (wire -> portal DTOs).
// ---------------------------------------------------------------------------

const APPOINTMENT_STATUSES: AppointmentStatus[] = [
  "PROGRAMADO",
  "CONFIRMADO",
  "EN_SALA_DE_ESPERA",
  "EN_ATENCION",
  "ATENDIDO",
  "CANCELADO",
  "NO_SHOW",
];

export function toAppointmentStatus(value: unknown): AppointmentStatus {
  return APPOINTMENT_STATUSES.includes(value as AppointmentStatus)
    ? (value as AppointmentStatus)
    : "PROGRAMADO";
}

function toCriticalAlerts(wire: WireCriticalAlert[] | null | undefined): CriticalAlertDTO[] {
  if (!Array.isArray(wire)) {
    return [];
  }
  return wire.map((alert) => ({
    code: alert?.code ?? "",
    label: alert?.label ?? "",
    severity: alert?.severity === "CRITICAL" ? "CRITICAL" : "HIGH",
  }));
}

function joinNames(wire: WirePatient): string {
  if (typeof wire.full_name === "string" && wire.full_name.trim() !== "") {
    return wire.full_name.trim();
  }
  const first = (wire.first_name ?? "").trim();
  const last = (wire.last_name ?? "").trim();
  return [first, last].filter((part) => part !== "").join(" ");
}

/** Maps a patient record (search result or clinical header) to the portal DTO. */
export function mapPatientSummary(wire: WirePatient | null | undefined): PatientSummaryDTO {
  const source = wire ?? {};
  const insurance = source.insurance_plan;

  return {
    id: source.id ?? "",
    hcNumber: source.hc_number ?? "",
    fullName: joinNames(source),
    age: typeof source.age === "number" ? source.age : 0,
    isMinor: source.is_minor === true,
    insurancePlan: insurance
      ? {
          providerName: insurance.provider_name ?? "",
          planName: insurance.plan_name ?? "",
        }
      : undefined,
    criticalAlerts: toCriticalAlerts(source.critical_alerts),
  };
}

const DENTAL_SURFACES: DentalSurface[] = [
  "MESIAL",
  "DISTAL",
  "OCLUSAL_INCISAL",
  "VESTIBULAR",
  "LINGUAL_PALATINO",
];

function toDentalSurface(value: unknown): DentalSurface | "WHOLE_TOOTH" {
  if (value === "WHOLE_TOOTH") {
    return "WHOLE_TOOTH";
  }
  return DENTAL_SURFACES.includes(value as DentalSurface)
    ? (value as DentalSurface)
    : "WHOLE_TOOTH";
}

/**
 * Maps the clinical finding catalog of the odontogram domain onto the portal
 * catalog. Returns null for values outside both catalogs so the caller can
 * report them instead of silently rendering a wrong clinical color.
 */
export function mapFindingCode(value: unknown): OdontogramFindingCode | null {
  switch (value) {
    case "CARIES":
      return "CARIES";
    case "EXISTING_RESTORATION":
      return "EXISTING_RESTORATION";
    case "ENDODONTICS_DONE":
    case "ENDODONTICS_REQUIRED":
      return "ENDODONTICS";
    case "IMPLANT":
      return "IMPLANT";
    case "FIXED_PROSTHESIS":
    case "REMOVABLE_PROSTHESIS":
      return "PROSTHESIS_FIXED";
    case "EXTRACTION_DONE":
    case "EXTRACTION_INDICATED":
      return "EXTRACTION";
    case "SEALANT":
      return "SEALANT";
    default:
      return null;
  }
}

export interface MappedOdontogram {
  findings: OdontogramFindingDTO[];
  /** Findings whose clinical catalog could not be mapped to the portal catalog. */
  unmappedCount: number;
}

/** Maps the odontogram snapshot onto the portal findings. */
export function mapOdontogramFindings(
  wire: WireOdontogramFinding[] | null | undefined
): MappedOdontogram {
  if (!Array.isArray(wire)) {
    return { findings: [], unmappedCount: 0 };
  }

  const findings: OdontogramFindingDTO[] = [];
  let unmappedCount = 0;

  for (const raw of wire) {
    const code = mapFindingCode(raw?.finding_type);
    if (!code) {
      unmappedCount += 1;
      continue;
    }
    findings.push({
      id: raw?.id ?? "",
      fdiTooth: raw?.tooth_fdi ?? "",
      surface: toDentalSurface(raw?.surface),
      findingCode: code,
      status: raw?.status === "REALIZADO" ? "REALIZADO" : "PLANIFICADO",
    });
  }

  return { findings, unmappedCount };
}

function toSixSiteTuple<T>(values: T[] | undefined, fallback: T): SixSiteTuple<T> {
  const source = Array.isArray(values) ? values : [];
  return [
    (source[0] ?? fallback) as T,
    (source[1] ?? fallback) as T,
    (source[2] ?? fallback) as T,
    (source[3] ?? fallback) as T,
    (source[4] ?? fallback) as T,
    (source[5] ?? fallback) as T,
  ];
}

function toGrade(value: unknown): 0 | 1 | 2 | 3 {
  const grade = typeof value === "number" ? Math.trunc(value) : 0;
  if (grade < 0) return 0;
  if (grade > 3) return 3;
  return grade as 0 | 1 | 2 | 3;
}

/** Maps the periodontogram chart teeth onto the portal grid DTO. */
export function mapPeriodontogramTeeth(
  wire: WirePeriodontogramTooth[] | null | undefined
): PeriodontogramToothDTO[] {
  if (!Array.isArray(wire)) {
    return [];
  }

  return wire.map((tooth) => ({
    fdiTooth: tooth?.tooth_fdi ?? "",
    probingDepthMm: toSixSiteTuple(tooth?.probing_depth_mm, 0),
    gingivalMarginMm: toSixSiteTuple(tooth?.gingival_margin_mm, 0),
    bleedingOnProbing: toSixSiteTuple(tooth?.bleeding_on_probing, false),
    furcationGrade: toGrade(tooth?.furcation_grade),
    mobilityGrade: toGrade(tooth?.mobility_grade),
  }));
}

/** Maps the SOEP evolution notes onto the portal timeline DTO. */
export function mapSoepNotes(
  wire: WireSoepNote[] | null | undefined
): SoepNoteDTO[] {
  if (!Array.isArray(wire)) {
    return [];
  }

  return wire.map((note) => ({
    id: note?.id ?? "",
    patientId: note?.patient_id ?? "",
    authorName: note?.author_dentist_id ?? "",
    createdAtIso: note?.created_at ?? "",
    isLocked: note?.is_locked === true,
    subjective: note?.subjective ?? "",
    objective: note?.objective ?? "",
    assessment: note?.assessment ?? "",
    plan: note?.plan ?? "",
    addenda: (note?.addenda ?? []).map((addendum) => ({
      id: addendum?.id ?? "",
      authorName: addendum?.author_dentist_id ?? "",
      signedAtIso: addendum?.created_at ?? "",
      content: addendum?.correction_text ?? "",
    })),
  }));
}

/** Extracts the calendar date (YYYY-MM-DD) of an RFC3339 timestamp. */
export function toCalendarDate(timestamp: string | undefined): string {
  if (typeof timestamp !== "string" || timestamp.length < 10) {
    return "";
  }
  return timestamp.slice(0, 10);
}

/** Extracts the wall time (HH:mm) of an RFC3339 timestamp. */
export function toCalendarTime(timestamp: string | undefined): string {
  if (typeof timestamp !== "string" || timestamp.length < 16) {
    return "";
  }
  return timestamp.slice(11, 16);
}

/** Maps a scheduling appointment onto the portal calendar DTO. */
export function mapAppointment(
  wire: WireAppointment | null | undefined,
  patientNames: Record<string, string> = {}
): CalendarAppointmentDTO {
  const source = wire ?? {};
  const patientId = source.patient_id ?? "";
  const chairId = source.chair_id ?? "";
  const dentistId = source.dentist_id ?? "";

  return {
    id: source.id ?? "",
    patientId,
    patientName: patientNames[patientId] ?? `Paciente ${patientId}`,
    chairId,
    chairLabel: chairId === "" ? "Sillón sin asignar" : `Sillón ${chairId}`,
    dentistId,
    dentistName: dentistId === "" ? "Odontólogo sin asignar" : `Odontólogo ${dentistId}`,
    date: toCalendarDate(source.start_at),
    startTime: toCalendarTime(source.start_at),
    endTime: toCalendarTime(source.end_at),
    status: toAppointmentStatus(source.status),
    procedureSummary: source.reason ?? "",
  };
}

// ---------------------------------------------------------------------------
// Gateway client.
// ---------------------------------------------------------------------------

export interface PatientListResult {
  items: PatientSummaryDTO[];
  total: number;
}

export interface AppointmentListResult {
  items: CalendarAppointmentDTO[];
}

/**
 * Creates the server-side gateway client. Every method maps the snake_case
 * wire payload onto the camelCase DTOs consumed by the portal view layer.
 */
export function createGatewayServerClient(options: GatewayServerOptions = {}) {
  const baseUrl = (
    options.baseUrl ?? getGatewayInternalUrl(options.env)
  ).replace(/\/+$/, "");
  const accessToken = options.accessToken ?? null;
  const fetchImpl = options.fetchImpl ?? globalThis.fetch;
  const timeoutMs = options.timeoutMs ?? DEFAULT_GATEWAY_TIMEOUT_MS;

  async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(init.headers as Record<string, string> | undefined),
    };
    if (accessToken) {
      headers.Authorization = `Bearer ${accessToken}`;
    }

    let response: Response;
    try {
      response = await fetchImpl(`${baseUrl}${path}`, {
        ...init,
        headers,
        cache: "no-store",
        signal: init.signal ?? AbortSignal.timeout(timeoutMs),
      });
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      throw new GatewayError(
        "GATEWAY_UNREACHABLE",
        `No se pudo contactar al API Gateway (${detail}).`,
        0
      );
    }

    if (!response.ok) {
      let code = `HTTP_${response.status}`;
      let message = response.statusText || "Error inesperado del API Gateway";
      try {
        const body = (await response.json()) as { code?: string; message?: string };
        if (body?.code) code = body.code;
        if (body?.message) message = body.message;
      } catch {
        // Non-JSON error bodies keep the generic message.
      }
      throw new GatewayError(code, message, response.status);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return (await response.json()) as T;
  }

  return {
    baseUrl,

    async login(email: string, password: string): Promise<LoginWireResponse> {
      return request<LoginWireResponse>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      });
    },

    async verifyMfa(challengeToken: string, totpCode: string): Promise<VerifyMfaWireResponse> {
      return request<VerifyMfaWireResponse>("/api/v1/auth/mfa/verify", {
        method: "POST",
        body: JSON.stringify({ challenge_token: challengeToken, totp_code: totpCode }),
      });
    },

    async listPatients(): Promise<PatientListResult> {
      const body = await request<{ items?: WirePatient[]; total?: number }>("/api/v1/patients");
      const items = Array.isArray(body?.items) ? body.items : [];
      return {
        items: items.map((item) => mapPatientSummary(item)),
        total: typeof body?.total === "number" ? body.total : items.length,
      };
    },

    async getPatientHeader(patientId: string): Promise<PatientSummaryDTO> {
      const body = await request<WirePatient>(
        `/api/v1/patients/${encodeURIComponent(patientId)}/header`
      );
      return mapPatientSummary(body);
    },

    async getOdontogram(patientId: string): Promise<MappedOdontogram> {
      const body = await request<{ findings?: WireOdontogramFinding[] }>(
        `/api/v1/patients/${encodeURIComponent(patientId)}/odontogram`
      );
      return mapOdontogramFindings(body?.findings);
    },

    async getPeriodontogram(patientId: string): Promise<PeriodontogramToothDTO[]> {
      const body = await request<{ items?: { teeth?: WirePeriodontogramTooth[] }[] }>(
        `/api/v1/patients/${encodeURIComponent(patientId)}/periodontograms`
      );
      const charts = Array.isArray(body?.items) ? body.items : [];
      const latest = charts.length > 0 ? charts[charts.length - 1] : undefined;
      return mapPeriodontogramTeeth(latest?.teeth);
    },

    async getSoepNotes(patientId: string): Promise<SoepNoteDTO[]> {
      const body = await request<{ items?: WireSoepNote[] }>(
        `/api/v1/patients/${encodeURIComponent(patientId)}/soep-notes`
      );
      return mapSoepNotes(body?.items);
    },

    async listAppointments(
      patientNames: Record<string, string> = {}
    ): Promise<AppointmentListResult> {
      const body = await request<{ items?: WireAppointment[] }>(
        "/api/v1/scheduling/appointments"
      );
      const items = Array.isArray(body?.items) ? body.items : [];
      return { items: items.map((item) => mapAppointment(item, patientNames)) };
    },

    async transitionAppointment(
      appointmentId: string,
      targetStatus: AppointmentStatus
    ): Promise<{ id: string; status: AppointmentStatus }> {
      const body = await request<{ id?: string; status?: string }>(
        `/api/v1/scheduling/appointments/${encodeURIComponent(appointmentId)}/transition`,
        {
          method: "POST",
          body: JSON.stringify({ target_status: targetStatus }),
        }
      );
      return { id: body?.id ?? appointmentId, status: toAppointmentStatus(body?.status) };
    },

    async createPatient(payload: {
      documentType: string;
      documentNumber: string;
      firstName: string;
      lastName: string;
      birthDate: string;
      sex: string;
      phone: string;
      email?: string;
      emergencyContact: { fullName: string; phone: string; relationship: string };
      legalGuardian?: {
        fullName: string;
        documentNumber: string;
        phone: string;
        relationship: string;
      };
    }): Promise<PatientSummaryDTO> {
      const body = await request<WirePatient>("/api/v1/patients", {
        method: "POST",
        body: JSON.stringify({
          document_type: payload.documentType,
          document_number: payload.documentNumber,
          first_name: payload.firstName,
          last_name: payload.lastName,
          birth_date: payload.birthDate,
          sex: payload.sex,
          phone: payload.phone,
          email: payload.email ?? "",
          emergency_contact: {
            full_name: payload.emergencyContact.fullName,
            phone: payload.emergencyContact.phone,
            relationship: payload.emergencyContact.relationship,
          },
          legal_guardian: payload.legalGuardian
            ? {
                full_name: payload.legalGuardian.fullName,
                document_number: payload.legalGuardian.documentNumber,
                phone: payload.legalGuardian.phone,
                relationship: payload.legalGuardian.relationship,
              }
            : null,
        }),
      });
      return mapPatientSummary(body);
    },

    request,
  };
}
