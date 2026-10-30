/**
 * StickyPatientHeader Component (wi-10 S3, RF-02, AGENTS.md §3).
 * Permanently displays high-visibility semantic medical alerts (penicillin allergy, anticoagulants, etc.)
 * with `sticky top-0` and `role="alert"`.
 */

import {
  escapeHtml,
  type CriticalAlertDTO,
  type PatientSummaryDTO,
} from "../../lib/session-idle-guard.ts";

export interface AlertBadgeViewModel {
  code: string;
  label: string;
  severity: "CRITICAL" | "HIGH";
  ariaRole: "alert";
  badgeClass: string;
}

export interface StickyPatientHeaderViewModel {
  isSticky: true;
  stickyClass: string;
  hasCriticalAlerts: boolean;
  alertBadges: AlertBadgeViewModel[];
}

/**
 * Semantic class of a critical alert badge (RF-02). Kept in a single place so
 * the sticky header and the patient list rows cannot drift apart.
 */
function criticalAlertBadgeClass(severity: CriticalAlertDTO["severity"]): string {
  return severity === "CRITICAL"
    ? "bg-red-600 text-white border border-red-700 font-semibold"
    : "bg-amber-100 text-amber-950 border border-amber-400 font-semibold";
}

export function buildStickyPatientHeaderModel(
  patient: PatientSummaryDTO
): StickyPatientHeaderViewModel {
  const alertBadges: AlertBadgeViewModel[] = patient.criticalAlerts.map((alert: CriticalAlertDTO) => ({
    code: alert.code,
    label: alert.label,
    severity: alert.severity,
    ariaRole: "alert",
    badgeClass: criticalAlertBadgeClass(alert.severity),
  }));

  return {
    isSticky: true,
    stickyClass: "sticky top-0 z-30 bg-white border-b border-slate-200 px-6 py-3",
    hasCriticalAlerts: alertBadges.length > 0,
    alertBadges,
  };
}

/**
 * Renders the RF-02 critical alert badges of a patient.
 *
 * This is the single source of the badge markup: the sticky clinical header and
 * the patient list rows both call it, so an alert can never be rendered with
 * two different visual treatments.
 */
export function renderCriticalAlertBadgesHtml(alerts: CriticalAlertDTO[]): string {
  if (!Array.isArray(alerts) || alerts.length === 0) {
    return "";
  }

  return alerts
    .map(
      (alert) =>
        `<span role="alert" data-alert-code="${escapeHtml(alert.code)}" class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs ${criticalAlertBadgeClass(alert.severity)}">
          <span>ALERTA ${escapeHtml(alert.severity)}:</span>
          <span>${escapeHtml(alert.label)}</span>
        </span>`
    )
    .join("\n");
}

export function renderStickyPatientHeaderHtml(patient: PatientSummaryDTO): string {
  const model = buildStickyPatientHeaderModel(patient);

  const badgesHtml = renderCriticalAlertBadgesHtml(patient.criticalAlerts);

  const insuranceText = patient.insurancePlan
    ? `${escapeHtml(patient.insurancePlan.providerName)} · ${escapeHtml(patient.insurancePlan.planName)}`
    : "Particular";

  return `<header role="banner" class="${model.stickyClass}" data-patient-id="${escapeHtml(patient.id)}">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <div class="flex items-center gap-3">
          <h1 class="text-lg font-bold text-slate-900">${escapeHtml(patient.fullName)}</h1>
          <span class="text-xs font-mono tabular-nums px-2 py-0.5 rounded bg-slate-100 text-slate-700 border border-slate-200">
            ${escapeHtml(patient.hcNumber)}
          </span>
          <span class="text-xs tabular-nums text-slate-600">${patient.age} años${patient.isMinor ? " (Menor)" : ""}</span>
        </div>
        <p class="text-xs text-slate-500 mt-0.5">Cobertura: ${insuranceText}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2" aria-live="assertive">
        ${
          model.hasCriticalAlerts
            ? badgesHtml
            : `<span class="text-xs text-slate-500">Sin alertas médicas críticas registradas</span>`
        }
      </div>
    </div>
  </header>`;
}

export default renderStickyPatientHeaderHtml;
