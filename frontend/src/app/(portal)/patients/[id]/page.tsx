/**
 * Patient Clinical Record Workspace Page (wi-11 S1..S5, AGENTS.md §3 Progressive Disclosure).
 * Level 1: StickyPatientHeader with critical medical alerts (RF-02).
 * Level 2: Scoped Clinical Tabs (Odontograma 2D, Periodontograma, Evolución SOEP, Presupuestos, Consentimientos)
 * rendered on pure white (#FFFFFF) active canvas inside cool off-white shell (bg-slate-50).
 */

import { renderStickyPatientHeaderHtml } from "../../../../components/patients/StickyPatientHeader.tsx";
import {
  renderOdontogramCanvasHtml,
  type DentalNotation,
  type OdontogramFindingDTO,
} from "../../../../components/clinical/OdontogramCanvas.tsx";
import {
  renderPeriodontogramGridHtml,
  type PeriodontogramToothDTO,
} from "../../../../components/clinical/PeriodontogramGrid.tsx";
import {
  renderSoepEvolutionTimelineHtml,
  type SoepNoteDTO,
} from "../../../../components/clinical/SoepEvolutionTimeline.tsx";
import {
  renderInformedConsentSheetHtml,
  type ConsentSignMode,
} from "../../../../components/clinical/InformedConsentSheet.tsx";
import {
  renderTreatmentPlanWorkspaceHtml,
  type TreatmentPlanDTO,
} from "../../../../components/treatments/TreatmentPlanWorkspace.tsx";
import type { PatientSummaryDTO } from "../../../../lib/session-idle-guard.ts";

export type ClinicalWorkspaceTab =
  | "ODONTOGRAMA_2D"
  | "PERIODONTOGRAMA"
  | "EVOLUCION_SOEP"
  | "PRESUPUESTOS"
  | "CONSENTIMIENTOS";

export interface PatientClinicalWorkspacePageProps {
  patient: PatientSummaryDTO;
  activeTab: ClinicalWorkspaceTab;
  notation?: DentalNotation;
  odontogramFindings?: OdontogramFindingDTO[];
  periodontogramTeeth?: PeriodontogramToothDTO[];
  soepNotes?: SoepNoteDTO[];
  treatmentPlan?: TreatmentPlanDTO;
  selectedPlanItemIds?: string[];
  consentSheet?: {
    isOpen: boolean;
    consentId: string;
    procedureTitle: string;
    mode: ConsentSignMode;
    svgStrokes: string[];
    otpCode: string;
  };
}

export const CLINICAL_WORKSPACE_TABS: Array<{ id: ClinicalWorkspaceTab; label: string }> = [
  { id: "ODONTOGRAMA_2D", label: "Odontograma 2D" },
  { id: "PERIODONTOGRAMA", label: "Periodontograma" },
  { id: "EVOLUCION_SOEP", label: "Evolución SOEP" },
  { id: "PRESUPUESTOS", label: "Presupuestos" },
  { id: "CONSENTIMIENTOS", label: "Consentimientos" },
];

export function renderPatientClinicalWorkspacePageHtml(
  props: PatientClinicalWorkspacePageProps
): string {
  const tabsHtml = CLINICAL_WORKSPACE_TABS.map(
    (tab) =>
      `<button type="button" role="tab" aria-selected="${
        props.activeTab === tab.id
      }" data-tab="${tab.id}" class="min-h-[44px] px-4 py-2 text-xs font-semibold rounded-t-lg border-b-2 ${
        props.activeTab === tab.id
          ? "border-blue-600 text-blue-700 bg-white"
          : "border-transparent text-slate-600 hover:text-slate-900"
      }">${tab.label}</button>`
  ).join("");

  let activeWorkspaceHtml = "";
  if (props.activeTab === "ODONTOGRAMA_2D") {
    activeWorkspaceHtml = renderOdontogramCanvasHtml({
      patientId: props.patient.id,
      notation: props.notation ?? "FDI",
      findings: props.odontogramFindings ?? [],
    });
  } else if (props.activeTab === "PERIODONTOGRAMA") {
    activeWorkspaceHtml = renderPeriodontogramGridHtml({
      patientId: props.patient.id,
      teethData: props.periodontogramTeeth ?? [],
    });
  } else if (props.activeTab === "EVOLUCION_SOEP") {
    activeWorkspaceHtml = renderSoepEvolutionTimelineHtml({
      patientId: props.patient.id,
      notes: props.soepNotes ?? [],
    });
  } else if (props.activeTab === "PRESUPUESTOS" && props.treatmentPlan) {
    activeWorkspaceHtml = renderTreatmentPlanWorkspaceHtml({
      patientId: props.patient.id,
      plan: props.treatmentPlan,
      selectedItemIds: props.selectedPlanItemIds ?? [],
    });
  }

  const consentSheetHtml = props.consentSheet
    ? renderInformedConsentSheetHtml({
        isOpen: props.consentSheet.isOpen,
        consentId: props.consentSheet.consentId,
        patientName: props.patient.fullName,
        procedureTitle: props.consentSheet.procedureTitle,
        mode: props.consentSheet.mode,
        svgStrokes: props.consentSheet.svgStrokes,
        otpCode: props.consentSheet.otpCode,
      })
    : "";

  return `<div class="min-h-screen bg-slate-50 flex flex-col">
    ${renderStickyPatientHeaderHtml(props.patient)}
    <nav role="tablist" aria-label="Espacios de trabajo clínico" class="px-6 pt-3 bg-slate-100 border-b border-slate-200 flex flex-wrap gap-2">
      ${tabsHtml}
    </nav>
    <div class="p-6 flex-1">
      ${activeWorkspaceHtml}
    </div>
    ${consentSheetHtml}
  </div>`;
}

export default renderPatientClinicalWorkspacePageHtml;
