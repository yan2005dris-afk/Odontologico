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
  /** Age in years; meaningful only when `ageKnown` is true. */
  age: number;
  /** True only when the age could be computed AND the patient is under 18. */
  isMinor: boolean;
  /** False when the birth date is missing or malformed, i.e. the age is unknown. */
  ageKnown: boolean;
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

/**
 * True when the birth date is a well-formed YYYY-MM-DD value, i.e. when the
 * age can actually be computed.
 *
 * A missing or malformed birth date means "unknown age", never "0 years".
 * Treating an empty date as age 0 made every untouched intake draft look like
 * a minor and raised a false LEGAL_GUARDIAN_REQUIRED_FOR_MINOR on page load
 * (defect 14).
 */
export function hasCompleteBirthDate(birthDateIso: string): boolean {
  const trimmed = birthDateIso.trim();
  if (!/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) {
    return false;
  }
  const [year, month, day] = trimmed.split("-").map(Number);
  return year > 0 && month >= 1 && month <= 12 && day >= 1 && day <= 31;
}

/**
 * True while the intake form is still untouched: the user has not entered any
 * identifying data yet.
 *
 * A pristine draft must not report validation errors — the missing fields are
 * expected at that point and the age is still unknown. The real validation
 * happens on submit, in the clinical service, and its error reaches the
 * portal through the `?error=` query parameter.
 */
export function isPristinePatientDraft(input: CreatePatientInput): boolean {
  const values = [
    input.fullName,
    input.documentNumber,
    input.birthDate,
    input.phone,
    input.email ?? "",
    input.emergencyContact?.fullName ?? "",
    input.emergencyContact?.phone ?? "",
    input.emergencyContact?.relationship ?? "",
    input.legalGuardian?.fullName ?? "",
    input.legalGuardian?.documentNumber ?? "",
    input.legalGuardian?.phone ?? "",
    input.legalGuardian?.relationship ?? "",
  ];
  return values.every((value) => value.trim() === "");
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

  const ageKnown = hasCompleteBirthDate(input.birthDate);
  const age = ageKnown ? calculatePatientAge(input.birthDate, referenceDateIso) : 0;
  const isMinor = ageKnown && age < 18;

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
        ageKnown,
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
      ageKnown,
      errorCode: "INVALID_PATIENT_FIELDS",
      fieldErrors,
    };
  }

  return {
    valid: true,
    age,
    isMinor,
    ageKnown,
    fieldErrors: {},
  };
}

/**
 * Options of the registration form renderer.
 *
 * `formAction` / `formMethod` turn the static preview into a real HTML form
 * that the browser can submit to a Next.js route handler (wi-13 S6).
 */
export interface PatientRegistrationFormOptions {
  /** Target of the emitted `<form>` tag, e.g. `/patients/create`. */
  formAction?: string;
  /** HTTP method of the emitted `<form>` tag, e.g. `post`. */
  formMethod?: string;
}

const DOCUMENT_TYPE_OPTIONS: ReadonlyArray<{
  value: NonNullable<CreatePatientInput["documentType"]>;
  label: string;
}> = [
  { value: "DNI", label: "DNI" },
  { value: "PASSPORT", label: "Pasaporte" },
  { value: "FOREIGN_ID", label: "Cédula extranjera" },
];

const SEX_OPTIONS: ReadonlyArray<{
  value: NonNullable<CreatePatientInput["sex"]>;
  label: string;
}> = [
  { value: "F", label: "Femenino" },
  { value: "M", label: "Masculino" },
  { value: "X", label: "No binario (X)" },
];

export function renderPatientRegistrationFormHtml(
  input: CreatePatientInput,
  referenceDateIso = "2026-10-30",
  options?: PatientRegistrationFormOptions
): string {
  const validation = validatePatientRegistration(input, referenceDateIso);

  // A submittable form is validated by the clinical service, so the guardian
  // block is always visible and the submit button is never disabled.
  const isSubmittable = typeof options?.formAction === "string" && options.formAction !== "";
  // An untouched draft has no expected errors and an unknown age: reporting
  // them before the user types anything is a false positive (defect 14).
  const isPristine = isPristinePatientDraft(input);
  const showGuardianSection = validation.isMinor || isSubmittable;
  const guardianRequired = validation.isMinor && !isSubmittable;

  const documentTypeOptionsHtml = DOCUMENT_TYPE_OPTIONS.map((option) => {
    const selected = input.documentType === option.value ? " selected" : "";
    return `<option value="${option.value}"${selected}>${escapeHtml(option.label)}</option>`;
  }).join("");

  const sexOptionsHtml = SEX_OPTIONS.map((option) => {
    const selected = input.sex === option.value ? " selected" : "";
    return `<option value="${option.value}"${selected}>${escapeHtml(option.label)}</option>`;
  }).join("");

  const guardianRequiredAttr = guardianRequired ? "required" : "";
  const guardianLegendHtml = validation.isMinor
    ? "Tutor Legal Obligatorio (Paciente menor de 18 años — RF-01)"
    : "Tutor Legal Obligatorio (obligatorio si el paciente es menor de 18 años — RF-01)";
  const guardianSectionHtml = showGuardianSection
    ? `<fieldset class="p-4 rounded-lg border border-amber-300 bg-amber-50/50 space-y-3" data-testid="legal-guardian-section">
        <legend class="text-sm font-semibold text-amber-950 px-1">${guardianLegendHtml}</legend>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div>
            <label class="block text-xs font-medium text-slate-700" for="guardianFullName">Nombre del Tutor Legal *</label>
            <input id="guardianFullName" name="guardianFullName" type="text" ${guardianRequiredAttr} value="${escapeHtml(input.legalGuardian?.fullName ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-700" for="guardianDocumentNumber">Documento del Tutor *</label>
            <input id="guardianDocumentNumber" name="guardianDocumentNumber" type="text" ${guardianRequiredAttr} value="${escapeHtml(input.legalGuardian?.documentNumber ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-700" for="guardianPhone">Teléfono del Tutor *</label>
            <input id="guardianPhone" name="guardianPhone" type="tel" ${guardianRequiredAttr} value="${escapeHtml(input.legalGuardian?.phone ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-700" for="guardianRelationship">Parentesco *</label>
            <input id="guardianRelationship" name="guardianRelationship" type="text" ${guardianRequiredAttr} value="${escapeHtml(input.legalGuardian?.relationship ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
          </div>
        </div>
      </fieldset>`
    : "";

  const emergencyContactSectionHtml = `<fieldset class="p-4 rounded-lg border border-slate-200 bg-slate-50/60 space-y-3" data-testid="emergency-contact-section">
        <legend class="text-sm font-semibold text-slate-900 px-1">Contacto de Emergencia *</legend>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
          <div>
            <label class="block text-xs font-medium text-slate-700" for="emergencyContactFullName">Nombre Completo *</label>
            <input id="emergencyContactFullName" name="emergencyContactFullName" type="text" required value="${escapeHtml(input.emergencyContact?.fullName ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-700" for="emergencyContactPhone">Teléfono *</label>
            <input id="emergencyContactPhone" name="emergencyContactPhone" type="tel" required value="${escapeHtml(input.emergencyContact?.phone ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
          </div>
          <div>
            <label class="block text-xs font-medium text-slate-700" for="emergencyContactRelationship">Parentesco *</label>
            <input id="emergencyContactRelationship" name="emergencyContactRelationship" type="text" required value="${escapeHtml(input.emergencyContact?.relationship ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
          </div>
        </div>
      </fieldset>`;

  const errorBannerHtml = !validation.valid && !isPristine
    ? `<div role="alert" class="p-3 rounded border border-red-300 bg-red-50 text-xs text-red-800 font-medium">
        Error de validación (${validation.errorCode}): complete todos los campos obligatorios.
      </div>`
    : "";

  const formAttributesHtml = [
    ...(options?.formAction ? [`action="${escapeHtml(options.formAction)}"`] : []),
    ...(options?.formMethod ? [`method="${escapeHtml(options.formMethod)}"`] : []),
    'class="bg-white p-6 rounded-lg border border-slate-200 space-y-4"',
    'aria-label="Formulario de Ficha Única de Paciente"',
  ].join(" ");

  const submitDisabledAttr = !isSubmittable && !validation.valid ? "disabled" : "";
  const ageSummaryHtml = validation.ageKnown
    ? `${validation.age} años`
    : "—";

  return `<form ${formAttributesHtml}>
    <div class="flex items-center justify-between border-b border-slate-200 pb-3">
      <h2 class="text-base font-bold text-slate-900">Ficha Única de Paciente (RF-01)</h2>
      <span class="text-xs tabular-nums text-slate-600">Edad calculada: ${ageSummaryHtml}</span>
    </div>
    ${errorBannerHtml}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div>
        <label class="block text-xs font-medium text-slate-700" for="fullName">Nombre Completo *</label>
        <input id="fullName" name="fullName" type="text" required value="${escapeHtml(input.fullName)}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700" for="documentType">Tipo de Documento *</label>
        <select id="documentType" name="documentType" required class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm bg-white">
          ${documentTypeOptionsHtml}
        </select>
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700" for="documentNumber">Número de Documento *</label>
        <input id="documentNumber" name="documentNumber" type="text" required value="${escapeHtml(input.documentNumber)}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700" for="birthDate">Fecha de Nacimiento *</label>
        <input id="birthDate" name="birthDate" type="date" required value="${escapeHtml(input.birthDate)}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700" for="sex">Sexo *</label>
        <select id="sex" name="sex" required class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm bg-white">
          ${sexOptionsHtml}
        </select>
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700" for="phone">Teléfono *</label>
        <input id="phone" name="phone" type="tel" required value="${escapeHtml(input.phone)}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm tabular-nums" />
      </div>
      <div>
        <label class="block text-xs font-medium text-slate-700" for="email">Correo Electrónico</label>
        <input id="email" name="email" type="email" value="${escapeHtml(input.email ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
      </div>
    </div>
    ${emergencyContactSectionHtml}
    ${guardianSectionHtml}
    <div class="flex justify-end pt-3">
      <button type="submit" ${submitDisabledAttr} class="min-h-[44px] px-4 py-2 rounded bg-slate-900 text-white text-sm font-semibold disabled:opacity-50">
        Guardar Ficha de Paciente
      </button>
    </div>
  </form>`;
}

export default renderPatientRegistrationFormHtml;
