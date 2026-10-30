/**
 * Patient Clinical Record Workspace Page (wi-13 S7, AGENTS.md §3).
 *
 * Server component: the clinical header, the odontogram, the periodontogram
 * and the SOEP notes are loaded in parallel with `Promise.allSettled`, so a
 * failing clinical section never breaks the whole record. Each tab shows its
 * own empty or error state, and the tabs without a read endpoint declare it.
 */

import Link from "next/link";
import { redirect } from "next/navigation";
import { renderStickyPatientHeaderHtml } from "../../../../components/patients/StickyPatientHeader.tsx";
import { renderOdontogramCanvasHtml } from "../../../../components/clinical/OdontogramCanvas.tsx";
import { renderPeriodontogramGridHtml } from "../../../../components/clinical/PeriodontogramGrid.tsx";
import { renderSoepEvolutionTimelineHtml } from "../../../../components/clinical/SoepEvolutionTimeline.tsx";
import { GatewayError, createGatewayServerClient } from "../../../../lib/gateway-server.ts";
import {
  buildPatientDetailModel,
  isSessionExpiredError,
  type PatientSectionStatus,
} from "../../../../lib/portal-views.ts";
import { requirePortalSession } from "../../../../lib/session.ts";

type PortalSearchParams = Record<string, string | string[] | undefined>;

const CLINICAL_TABS = [
  { id: "odontogram", label: "Odontograma 2D" },
  { id: "periodontogram", label: "Periodontograma" },
  { id: "soep", label: "Evolución SOEP" },
  { id: "budgets", label: "Presupuestos" },
  { id: "consents", label: "Consentimientos" },
] as const;

type ClinicalTabId = (typeof CLINICAL_TABS)[number]["id"];

const STATUS_LABELS: Record<PatientSectionStatus, string> = {
  ready: "Listo",
  empty: "Sin datos",
  error: "Error",
  unavailable: "No disponible",
};

/** Reads a single query parameter; repeated parameters are ignored. */
function readParam(params: PortalSearchParams, key: string): string {
  const value = params[key];
  return typeof value === "string" ? value : "";
}

/** Resolves the active clinical tab, defaulting to the odontogram. */
function resolveActiveTab(params: PortalSearchParams): ClinicalTabId {
  const requested = readParam(params, "tab");
  const known = CLINICAL_TABS.find((tab) => tab.id === requested);
  return known ? known.id : "odontogram";
}

export default async function PatientClinicalRecordPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<PortalSearchParams>;
}) {
  const session = await requirePortalSession();
  const client = createGatewayServerClient({ accessToken: session.accessToken });
  const { id } = await params;
  const query = await searchParams;

  const [headerResult, odontogramResult, periodontogramResult, soepResult] =
    await Promise.allSettled([
      client.getPatientHeader(id),
      client.getOdontogram(id),
      client.getPeriodontogram(id),
      client.getSoepNotes(id),
    ]);

  const headerError =
    headerResult.status === "rejected" ? headerResult.reason : undefined;

  // An expired workstation session is not a recoverable page state (RNF-05).
  if (isSessionExpiredError(headerError)) {
    redirect("/login?reason=idle_timeout");
  }
  // A missing patient is a "not found" state, never a crash (S7).
  const patientNotFound =
    headerError instanceof GatewayError && headerError.status === 404;

  const model = buildPatientDetailModel({
    patient: headerResult.status === "fulfilled" ? headerResult.value : null,
    odontogram:
      odontogramResult.status === "fulfilled" ? odontogramResult.value : undefined,
    periodontogram:
      periodontogramResult.status === "fulfilled"
        ? { teeth: periodontogramResult.value }
        : undefined,
    soep: soepResult.status === "fulfilled" ? { notes: soepResult.value } : undefined,
    errors: {
      patient: patientNotFound ? undefined : headerError,
      odontogram:
        odontogramResult.status === "rejected" ? odontogramResult.reason : undefined,
      periodontogram:
        periodontogramResult.status === "rejected"
          ? periodontogramResult.reason
          : undefined,
      soep: soepResult.status === "rejected" ? soepResult.reason : undefined,
    },
  });

  if (model.status === "not-found") {
    return (
      <div className="p-6">
        <section
          role="status"
          data-state="not-found"
          data-error-code={model.error?.code ?? "PATIENT_NOT_FOUND"}
          className="p-6 rounded-lg border border-dashed border-slate-300 bg-white text-center space-y-2"
        >
          <h1 className="text-sm font-bold text-slate-900">Paciente no encontrado</h1>
          <p className="text-xs text-slate-600">
            No existe una ficha clínica con el identificador solicitado ({id}).
          </p>
          <Link
            href="/patients"
            className="inline-block text-xs font-semibold text-blue-700 underline"
          >
            Volver al directorio de pacientes
          </Link>
        </section>
      </div>
    );
  }

  if (model.status === "error" || !model.patient) {
    return (
      <div className="p-6">
        <section
          role="alert"
          data-state="error"
          data-error-code={model.error?.code ?? "UNKNOWN_ERROR"}
          className="p-4 rounded-lg border border-red-300 bg-red-50 space-y-2"
        >
          <h1 className="text-sm font-bold text-red-900">
            No se pudo cargar la ficha clínica
          </h1>
          <p className="text-xs text-red-800">
            {model.error?.message ?? "No se pudo obtener la información solicitada."} (
            {model.error?.code ?? "UNKNOWN_ERROR"})
          </p>
          {model.error?.retryable ? (
            <a
              href={`/patients/${encodeURIComponent(id)}`}
              className="inline-block text-xs font-semibold text-red-900 underline"
            >
              Reintentar
            </a>
          ) : null}
        </section>
      </div>
    );
  }

  const patient = model.patient;
  const activeTab = resolveActiveTab(query);
  const patientId = patient.id !== "" ? patient.id : id;

  const sectionOf = (tabId: ClinicalTabId) =>
    model.sections.find((section) => section.id === tabId);

  const unmappedNoticeHtml =
    model.odontogram.unmappedCount > 0 ? (
      <p
        data-unmapped-count={model.odontogram.unmappedCount}
        className="text-xs text-amber-900 bg-amber-50 border border-amber-300 rounded px-3 py-2"
      >
        {model.odontogram.unmappedCount} hallazgos con un catálogo clínico desconocido no
        pudieron representarse en el odontograma.
      </p>
    ) : null;

  return (
    <div className="min-h-full bg-slate-50 flex flex-col">
      <div dangerouslySetInnerHTML={{ __html: renderStickyPatientHeaderHtml(patient) }} />

      <nav
        role="tablist"
        aria-label="Espacios de trabajo clínico"
        className="px-6 pt-3 bg-slate-100 border-b border-slate-200 flex flex-wrap gap-2"
      >
        {CLINICAL_TABS.map((tab) => {
          const section = sectionOf(tab.id);
          const status: PatientSectionStatus = section?.status ?? "unavailable";
          const isActive = tab.id === activeTab;

          return (
            <a
              key={tab.id}
              role="tab"
              aria-selected={isActive}
              href={`/patients/${encodeURIComponent(patientId)}?tab=${tab.id}`}
              className={`min-h-[44px] px-4 py-2 text-xs font-semibold rounded-t-lg border-b-2 flex items-center gap-2 ${
                isActive
                  ? "border-blue-600 text-blue-700 bg-white"
                  : "border-transparent text-slate-600 hover:text-slate-900"
              }`}
            >
              <span>{tab.label}</span>
              <span className="text-[10px] font-medium uppercase tracking-wide text-slate-500">
                {STATUS_LABELS[status]}
              </span>
            </a>
          );
        })}
      </nav>

      <div className="p-6 flex-1 space-y-4">
        {activeTab === "odontogram" ? (
          <section aria-label="Odontograma 2D" className="space-y-3">
            {sectionOf("odontogram")?.status === "error" ? (
              <div
                role="alert"
                data-state="error"
                className="p-3 rounded border border-red-300 bg-red-50 text-xs text-red-800"
              >
                {sectionOf("odontogram")?.note}
              </div>
            ) : null}
            {sectionOf("odontogram")?.status === "empty" ? (
              <div
                data-state="empty"
                className="p-4 rounded-lg border border-dashed border-slate-300 bg-white text-xs text-slate-600"
              >
                {sectionOf("odontogram")?.note}
              </div>
            ) : null}
            {sectionOf("odontogram")?.status === "ready" ? (
              <div
                dangerouslySetInnerHTML={{
                  __html: renderOdontogramCanvasHtml({
                    patientId,
                    notation: "FDI",
                    findings: model.odontogram.findings,
                  }),
                }}
              />
            ) : null}
            {unmappedNoticeHtml}
          </section>
        ) : null}

        {activeTab === "periodontogram" ? (
          <section aria-label="Periodontograma" className="space-y-3">
            {sectionOf("periodontogram")?.status === "error" ? (
              <div
                role="alert"
                data-state="error"
                className="p-3 rounded border border-red-300 bg-red-50 text-xs text-red-800"
              >
                {sectionOf("periodontogram")?.note}
              </div>
            ) : null}
            {sectionOf("periodontogram")?.status === "empty" ? (
              <div
                data-state="empty"
                className="p-4 rounded-lg border border-dashed border-slate-300 bg-white text-xs text-slate-600"
              >
                {sectionOf("periodontogram")?.note}
              </div>
            ) : null}
            {sectionOf("periodontogram")?.status === "ready" ? (
              <div
                dangerouslySetInnerHTML={{
                  __html: renderPeriodontogramGridHtml({
                    patientId,
                    teethData: model.periodontogram.teeth,
                  }),
                }}
              />
            ) : null}
          </section>
        ) : null}

        {activeTab === "soep" ? (
          <section aria-label="Evolución SOEP" className="space-y-3">
            {sectionOf("soep")?.status === "error" ? (
              <div
                role="alert"
                data-state="error"
                className="p-3 rounded border border-red-300 bg-red-50 text-xs text-red-800"
              >
                {sectionOf("soep")?.note}
              </div>
            ) : null}
            {sectionOf("soep")?.status === "empty" ? (
              <div
                data-state="empty"
                className="p-4 rounded-lg border border-dashed border-slate-300 bg-white text-xs text-slate-600"
              >
                {sectionOf("soep")?.note}
              </div>
            ) : null}
            {sectionOf("soep")?.status === "ready" ? (
              <div
                dangerouslySetInnerHTML={{
                  __html: renderSoepEvolutionTimelineHtml({
                    patientId,
                    notes: model.soep.notes,
                  }),
                }}
              />
            ) : null}
          </section>
        ) : null}

        {activeTab === "budgets" ? (
          <section aria-label="Presupuestos" className="space-y-3">
            <div
              data-state="unavailable"
              className="p-4 rounded-lg border border-dashed border-slate-300 bg-white text-xs text-slate-600"
            >
              {sectionOf("budgets")?.note}
            </div>
          </section>
        ) : null}

        {activeTab === "consents" ? (
          <section aria-label="Consentimientos" className="space-y-3">
            <div
              data-state="unavailable"
              className="p-4 rounded-lg border border-dashed border-slate-300 bg-white text-xs text-slate-600"
            >
              {sectionOf("consents")?.note}
            </div>
          </section>
        ) : null}
      </div>
    </div>
  );
}
