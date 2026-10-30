/**
 * Billing, Mixed-Payment POS & Blind Cash Close Page (wi-12 S1 & S2, RF-12, RN-02).
 */

import {
  renderPosCheckoutSheetHtml,
  type PosCheckoutSheetProps,
} from "../../../components/billing/PosCheckoutSheet.tsx";
import {
  renderBlindCashCloseViewHtml,
  type BlindCashCloseViewProps,
} from "../../../components/billing/BlindCashCloseView.tsx";

export function renderBillingPortalPageHtml(props: {
  blindClose: BlindCashCloseViewProps;
  posSheet?: PosCheckoutSheetProps;
}): string {
  return `<div class="p-6 space-y-6">
    ${renderBlindCashCloseViewHtml(props.blindClose)}
    ${props.posSheet ? renderPosCheckoutSheetHtml(props.posSheet) : ""}
  </div>`;
}

export default renderBillingPortalPageHtml;
