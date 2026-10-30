/**
 * SlideOverSheet Component (wi-10, wi-11, wi-12, AGENTS.md §3).
 * Reusable contextual drawer over page pushes for scheduling, consent signing, SOEP addenda, and POS checkout.
 */

import { escapeHtml } from "../../lib/session-idle-guard.ts";

export interface SlideOverSheetProps {
  id: string;
  title: string;
  description?: string;
  isOpen: boolean;
  widthClass?: "max-w-md" | "max-w-lg" | "max-w-xl" | "max-w-2xl";
  bodyHtml: string;
  footerHtml?: string;
}

export function renderSlideOverSheetHtml(props: SlideOverSheetProps): string {
  if (!props.isOpen) {
    return "";
  }
  const widthClass = props.widthClass ?? "max-w-lg";
  const titleId = `${props.id}-title`;

  return `<div class="fixed inset-0 z-50 overflow-hidden bg-slate-900/40 flex justify-end" data-sheet-id="${escapeHtml(props.id)}">
    <section role="dialog" aria-modal="true" aria-labelledby="${escapeHtml(titleId)}" class="w-full ${widthClass} bg-white h-full border-l border-slate-200 shadow-xl flex flex-col justify-between">
      <header class="px-6 py-4 border-b border-slate-200 flex items-center justify-between bg-slate-50">
        <div>
          <h2 id="${escapeHtml(titleId)}" class="text-base font-semibold text-slate-900">${escapeHtml(props.title)}</h2>
          ${props.description ? `<p class="text-xs text-slate-600 mt-0.5">${escapeHtml(props.description)}</p>` : ""}
        </div>
        <button type="button" aria-label="Cerrar panel" class="min-h-[44px] min-w-[44px] rounded-md border border-slate-200 bg-white text-slate-700 text-sm font-medium">
          Cerrar
        </button>
      </header>
      <div class="p-6 overflow-y-auto flex-1 bg-white">
        ${props.bodyHtml}
      </div>
      ${
        props.footerHtml
          ? `<footer class="px-6 py-4 border-t border-slate-200 bg-slate-50 flex items-center justify-end gap-3">${props.footerHtml}</footer>`
          : ""
      }
    </section>
  </div>`;
}

export default renderSlideOverSheetHtml;
