/**
 * PatientRegistrationForm Component (wi-10 S2, RF-01).
 * Unified patient intake form enforcing mandatory Legal Guardian fields when calculated age < 18.
 */

import {
  escapeHtml,
  type CreatePatientInput,
} from "../../lib/session-idle-guard.ts";

export interface PatientRegistrationValidationResult {
  valid: boolean;
  age: number;
  isMinor: boolean;
  errorCode?: "INVALID_PATIENT_FIELDS" | "LEGAL_GUARDIAN_REQUIRED_FOR_MINOR";
  fieldErrors: Record<string, string>;
}

/**
 * Calculates exact age in years given YYYY-MM-DD birthDate and YYYY-MM-DD referenceDate.
 */
export function calculatePatientAge(birthDateIso: string, referenceDateIso = "2026-10-30"): number {
  const [bYear, bMonth, bDay] = birthDateIso.split("-").map(Number);
  const [rYear, rMonth, rDay] = referenceDateIso.split("-").map(Number);

  if (!bYear || !bMonth || !bDay || !rYear || !rMonth || !rDay) {
    return 0;
  }

  let age = rYear - bYear;
  if (rMonth < bMonth || (rMonth === bMonth && rDay < bDay)) {
    age -= 1;
  }
  return age;
}

export function validatePatientRegistration(
  input: CreatePatientInput,
  referenceDateIso = "2026-10-30"
): PatientRegistrationValidationResult {
  const fieldErrors: Record<string, string> = {};

  if (!input.fullName.trim()) {
    fieldErrors.fullName = "Nombre completo obligatorio";
  }
  if (!input.documentNumber.trim()) {
    fieldErrors.documentNumber = "Número de documento obligatorio";
  }
  if (!input.birthDate.trim()) {
    fieldErrors.birthDate = "Fecha de nacimiento obligatoria";
  }
  if (!input.phone.trim()) {
    fieldErrors.phone = "Teléfono de contacto obligatorio";
  }

  const age = calculatePatientAge(input.birthDate, referenceDateIso);
  const isMinor = age < 18;

  if (isMinor) {
    const guardian = input.legalGuardian;
    if (!guardian?.fullName?.trim()) {
      fieldErrors.guardianFullName = "Nombre del tutor legal obligatorio para menores de 18 años";
    }
    if (!guardian?.documentNumber?.trim()) {
      fieldErrors.guardianDocumentNumber = "Documento del tutor legal obligatorio";
    }
    if (!guardian?.phone?.trim()) {
      fieldErrors.guardianPhone = "Teléfono del tutor legal obligatorio";
    }
    if (!guardian?.relationship?.trim()) {
      fieldErrors.guardianRelationship = "Parentesco del tutor legal obligatorio";
    }

    if (
      fieldErrors.guardianFullName ||
      fieldErrors.guardianDocumentNumber ||
      fieldErrors.guardianPhone ||
      fieldErrors.guardianRelationship
    ) {
      return {
        valid: false,
        age,
        isMinor: true,
        errorCode: "LEGAL_GUARDIAN_REQUIRED_FOR_MINOR",
        fieldErrors,
      };
    }
  }

  if (Object.keys(fieldErrors).length > 0) {
    return {
      valid: false,
      age,
      isMinor,
      errorCode: "INVALID_PATIENT_FIELDS",
      fieldErrors,
    };
  }

  return {
    valid: true,
    age,
    isMinor,
    fieldErrors: {},
  };
}

export function renderPatientRegistrationFormHtml(
  input: CreatePatientInput,
  referenceDateIso = "2026-10-30"
): string {
  const validation = validatePatientRegistration(input, referenceDateIso);

  const guardianSectionHtml = validation.isMinor
    ? `<fieldset class="p-4 rounded-lg border border-amber-300 bg-amber-50/50 space-y-3" data-testid="legal-guardian-section">
        <legend class="text-sm font-semibold text-amber-950 px-1">Tutor Legal Obligatorio (Paciente menor de 18 años — RF-01)</legend>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div>
            <label class="block text-xs font-medium text-slate-700">Nombre del Tutor Legal *</label>
            <input type="text" required value="${escapeHtml(input.legalGuardian?.fullName ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-700">Documento del Tutor *</label>
            <input type="text" required value="${escapeHtml(input.legalGuardian?.documentNumber ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-700">Teléfono del Tutor *</label>
            <input type="tel" required value="${escapeHtml(input.legalGuardian?.phone ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-700">Parentesco *</label>
            <input type="text" required value="${escapeHtml(input.legalGuardian?.relationship ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
          </div>
        </div>
      </fieldset>`
    : "";

  const errorBannerHtml = !validation.valid
    ? `<div role="alert" class="p-3 rounded border border-red-300 bg-red-50 text-xs text-red-800 font-medium">
        Error de validación (${validation.errorCode}): complete todos los campos obligatorios.
      </div>`
    : "";

  return `<form class="bg-white p-6 rounded-lg border border-slate-200 space-y-4" aria-label="Formulario de Ficha Única de Paciente">
    <div class="flex items-center justify-between border-b border-slate-200 pb-3">
      <h2 class="text-base font-bold text-slate-900">Ficha Única de Paciente (RF-01)</h2>
      <span class="text-xs tabular-nums text-slate-600">Edad calculada: ${validation.age} años</span>
    </div>
    ${errorBannerHtml}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div>
        <label class="block text-xs font-medium text-slate-700">Nombre Completo *</label>
        <input type="text" required value="${escapeHtml(input.fullName)}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700">Documento (${escapeHtml(input.documentType)}) *</label>
        <input type="text" required value="${escapeHtml(input.documentNumber)}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700">Fecha de Nacimiento *</label>
        <input type="date" required value="${escapeHtml(input.birthDate)}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700">Teléfono *</label>
        <input type="tel" required value="${escapeHtml(input.phone)}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
      </div>
    </div>
    ${guardianSectionHtml}
    <div class="flex justify-end pt-3">
      <button type="submit" ${!validation.valid ? "disabled" : ""} class="min-h-[44px] px-4 py-2 rounded bg-slate-900 text-white text-sm font-semibold disabled:opacity-50">
        Guardar Ficha de Paciente
      </button>
    </div>
  </form>`;
}

export default renderPatientRegistrationFormHtml;
