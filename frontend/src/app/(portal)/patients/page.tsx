/**
 * Unified Patient Directory & Registration Page (wi-10 S2 & S3, RF-01, RF-02).
 */

import { renderPatientRegistrationFormHtml } from "../../../components/patients/PatientRegistrationForm.tsx";
import { renderStickyPatientHeaderHtml } from "../../../components/patients/StickyPatientHeader.tsx";
import type { CreatePatientInput, PatientSummaryDTO } from "../../../lib/session-idle-guard.ts";

export function renderPatientsPortalPageHtml(props: {
  selectedPatient: PatientSummaryDTO;
  draftRegistration: CreatePatientInput;
}): string {
  return `<div class="space-y-6">
    ${renderStickyPatientHeaderHtml(props.selectedPatient)}
    <div class="p-6">
      ${renderPatientRegistrationFormHtml(props.draftRegistration)}
    </div>
  </div>`;
}

export default renderPatientsPortalPageHtml;
