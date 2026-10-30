/**
 * 15-Minute Workstation Idle Session Guard, RBAC Navigation Filter & Shared DTOs (RNF-05, RNF-07, wi-10 S1).
 */

export type UserRole = "admin" | "receptionist" | "dentist" | "assistant" | "auditor";

export interface CriticalAlertDTO {
  code: string;
  label: string;
  severity: "CRITICAL" | "HIGH";
}

export interface PatientSummaryDTO {
  id: string;
  hcNumber: string;
  fullName: string;
  age: number;
  isMinor: boolean;
  insurancePlan?: {
    providerName: string;
    planName: string;
  };
  criticalAlerts: CriticalAlertDTO[];
}

export interface LegalGuardianInput {
  fullName: string;
  documentNumber: string;
  phone: string;
  relationship: string;
}

/** Emergency contact required by the clinical service for every patient (RF-01). */
export interface EmergencyContactInput {
  fullName: string;
  phone: string;
  relationship: string;
}

/** Sex catalog accepted by the clinical service (`F`, `M` or `X`). */
export type PatientSex = "F" | "M" | "X";

export interface CreatePatientInput {
  documentType: "DNI" | "PASSPORT" | "FOREIGN_ID";
  documentNumber: string;
  fullName: string;
  birthDate: string; // YYYY-MM-DD
  /** Optional for backward compatibility; mandatory in the clinical API payload. */
  sex?: PatientSex;
  phone: string;
  email?: string;
  /** Optional for backward compatibility; mandatory in the clinical API payload. */
  emergencyContact?: EmergencyContactInput;
  legalGuardian?: LegalGuardianInput;
  criticalAlerts: CriticalAlertDTO[];
}

export type AppointmentStatus =
  | "PROGRAMADO"
  | "CONFIRMADO"
  | "EN_SALA_DE_ESPERA"
  | "EN_ATENCION"
  | "ATENDIDO"
  | "CANCELADO"
  | "NO_SHOW";

export interface AgendaFilterState {
  view: "day" | "week";
  date: string; // YYYY-MM-DD
  chairId?: string;
  dentistId?: string;
}

/**
 * Escapes HTML special characters to prevent XSS across clinical views (Section 6 Security).
 */
export function escapeHtml(raw: string): string {
  return raw
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

/**
 * Formats integer cents into currency string using tabular numerals (AGENTS.md §3).
 */
export function formatCentsCurrency(cents: number, currencySymbol = "$"): string {
  const sign = cents < 0 ? "-" : "";
  const absCents = Math.abs(Math.trunc(cents));
  const whole = Math.floor(absCents / 100);
  const fraction = String(absCents % 100).padStart(2, "0");
  const wholeFormatted = whole.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return `${sign}${currencySymbol}${wholeFormatted}.${fraction}`;
}

export const DEFAULT_IDLE_TIMEOUT_MS = 900_000; // 15 minutes (900 seconds) per RNF-05

export type WorkstationActivityEvent = "pointerdown" | "keydown" | "touchstart";

export interface SessionIdleGuardOptions {
  timeoutMs?: number;
  nowFn?: () => number;
  onIdleTimeout: () => void;
}

export interface SessionIdleGuard {
  start: () => void;
  recordActivity: (event: WorkstationActivityEvent) => void;
  checkIdle: () => boolean;
  getRemainingMs: () => number;
  isLocked: () => boolean;
}

/**
 * Creates a deterministic 15-minute idle session guard for chairside and reception terminals.
 */
export function createSessionIdleGuard(options: SessionIdleGuardOptions): SessionIdleGuard {
  const timeoutMs = options.timeoutMs ?? DEFAULT_IDLE_TIMEOUT_MS;
  const nowFn = options.nowFn ?? (() => Date.now());
  let lastActivityAt = nowFn();
  let locked = false;

  function start() {
    lastActivityAt = nowFn();
    locked = false;
  }

  function recordActivity(_event: WorkstationActivityEvent) {
    if (locked) {
      return;
    }
    lastActivityAt = nowFn();
  }

  function checkIdle(): boolean {
    if (locked) {
      return true;
    }
    const elapsed = nowFn() - lastActivityAt;
    if (elapsed >= timeoutMs) {
      locked = true;
      options.onIdleTimeout();
      return true;
    }
    return false;
  }

  function getRemainingMs(): number {
    if (locked) {
      return 0;
    }
    const remaining = timeoutMs - (nowFn() - lastActivityAt);
    return remaining > 0 ? remaining : 0;
  }

  function isLocked(): boolean {
    return locked;
  }

  return {
    start,
    recordActivity,
    checkIdle,
    getRemainingMs,
    isLocked,
  };
}

export function validateTotpCode(code: string): { valid: boolean; error?: string } {
  const trimmed = code.trim();
  if (!/^\d{6}$/.test(trimmed)) {
    return {
      valid: false,
      error: "TOTP_INVALID_FORMAT: MFA verification requires exactly 6 numeric digits",
    };
  }
  return { valid: true };
}

export interface NavigationModuleItem {
  id: "agenda" | "patients" | "billing-inventory" | "reports";
  label: string;
  href: string;
  allowedRoles: UserRole[];
}

export const PRIMARY_NAVIGATION_MODULES: NavigationModuleItem[] = [
  {
    id: "agenda",
    label: "Agenda",
    href: "/agenda",
    allowedRoles: ["admin", "receptionist", "dentist", "assistant"],
  },
  {
    id: "patients",
    label: "Pacientes",
    href: "/patients",
    allowedRoles: ["admin", "receptionist", "dentist", "assistant", "auditor"],
  },
  {
    id: "billing-inventory",
    label: "Caja e Inventario",
    href: "/billing",
    allowedRoles: ["admin", "receptionist", "assistant", "auditor"],
  },
  {
    id: "reports",
    label: "Reportes",
    href: "/settlements",
    allowedRoles: ["admin", "dentist", "auditor"],
  },
];

export function getAllowedNavigationModules(role: UserRole): NavigationModuleItem[] {
  return PRIMARY_NAVIGATION_MODULES.filter((mod) => mod.allowedRoles.includes(role));
}
