/**
 * Dentist Fee & Commission Settlements Page (wi-12 S3, RF-13).
 */

import {
  renderDentistSettlementPanelHtml,
  type DentistSettlementPanelProps,
} from "../../../components/billing/DentistSettlementPanel.tsx";

export function renderSettlementsPortalPageHtml(props: DentistSettlementPanelProps): string {
  return `<div class="p-6">
    ${renderDentistSettlementPanelHtml(props)}
  </div>`;
}

export default renderSettlementsPortalPageHtml;
