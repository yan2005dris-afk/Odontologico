/**
 * Multi-Chair Clinical Agenda Page (wi-13 S8, RF-09, RF-10, RN-03).
 *
 * Server component: the appointments and the patient names are loaded through
 * the API Gateway over the internal network. The appointment state transition is
 * a real HTML form that posts to `/agenda/transition`; a chair overlap conflict
 * (RN-03) is shown without losing that form.
 */

import { redirect } from "next/navigation";
import {
  filterAppointments,
  getAllowedStatusTransitions,
  renderMultiChairCalendarHtml,
  type CalendarAppointmentDTO,
} from "../../../components/agenda/MultiChairCalendar.tsx";
import { createGatewayServerClient } from "../../../lib/gateway-server.ts";
import {
  buildAgendaModel,
  isSessionExpiredError,
} from "../../../lib/portal-views.ts";
import { requirePortalSession } from "../../../lib/session.ts";
import { escapeHtml, type AgendaFilterState } from "../../../lib/session-idle-guard.ts";

type PortalSearchParams = Record<string, string | string[] | undefined>;

/** Reads a single query parameter; repeated parameters are ignored. */
function readParam(params: PortalSearchParams, key: string): string {
  const value = params[key];
  return typeof value === "string" ? value : "";
}

/** Current UTC calendar date (YYYY-MM-DD); the clinic works in UTC. */
function todayIso(): string {
  return new Date().toISOString().slice(0, 10);
}

/** Shifts a YYYY-MM-DD date by a number of days. */
function shiftDate(dateIso: string, days: number): string {
  const parsed = new Date(`${dateIso}T00:00:00Z`);
  if (Number.isNaN(parsed.getTime())) {
    return dateIso;
  }
  parsed.setUTCDate(parsed.getUTCDate() + days);
  return parsed.toISOString().slice(0, 10);
}

export default async function AgendaPage({
  searchParams,
}: {
  searchParams: Promise<PortalSearchParams>;
}) {
  const session = await requirePortalSession();
  const client = createGatewayServerClient({ accessToken: session.accessToken });
  const params = await searchParams;

  const requestedDate = readParam(params, "date");
  const date = /^\d{4}-\d{2}-\d{2}$/.test(requestedDate) ? requestedDate : todayIso();
  const view: AgendaFilterState["view"] = readParam(params, "view") === "week" ? "week" : "day";
  const filter: AgendaFilterState = { view, date };

  let appointments: CalendarAppointmentDTO[] = [];
  let loadError: unknown = undefined;

  try {
    // Patient names come from the real directory; an unknown patient keeps its
    // raw id instead of an invented name.
    const patients = await client.listPatients();
    const patientNames: Record<string, string> = {};
    for (const patient of patients.items) {
      patientNames[patient.id] = patient.fullName;
    }

    const listed = await client.listAppointments(patientNames);
    appointments = listed.items;
  } catch (error) {
    loadError = error;
  }

  // An expired workstation session is not a recoverable page state (RNF-05).
  if (isSessionExpiredError(loadError)) {
    redirect("/login?reason=idle_timeout");
  }

  const conflictCode = readParam(params, "conflict");
  const conflictAppointmentId = readParam(params, "appointment");
  const conflictTarget = readParam(params, "target");
  const updated = readParam(params, "updated") === "1";
  const transitionErrorCode = readParam(params, "error");

  const model = buildAgendaModel({
    appointments,
    filter,
    error: loadError,
    conflict: conflictCode
      ? {
          code: conflictCode,
          message:
            "CHAIR_OVERLAP_CONFLICT: el sillón físico ya se encuentra reservado en ese horario (RN-03).",
          appointmentId: conflictAppointmentId !== "" ? conflictAppointmentId : undefined,
          targetStatus: conflictTarget !== "" ? conflictTarget : undefined,
        }
      : undefined,
  });

  const visibleAppointments = filterAppointments(model.appointments, model.filter);

  // The requested target of a rejected transition stays selected, so the
  // operator can retry the same transition without losing the form (RN-03).
  const transitionControlHtml = (appointment: CalendarAppointmentDTO): string => {
    const allowed = getAllowedStatusTransitions(appointment.status);
    const selectedTarget =
      model.conflict?.appointmentId === appointment.id
        ? (model.conflict.targetStatus ?? "")
        : "";

    const optionsHtml = allowed
      .map(
        (status) =>
          `<option value="${escapeHtml(status)}"${
            status === selectedTarget ? " selected" : ""
          }>${escapeHtml(status)}</option>`
      )
      .join("");

    return `<form method="post" action="/agenda/transition" class="flex items-center gap-2">
          <input type="hidden" name="appointmentId" value="${escapeHtml(appointment.id)}" />
          <input type="hidden" name="date" value="${escapeHtml(model.filter.date)}" />
          <input type="hidden" name="view" value="${escapeHtml(model.filter.view)}" />
          <select name="targetStatus" aria-label="Cambiar estado de cita" class="rounded border border-slate-300 px-2 py-1 text-xs bg-white">
            <option value="">Seleccione el estado destino</option>
            ${optionsHtml}
          </select>
          <button type="submit" class="min-h-[44px] px-3 py-1.5 rounded bg-slate-900 text-white text-xs font-semibold">
            Aplicar
          </button>
        </form>`;
  };

  return (
    <div className="p-6 space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-bold text-slate-900">Agenda</h1>
          <p className="text-xs text-slate-600">
            Agenda multirrecinto (RF-09) con máquina de estados de citas (RF-10).
          </p>
        </div>
        <nav className="flex flex-wrap items-center gap-2 text-xs">
          <a
            href={`/agenda?date=${shiftDate(date, -1)}&view=${view}`}
            className="min-h-[44px] inline-flex items-center px-3 py-2 rounded border border-slate-300 bg-white font-semibold text-slate-700"
          >
            Día anterior
          </a>
          <a
            href={`/agenda?date=${todayIso()}&view=${view}`}
            className="min-h-[44px] inline-flex items-center px-3 py-2 rounded border border-slate-300 bg-white font-semibold text-slate-700"
          >
            Hoy
          </a>
          <a
            href={`/agenda?date=${shiftDate(date, 1)}&view=${view}`}
            className="min-h-[44px] inline-flex items-center px-3 py-2 rounded border border-slate-300 bg-white font-semibold text-slate-700"
          >
            Día siguiente
          </a>
          <a
            href={`/agenda?date=${date}&view=${view === "day" ? "week" : "day"}`}
            className="min-h-[44px] inline-flex items-center px-3 py-2 rounded border border-slate-300 bg-white font-semibold text-slate-700"
          >
            {view === "day" ? "Ver semana" : "Ver día"}
          </a>
        </nav>
      </header>

      <form method="get" action="/agenda" className="flex flex-wrap items-end gap-2">
        <div>
          <label className="block text-xs font-medium text-slate-700" htmlFor="agenda-date">
            Fecha de la agenda
          </label>
          <input
            id="agenda-date"
            type="date"
            name="date"
            defaultValue={date}
            className="mt-1 rounded border border-slate-300 px-3 py-2 text-sm tabular-nums bg-white"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-700" htmlFor="agenda-view">
            Vista
          </label>
          <select
            id="agenda-view"
            name="view"
            defaultValue={view}
            className="mt-1 rounded border border-slate-300 px-3 py-2 text-sm bg-white"
          >
            <option value="day">Día</option>
            <option value="week">Semana</option>
          </select>
        </div>
        <button
          type="submit"
          className="min-h-[44px] px-4 py-2 rounded bg-slate-900 text-white text-sm font-semibold"
        >
          Aplicar filtros
        </button>
      </form>

      {updated ? (
        <div
          role="status"
          data-banner="updated"
          className="p-3 rounded border border-emerald-300 bg-emerald-50 text-xs font-semibold text-emerald-900"
        >
          El estado de la cita fue actualizado.
        </div>
      ) : null}

      {transitionErrorCode ? (
        <div
          role="alert"
          data-banner="transition-error"
          data-error-code={transitionErrorCode}
          className="p-3 rounded border border-red-300 bg-red-50 text-xs font-semibold text-red-800"
        >
          No se pudo actualizar la cita ({transitionErrorCode}).
        </div>
      ) : null}

      {model.conflict ? (
        <div
          data-conflict-code={model.conflict.code}
          data-conflict-appointment={model.conflict.appointmentId ?? ""}
          data-conflict-target={model.conflict.targetStatus ?? ""}
          className="px-3 py-2 rounded border border-slate-300 bg-white text-xs text-slate-700"
        >
          <p>
            <span className="font-semibold text-red-800">{model.conflict.code}</span>
            {" · "}
            {model.conflict.appointmentId ? (
              <span className="tabular-nums">
                Cita {model.conflict.appointmentId}
                {model.conflict.targetStatus
                  ? ` · estado solicitado: ${model.conflict.targetStatus}`
                  : ""}
              </span>
            ) : null}
          </p>
        </div>
      ) : null}

      {model.status === "error" && model.error ? (
        <section
          role="alert"
          data-state="error"
          data-error-code={model.error.code}
          className="p-4 rounded-lg border border-red-300 bg-red-50 space-y-2"
        >
          <h2 className="text-sm font-bold text-red-900">No se pudo cargar la agenda</h2>
          <p className="text-xs text-red-800">
            {model.error.message} ({model.error.code})
          </p>
          {model.error.retryable ? (
            <a
              href={`/agenda?date=${date}&view=${view}`}
              className="inline-block text-xs font-semibold text-red-900 underline"
            >
              Reintentar
            </a>
          ) : null}
        </section>
      ) : null}

      {model.status !== "error" && visibleAppointments.length === 0 ? (
        <section
          data-state="empty"
          className="p-6 rounded-lg border border-dashed border-slate-300 bg-white text-center space-y-2"
        >
          <h2 className="text-sm font-bold text-slate-900">Sin citas para esta fecha</h2>
          <p className="text-xs text-slate-600">
            {view === "day"
              ? `No hay citas programadas para el ${date}.`
              : `No hay citas programadas para la semana del ${date}.`}{" "}
            Use los filtros de fecha para consultar otro día.
          </p>
        </section>
      ) : null}

      {model.status !== "error" && visibleAppointments.length > 0 ? (
        <div
          dangerouslySetInnerHTML={{
            __html: renderMultiChairCalendarHtml({
              appointments: model.appointments,
              filter: model.filter,
              overlapError: model.conflict?.message,
              transitionControlHtml,
            }),
          }}
        />
      ) : null}
    </div>
  );
}
