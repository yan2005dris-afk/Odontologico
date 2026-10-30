/**
 * SoepEvolutionTimeline Component (wi-11 S2, RF-05, RN-01).
 * Chronological clinical evolution log (Subjective, Objective, Evaluation/Assessment, Plan)
 * enforcing the 24-hour immutability lock window (RN-01) and SlideOverSheet for Signed Addenda.
 */

import { escapeHtml } from "../../lib/session-idle-guard.ts";
import { renderSlideOverSheetHtml } from "../shell/SlideOverSheet.tsx";

export interface SoepAddendumDTO {
  id: string;
  authorName: string;
  signedAtIso: string;
  content: string;
}

export interface SoepNoteDTO {
  id: string;
  patientId: string;
  authorName: string;
  createdAtIso: string;
  isLocked: boolean;
  subjective: string;
  objective: string;
  assessment: string;
  plan: string;
  addenda: SoepAddendumDTO[];
}

export const SOEP_LOCK_WINDOW_MS = 24 * 60 * 60 * 1000; // 24 hours per RF-05 / RN-01

export function isSoepNoteLockedByWindow(
  createdAtIso: string,
  nowIso: string,
  explicitLockFlag = false
): boolean {
  if (explicitLockFlag) {
    return true;
  }
  const createdMs = Date.parse(createdAtIso);
  const nowMs = Date.parse(nowIso);
  if (Number.isNaN(createdMs) || Number.isNaN(nowMs)) {
    return true;
  }
  return nowMs - createdMs >= SOEP_LOCK_WINDOW_MS;
}

export function validateSoepAddendumInput(input: {
  noteId: string;
  content: string;
  confirmSignature: boolean;
}): { valid: boolean; errorCode?: string } {
  if (!input.noteId.trim() || !input.content.trim()) {
    return { valid: false, errorCode: "ADDENDUM_CONTENT_REQUIRED" };
  }
  if (!input.confirmSignature) {
    return { valid: false, errorCode: "PROFESSIONAL_SIGNATURE_REQUIRED" };
  }
  return { valid: true };
}

export function renderSoepEvolutionTimelineHtml(props: {
  patientId: string;
  notes: SoepNoteDTO[];
  nowIso?: string;
  activeAddendumNoteId?: string;
}): string {
  const nowIso = props.nowIso ?? "2026-10-30T12:00:00Z";

  const notesHtml = props.notes
    .map((note) => {
      const locked = isSoepNoteLockedByWindow(note.createdAtIso, nowIso, note.isLocked);
      const addendaHtml = note.addenda
        .map(
          (add) =>
            `<div class="mt-2 p-3 rounded bg-slate-50 border border-slate-200 text-xs">
              <p class="font-semibold text-slate-800">Adenda Firmada · ${escapeHtml(add.authorName)} · <span class="tabular-nums">${escapeHtml(add.signedAtIso)}</span></p>
              <p class="text-slate-700 mt-1">${escapeHtml(add.content)}</p>
            </div>`
        )
        .join("");

      const actionButtonsHtml = locked
        ? `<div class="flex items-center gap-2">
            <span class="text-xs px-2 py-0.5 rounded bg-slate-800 text-white font-semibold">Bloqueada (RN-01 · &gt;24h)</span>
            <button type="button" data-action="open-addendum-sheet" class="min-h-[44px] px-3 py-1.5 rounded bg-blue-600 text-white text-xs font-semibold">
              Agregar Adenda Firmada
            </button>
          </div>`
        : `<div class="flex items-center gap-2">
            <button type="button" data-action="edit-soep" class="min-h-[44px] px-3 py-1.5 rounded border border-slate-300 text-xs font-medium">
              Editar Nota (Ventana &lt;24h)
            </button>
          </div>`;

      return `<article class="p-4 rounded-lg bg-white border border-slate-200 space-y-3" data-soep-id="${escapeHtml(note.id)}">
        <header class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-100 pb-2">
          <div>
            <span class="text-sm font-bold text-slate-900">${escapeHtml(note.authorName)}</span>
            <span class="text-xs font-mono tabular-nums text-slate-500 ml-2">${escapeHtml(note.createdAtIso)}</span>
          </div>
          ${actionButtonsHtml}
        </header>
        <dl class="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
          <div><dt class="font-bold text-slate-700">S — Subjetivo</dt><dd class="text-slate-800 mt-0.5">${escapeHtml(note.subjective)}</dd></div>
          <div><dt class="font-bold text-slate-700">O — Objetivo</dt><dd class="text-slate-800 mt-0.5">${escapeHtml(note.objective)}</dd></div>
          <div><dt class="font-bold text-slate-700">E — Evaluación</dt><dd class="text-slate-800 mt-0.5">${escapeHtml(note.assessment)}</dd></div>
          <div><dt class="font-bold text-slate-700">P — Plan</dt><dd class="text-slate-800 mt-0.5">${escapeHtml(note.plan)}</dd></div>
        </dl>
        ${addendaHtml}
      </article>`;
    })
    .join("\n");

  const addendumSheetHtml = props.activeAddendumNoteId
    ? renderSlideOverSheetHtml({
        id: `soep-addendum-${props.activeAddendumNoteId}`,
        title: "Agregar Adenda Firmada (RN-01)",
        description:
          "Las notas SOEP bloqueadas tras 24h son inmutables. Toda rectificación queda anexada como adenda con firma digital y sello de tiempo.",
        isOpen: true,
        bodyHtml: `<form class="space-y-4">
          <div>
            <label class="block text-xs font-medium text-slate-700">Texto de Rectificación / Adenda Clínica *</label>
            <textarea rows="4" required class="mt-1 w-full rounded border border-slate-300 p-2 text-sm"></textarea>
          </div>
          <label class="flex items-center gap-2 text-xs text-slate-800">
            <input type="checkbox" required />
            <span>Confirmo firma digital profesional e inmutabilidad de esta adenda (RN-01)</span>
          </label>
        </form>`,
        footerHtml: `<button type="submit" class="min-h-[44px] px-4 py-2 rounded bg-slate-900 text-white text-xs font-semibold">Firmar y Anexar Adenda</button>`,
      })
    : "";

  return `<section class="space-y-4" aria-label="Evolución Clínica SOEP">
    ${notesHtml}
    ${addendumSheetHtml}
  </section>`;
}

export default renderSoepEvolutionTimelineHtml;
