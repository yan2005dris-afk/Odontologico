/**
 * Patient Directory & Registration Page (wi-13 S6, RF-01, RF-02).
 *
 * Server component: the session is read from the httpOnly cookie, the patients
 * are loaded through the API Gateway over the internal network (same-origin
 * data flow, S4/S5) and the ready / empty / error states are delegated to the
 * pure view model of `portal-views.ts`.
 */

import Link from "next/link";
import { redirect } from "next/navigation";
import { renderCriticalAlertBadgesHtml } from "../../../components/patients/StickyPatientHeader.tsx";
import { renderPatientRegistrationFormHtml } from "../../../components/patients/PatientRegistrationForm.tsx";
import { createGatewayServerClient } from "../../../lib/gateway-server.ts";
import {
  buildPatientsListModel,
  isSessionExpiredError,
} from "../../../lib/portal-views.ts";
import { requirePortalSession } from "../../../lib/session.ts";
import type { CreatePatientInput, PatientSummaryDTO } from "../../../lib/session-idle-guard.ts";

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

/** Blank intake draft rendered by the registration form. */
const EMPTY_DRAFT: CreatePatientInput = {
  documentType: "DNI",
  documentNumber: "",
  fullName: "",
  birthDate: "",
  sex: "F",
  phone: "",
  email: "",
  emergencyContact: { fullName: "", phone: "", relationship: "" },
  legalGuardian: { fullName: "", documentNumber: "", phone: "", relationship: "" },
  criticalAlerts: [],
};

export default async function PatientsPage({
  searchParams,
}: {
  searchParams: Promise<PortalSearchParams>;
}) {
  const session = await requirePortalSession();
  const client = createGatewayServerClient({ accessToken: session.accessToken });
  const params = await searchParams;

  let listed: { items: PatientSummaryDTO[]; total: number } | null = null;
  let listError: unknown = undefined;

  try {
    listed = await client.listPatients();
  } catch (error) {
    listError = error;
  }

  // An expired workstation session is not a recoverable page state (RNF-05).
  if (isSessionExpiredError(listError)) {
    redirect("/login?reason=idle_timeout");
  }

  // The clinical header carries the RF-02 critical alerts; a failing header
  // falls back to the directory item instead of failing the whole page (S6).
  const patients = await Promise.all(
    (listed?.items ?? []).map(async (summary) => {
      try {
        return await client.getPatientHeader(summary.id);
      } catch {
        return summary;
      }
    })
  );

  const model = buildPatientsListModel({
    patients,
    total: listed?.total,
    error: listError,
  });

  const created = readParam(params, "created") === "1";
  const intakeErrorCode = readParam(params, "error");

  return (
    <div className="p-6 space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-bold text-slate-900">Pacientes</h1>
          <p className="text-xs text-slate-600">
            Ficha única del paciente (RF-01) y alertas médicas críticas (RF-02).
          </p>
        </div>
        <span className="text-xs tabular-nums text-slate-600">
          Total de pacientes: {model.total}
        </span>
      </header>

      {created ? (
        <div
          role="status"
          data-banner="created"
          className="p-3 rounded border border-emerald-300 bg-emerald-50 text-xs font-semibold text-emerald-900"
        >
          Paciente registrado correctamente.
        </div>
      ) : null}

      {intakeErrorCode ? (
        <div
          role="alert"
          data-banner="intake-error"
          data-error-code={intakeErrorCode}
          className="p-3 rounded border border-red-300 bg-red-50 text-xs font-semibold text-red-800"
        >
          No se pudo registrar el paciente ({intakeErrorCode}). Revise los datos del
          formulario e intente nuevamente.
        </div>
      ) : null}

      {model.status === "error" && model.error ? (
        <section
          role="alert"
          data-state="error"
          data-error-code={model.error.code}
          className="p-4 rounded-lg border border-red-300 bg-red-50 space-y-2"
        >
          <h2 className="text-sm font-bold text-red-900">
            No se pudo cargar el directorio de pacientes
          </h2>
          <p className="text-xs text-red-800">
            {model.error.message} ({model.error.code})
          </p>
          {model.error.retryable ? (
            <Link
              href="/patients"
              className="inline-block text-xs font-semibold text-red-900 underline"
            >
              Reintentar
            </Link>
          ) : null}
        </section>
      ) : null}

      {model.status === "empty" ? (
        <section
          data-state="empty"
          className="p-6 rounded-lg border border-dashed border-slate-300 bg-white text-center space-y-2"
        >
          <h2 className="text-sm font-bold text-slate-900">No hay pacientes</h2>
          <p className="text-xs text-slate-600">
            Todavía no hay pacientes registrados en la clínica. Use el formulario de alta
            para crear la primera ficha clínica.
          </p>
        </section>
      ) : null}

      {model.status === "ready" ? (
        <ul className="divide-y divide-slate-200 bg-white rounded-lg border border-slate-200">
          {model.patients.map((patient) => {
            const badgesHtml = renderCriticalAlertBadgesHtml(patient.criticalAlerts);

            return (
              <li
                key={patient.id}
                data-patient-id={patient.id}
                className="p-4 flex flex-wrap items-center justify-between gap-3"
              >
                <div className="min-w-0">
                  <a
                    href={`/patients/${encodeURIComponent(patient.id)}`}
                    className="text-sm font-semibold text-blue-700 hover:underline"
                  >
                    {patient.fullName}
                  </a>
                  <div className="flex flex-wrap items-center gap-2 mt-1">
                    <span className="text-xs font-mono tabular-nums px-2 py-0.5 rounded bg-slate-100 text-slate-700 border border-slate-200">
                      {patient.hcNumber}
                    </span>
                    <span className="text-xs tabular-nums text-slate-600">
                      {patient.age} años
                    </span>
                    {patient.isMinor ? (
                      <span className="text-xs font-semibold px-2 py-0.5 rounded bg-amber-100 text-amber-950 border border-amber-400">
                        Menor de edad
                      </span>
                    ) : null}
                  </div>
                </div>
                <div aria-live="assertive" className="flex flex-wrap items-center gap-2">
                  {badgesHtml ? (
                    <div dangerouslySetInnerHTML={{ __html: badgesHtml }} />
                  ) : (
                    <span className="text-xs text-slate-500">
                      Sin alertas médicas críticas registradas
                    </span>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      ) : null}

      <section aria-label="Alta de paciente" className="space-y-3">
        <h2 className="text-base font-bold text-slate-900">Alta de Paciente</h2>
        <div
          dangerouslySetInnerHTML={{
            __html: renderPatientRegistrationFormHtml(EMPTY_DRAFT, todayIso(), {
              formAction: "/patients/create",
              formMethod: "post",
            }),
          }}
        />
      </section>
    </div>
  );
}
