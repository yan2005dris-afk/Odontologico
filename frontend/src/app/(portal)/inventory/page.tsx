/**
 * Clinical Supplies Inventory & Procedure Recipes Page (wi-12 S4, RF-14).
 */

import {
  renderInventoryWorkspaceHtml,
  type InventoryWorkspaceProps,
} from "../../../components/inventory/InventoryWorkspace.tsx";

export function renderInventoryPortalPageHtml(props: InventoryWorkspaceProps): string {
  return `<div class="p-6">
    ${renderInventoryWorkspaceHtml(props)}
  </div>`;
}

export default renderInventoryPortalPageHtml;
