/**
 * MultiChairCalendar Component (wi-10 S4 & S5, RF-09, RF-10, RN-03).
 * Multi-chair/multi-specialist calendar view, physical dental chair overlap prevention (RN-03),
 * and 7-state appointment lifecycle state machine (RF-10).
 */

import {
  escapeHtml,
  type AgendaFilterState,
  type AppointmentStatus,
} from "../../lib/session-idle-guard.ts";

export interface CalendarAppointmentDTO {
  id: string;
  patientId: string;
  patientName: string;
  chairId: string;
  chairLabel: string;
  dentistId: string;
  dentistName: string;
  date: string; // YYYY-MM-DD
  startTime: string; // HH:mm
  endTime: string; // HH:mm
  status: AppointmentStatus;
  procedureSummary: string;
}

export interface ProposedAppointmentSlot {
  chairId: string;
  dentistId: string;
  date: string; // YYYY-MM-DD
  startTime: string; // HH:mm
  endTime: string; // HH:mm
  excludeAppointmentId?: string;
}

export interface ChairOverlapCheckResult {
  hasConflict: boolean;
  errorCode?: "CHAIR_OVERLAP_CONFLICT";
  conflictingAppointmentId?: string;
  message?: string;
}

const APPOINTMENT_STATE_GRAPH: Record<AppointmentStatus, AppointmentStatus[]> = {
  PROGRAMADO: ["CONFIRMADO", "EN_SALA_DE_ESPERA", "CANCELADO", "NO_SHOW"],
  CONFIRMADO: ["EN_SALA_DE_ESPERA", "CANCELADO", "NO_SHOW"],
  EN_SALA_DE_ESPERA: ["EN_ATENCION", "CANCELADO", "NO_SHOW"],
  EN_ATENCION: ["ATENDIDO"],
  ATENDIDO: [],
  CANCELADO: [],
  NO_SHOW: [],
};

function parseMinutesSinceMidnight(hhmm: string): number {
  const [hours, minutes] = hhmm.split(":").map(Number);
  return (hours || 0) * 60 + (minutes || 0);
}

export function filterAppointments(
  appointments: CalendarAppointmentDTO[],
  filter: AgendaFilterState
): CalendarAppointmentDTO[] {
  return appointments.filter((apt) => {
    if (filter.view === "day" && apt.date !== filter.date) {
      return false;
    }
    if (filter.chairId && apt.chairId !== filter.chairId) {
      return false;
    }
    if (filter.dentistId && apt.dentistId !== filter.dentistId) {
      return false;
    }
    return true;
  });
}

/**
 * Enforces RN-03: Never allow two simultaneous appointments on the same physical dental chair (`sillón`),
 * regardless of whether the dentists are different.
 */
export function detectChairOverlapConflict(
  existingAppointments: CalendarAppointmentDTO[],
  proposed: ProposedAppointmentSlot
): ChairOverlapCheckResult {
  const proposedStart = parseMinutesSinceMidnight(proposed.startTime);
  const proposedEnd = parseMinutesSinceMidnight(proposed.endTime);

  for (const apt of existingAppointments) {
    if (proposed.excludeAppointmentId && apt.id === proposed.excludeAppointmentId) {
      continue;
    }
    if (apt.status === "CANCELADO" || apt.status === "NO_SHOW") {
      continue;
    }
    if (apt.chairId !== proposed.chairId || apt.date !== proposed.date) {
      continue;
    }

    const existingStart = parseMinutesSinceMidnight(apt.startTime);
    const existingEnd = parseMinutesSinceMidnight(apt.endTime);

    if (proposedStart < existingEnd && proposedEnd > existingStart) {
      return {
        hasConflict: true,
        errorCode: "CHAIR_OVERLAP_CONFLICT",
        conflictingAppointmentId: apt.id,
        message: `CHAIR_OVERLAP_CONFLICT: El sillón físico (${apt.chairLabel}) ya se encuentra reservado entre ${apt.startTime} y ${apt.endTime} (RN-03).`,
      };
    }
  }

  return { hasConflict: false };
}

export function getAllowedStatusTransitions(current: AppointmentStatus): AppointmentStatus[] {
  return APPOINTMENT_STATE_GRAPH[current] ?? [];
}

export function canTransitionAppointmentStatus(
  current: AppointmentStatus,
  target: AppointmentStatus
): boolean {
  return getAllowedStatusTransitions(current).includes(target);
}

export function renderMultiChairCalendarHtml(props: {
  appointments: CalendarAppointmentDTO[];
  filter: AgendaFilterState;
  overlapError?: string;
}): string {
  const visible = filterAppointments(props.appointments, props.filter);

  const alertHtml = props.overlapError
    ? `<div role="alert" class="p-3 rounded-md bg-red-600 text-white text-xs font-semibold">
        ${escapeHtml(props.overlapError)}
      </div>`
    : "";

  const rowsHtml = visible
    .map((apt) => {
      const nextStates = getAllowedStatusTransitions(apt.status);
      const transitionsHtml = nextStates
        .map(
          (st) =>
            `<option value="${st}">${st}</option>`
        )
        .join("");

      return `<article class="p-4 rounded-lg bg-white border border-slate-200 flex items-center justify-between gap-4" data-appointment-id="${escapeHtml(apt.id)}">
        <div>
          <div class="flex items-center gap-2">
            <span class="font-mono text-xs font-bold tabular-nums text-slate-900">${escapeHtml(apt.startTime)} - ${escapeHtml(apt.endTime)}</span>
            <span class="text-xs px-2 py-0.5 rounded bg-slate-100 text-slate-800 font-medium">${escapeHtml(apt.chairLabel)}</span>
            <span class="text-xs px-2 py-0.5 rounded bg-blue-50 text-blue-800 font-semibold">${escapeHtml(apt.status)}</span>
          </div>
          <p class="text-sm font-semibold text-slate-900 mt-1">${escapeHtml(apt.patientName)} · ${escapeHtml(apt.procedureSummary)}</p>
          <p class="text-xs text-slate-600">Especialista: ${escapeHtml(apt.dentistName)}</p>
        </div>
        <div class="flex items-center gap-2">
          <label class="text-xs text-slate-600">Transición (RF-10):</label>
          <select aria-label="Cambiar estado de cita" class="rounded border border-slate-300 px-2 py-1 text-xs bg-white">
            <option value="">Actual: ${escapeHtml(apt.status)}</option>
            ${transitionsHtml}
          </select>
        </div>
      </article>`;
    })
    .join("\n");

  return `<section class="space-y-4" aria-label="Agenda Multirrecinto SIGO-Dental">
    <header class="bg-white p-4 rounded-lg border border-slate-200 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-bold text-slate-900">Agenda Multirrecinto (${props.filter.view === "day" ? "Vista Día" : "Vista Semana"})</h2>
        <p class="text-xs text-slate-600 tabular-nums">Fecha activa: ${escapeHtml(props.filter.date)}</p>
      </div>
    </header>
    ${alertHtml}
    <div class="space-y-2">
      ${rowsHtml || `<p class="text-xs text-slate-500 p-4 bg-white rounded border border-slate-200">Sin turnos para los filtros seleccionados.</p>`}
    </div>
  </section>`;
}

export default renderMultiChairCalendarHtml;
