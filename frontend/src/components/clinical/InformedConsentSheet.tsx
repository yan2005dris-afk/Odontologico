/**
 * InformedConsentSheet Component (wi-11 S4, RF-06, RNF-07, AGENTS.md §3).
 * Slide-over Sheet with HTML5/SVG biometric touch pad (targets >= 44x44px for chairside tablets >= 10.5'')
 * and 6-digit OTP verification fallback.
 */

import { escapeHtml } from "../../lib/session-idle-guard.ts";
import { renderSlideOverSheetHtml } from "../shell/SlideOverSheet.tsx";

export type ConsentSignMode = "BIOMETRIC_PAD" | "OTP_CODE";

export interface InformedConsentSignInput {
  consentId: string;
  mode: ConsentSignMode;
  svgStrokes: string[];
  otpCode: string;
}

export interface InformedConsentSheetProps {
  isOpen: boolean;
  consentId: string;
  patientName: string;
  procedureTitle: string;
  mode: ConsentSignMode;
  svgStrokes: string[];
  otpCode: string;
  onSignConsent?: (payload: InformedConsentSignInput) => Promise<void>;
}

/**
 * Validates biometric signature strokes or 6-digit numeric OTP code per RF-06 / S4.
 */
export function validateInformedConsentSubmission(
  input: InformedConsentSignInput
): { valid: boolean; errorCode?: "BIOMETRIC_SIGNATURE_REQUIRED" | "INVALID_OTP_CODE" } {
  if (input.mode === "BIOMETRIC_PAD") {
    const hasValidStroke = input.svgStrokes.some((s) => s.trim().length >= 5);
    if (!hasValidStroke) {
      return {
        valid: false,
        errorCode: "BIOMETRIC_SIGNATURE_REQUIRED",
      };
    }
    return { valid: true };
  }

  const trimmedOtp = input.otpCode.trim();
  if (!/^\d{6}$/.test(trimmedOtp)) {
    return {
      valid: false,
      errorCode: "INVALID_OTP_CODE",
    };
  }
  return { valid: true };
}

export function renderInformedConsentSheetHtml(props: InformedConsentSheetProps): string {
  const validation = validateInformedConsentSubmission({
    consentId: props.consentId,
    mode: props.mode,
    svgStrokes: props.svgStrokes,
    otpCode: props.otpCode,
  });

  const strokesSvg = props.svgStrokes
    .map(
      (pathD) =>
        `<path d="${escapeHtml(pathD)}" fill="none" stroke="#0F172A" stroke-width="2.5" stroke-linecap="round" />`
    )
    .join("");

  const bodyHtml = `<div class="space-y-4">
    <div class="p-3 rounded-lg bg-slate-50 border border-slate-200 text-xs text-slate-700 space-y-1">
      <p><span class="font-semibold text-slate-900">Paciente:</span> ${escapeHtml(props.patientName)}</p>
      <p><span class="font-semibold text-slate-900">Procedimiento Clínico:</span> ${escapeHtml(props.procedureTitle)}</p>
      <p class="text-[11px] text-slate-600 mt-1">Declaro haber sido informado/a sobre los riesgos, alternativas y cuidados postoperatorios conforme a RF-06.</p>
    </div>

    <div class="flex items-center gap-2" role="tablist" aria-label="Método de firma de consentimiento">
      <button type="button" role="tab" aria-selected="${props.mode === "BIOMETRIC_PAD"}" class="min-h-[44px] px-3 py-2 rounded border text-xs font-semibold ${
        props.mode === "BIOMETRIC_PAD"
          ? "bg-slate-900 text-white border-slate-900"
          : "bg-white text-slate-700 border-slate-300"
      }">
        Pad Biométrico Táctil
      </button>
      <button type="button" role="tab" aria-selected="${props.mode === "OTP_CODE"}" class="min-h-[44px] px-3 py-2 rounded border text-xs font-semibold ${
        props.mode === "OTP_CODE"
          ? "bg-slate-900 text-white border-slate-900"
          : "bg-white text-slate-700 border-slate-300"
      }">
        Código OTP (6 Dígitos)
      </button>
    </div>

    ${
      props.mode === "BIOMETRIC_PAD"
        ? `<div class="space-y-2">
            <label class="block text-xs font-medium text-slate-700">Firma Biométrica en Tablet (&ge; 10.5'')</label>
            <svg viewBox="0 0 400 160" class="w-full h-40 rounded border-2 border-dashed border-slate-300 bg-white touch-none" role="img" aria-label="Lienzo de firma biométrica">
              ${strokesSvg}
            </svg>
            <button type="button" class="min-h-[44px] px-3 py-1.5 rounded border border-slate-300 text-xs font-medium text-slate-700">
              Limpiar Trazos
            </button>
          </div>`
        : `<div class="space-y-2">
            <label for="consent-otp-input" class="block text-xs font-medium text-slate-700">Código OTP de 6 Dígitos</label>
            <input id="consent-otp-input" type="text" inputmode="numeric" maxlength="6" value="${escapeHtml(
              props.otpCode
            )}" class="min-h-[44px] w-full rounded border border-slate-300 px-3 py-2 font-mono text-base tracking-widest tabular-nums" placeholder="000000" />
          </div>`
    }
  </div>`;

  const footerHtml = `<button type="submit" ${
    !validation.valid ? "disabled" : ""
  } class="min-h-[44px] px-4 py-2 rounded bg-blue-600 text-white text-xs font-semibold disabled:opacity-50">
    Firmar y Sellar Consentimiento (RF-06)
  </button>`;

  return renderSlideOverSheetHtml({
    id: `informed-consent-${props.consentId}`,
    title: "Consentimiento Informado Digital (RF-06)",
    description: "Captura biométrica en sillón o validación por código OTP de 6 dígitos.",
    isOpen: props.isOpen,
    widthClass: "max-w-xl",
    bodyHtml,
    footerHtml,
  });
}

export default renderInformedConsentSheetHtml;
