/**
 * Typed HTTP Client for SIGO-Dental API Gateway (wi-10, wi-11, wi-12).
 * Handles JWT/MFA session headers, 401 SESSION_IDLE_TIMEOUT interception (RNF-05),
 * and deterministic error codes (409 CHAIR_OVERLAP_CONFLICT, 422 LEGAL_GUARDIAN_REQUIRED_FOR_MINOR).
 */

import type {
  AppointmentStatus,
  CreatePatientInput,
  PatientSummaryDTO,
  UserRole,
} from "./session-idle-guard.ts";

export interface ApiErrorPayload {
  code: string;
  message: string;
  status: number;
}

export class ApiClientError extends Error {
  public readonly code: string;
  public readonly status: number;

  constructor(payload: ApiErrorPayload) {
    super(`${payload.code}: ${payload.message}`);
    this.name = "ApiClientError";
    this.code = payload.code;
    this.status = payload.status;
  }
}

export interface ApiClientConfig {
  baseUrl?: string;
  getAccessToken?: () => string | null;
  onSessionIdleTimeout?: () => void;
  fetchImpl?: typeof fetch;
}

export interface LoginResponseDTO {
  mfaRequired: boolean;
  challengeToken: string;
  userRole: UserRole;
}

export interface MfaVerifyResponseDTO {
  accessToken: string;
  expiresAtIso: string;
  idleTimeoutMs: number;
  role: UserRole;
}

export function createApiClient(config: ApiClientConfig = {}) {
  const baseUrl = (
    config.baseUrl ??
    process.env.NEXT_PUBLIC_API_GATEWAY_URL ??
    "https://api.sigo-dental.local"
  ).replace(/\/+$/, "");

  const fetchFn = config.fetchImpl ?? globalThis.fetch;

  async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(init.headers as Record<string, string> | undefined),
    };

    const token = config.getAccessToken?.();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }

    const response = await fetchFn(`${baseUrl}${path}`, {
      ...init,
      credentials: "include",
      headers,
    });

    if (response.status === 401) {
      config.onSessionIdleTimeout?.();
      throw new ApiClientError({
        code: "SESSION_IDLE_TIMEOUT",
        message: "Workstation session expired after 15 minutes of inactivity",
        status: 401,
      });
    }

    if (!response.ok) {
      let errorBody: { code?: string; message?: string } = {};
      try {
        errorBody = (await response.json()) as { code?: string; message?: string };
      } catch {
        errorBody = {};
      }
      throw new ApiClientError({
        code: errorBody.code ?? `HTTP_${response.status}`,
        message: errorBody.message ?? response.statusText,
        status: response.status,
      });
    }

    return (await response.json()) as T;
  }

  return {
    baseUrl,
    login: (email: string, password: string) =>
      request<LoginResponseDTO>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      }),
    verifyMfa: (challengeToken: string, totpCode: string) =>
      request<MfaVerifyResponseDTO>("/api/v1/auth/mfa/verify", {
        method: "POST",
        body: JSON.stringify({ challengeToken, totpCode }),
      }),
    createPatient: (payload: CreatePatientInput) =>
      request<PatientSummaryDTO>("/api/v1/patients", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    updateAnamnesis: (patientId: string, criticalAlerts: PatientSummaryDTO["criticalAlerts"]) =>
      request<PatientSummaryDTO>(`/api/v1/patients/${encodeURIComponent(patientId)}/anamnesis`, {
        method: "PUT",
        body: JSON.stringify({ criticalAlerts }),
      }),
    transitionAppointment: (appointmentId: string, targetStatus: AppointmentStatus) =>
      request<{ id: string; status: AppointmentStatus }>(
        `/api/v1/scheduling/appointments/${encodeURIComponent(appointmentId)}/transition`,
        {
          method: "POST",
          body: JSON.stringify({ targetStatus }),
        }
      ),
    request,
  };
}
