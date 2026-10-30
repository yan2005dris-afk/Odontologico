import { describe, it } from "vitest";
import assert from "node:assert/strict";
import {
  calculatePatientAge,
  validatePatientRegistration,
  renderPatientRegistrationFormHtml,
  hasCompleteBirthDate,
  isPristinePatientDraft,
} from "./PatientRegistrationForm.tsx";

describe("wi-10 S2 — Unified Patient Registration & Minor Legal Guardian Validation (RF-01)", () => {
  const referenceDate = "2026-10-30";

  it("calculates age accurately and marks patients under 18 as minors", () => {
    assert.equal(calculatePatientAge("2014-05-10", referenceDate), 12);
    assert.equal(calculatePatientAge("2008-10-30", referenceDate), 18);
    assert.equal(calculatePatientAge("2008-10-31", referenceDate), 17);
  });

  it("blocks submission with LEGAL_GUARDIAN_REQUIRED_FOR_MINOR when age < 18 and guardian fields are missing", () => {
    const minorWithoutGuardian = {
      documentType: "DNI" as const,
      documentNumber: "52100999",
      fullName: "Lucas Mateo Díaz",
      birthDate: "2015-03-15",
      phone: "+5491144556677",
      email: "familia@example.com",
      criticalAlerts: [],
    };

    const result = validatePatientRegistration(minorWithoutGuardian, referenceDate);
    assert.equal(result.valid, false);
    assert.equal(result.isMinor, true);
    assert.equal(result.errorCode, "LEGAL_GUARDIAN_REQUIRED_FOR_MINOR");
    assert.equal(result.fieldErrors.guardianFullName !== undefined, true);
    assert.equal(result.fieldErrors.guardianDocumentNumber !== undefined, true);
    assert.equal(result.fieldErrors.guardianPhone !== undefined, true);
    assert.equal(result.fieldErrors.guardianRelationship !== undefined, true);
  });

  it("accepts minor registration only when all 4 legal guardian fields are populated", () => {
    const minorWithGuardian = {
      documentType: "DNI" as const,
      documentNumber: "52100999",
      fullName: "Lucas Mateo Díaz",
      birthDate: "2015-03-15",
      phone: "+5491144556677",
      legalGuardian: {
        fullName: "Carolina Díaz",
        documentNumber: "30111222",
        phone: "+5491144556677",
        relationship: "Madre",
      },
      criticalAlerts: [],
    };

    const result = validatePatientRegistration(minorWithGuardian, referenceDate);
    assert.equal(result.valid, true);
    assert.equal(result.isMinor, true);
    assert.equal(result.errorCode, undefined);

    const html = renderPatientRegistrationFormHtml(minorWithGuardian, referenceDate);
    assert.match(html, /Tutor Legal Obligatorio/);
  });

  it("allows adult registration (>= 18 years) without legal guardian fields", () => {
    const adultInput = {
      documentType: "DNI" as const,
      documentNumber: "28999888",
      fullName: "Roberto Carlos Pérez",
      birthDate: "1985-07-20",
      phone: "+5491122334455",
      criticalAlerts: [],
    };

    const result = validatePatientRegistration(adultInput, referenceDate);
    assert.equal(result.valid, true);
    assert.equal(result.isMinor, false);
  });
});

describe("wi-13 S6 — submittable patient registration form", () => {
  const referenceDate = "2026-10-30";

  const blankDraft = {
    documentType: "DNI" as const,
    documentNumber: "",
    fullName: "",
    birthDate: "",
    phone: "",
    email: "",
    sex: "F" as const,
    emergencyContact: { fullName: "", phone: "", relationship: "" },
    legalGuardian: { fullName: "", documentNumber: "", phone: "", relationship: "" },
    criticalAlerts: [],
  };

  it("injects the form action and method into the emitted <form> tag", () => {
    const html = renderPatientRegistrationFormHtml(blankDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    assert.match(html, /<form[^>]*action="\/patients\/create"/);
    assert.match(html, /<form[^>]*method="post"/);
  });

  it("keeps the legacy static rendering free of action and method attributes", () => {
    const html = renderPatientRegistrationFormHtml(blankDraft, referenceDate);

    assert.equal(html.includes("action="), false);
    assert.equal(html.includes("method="), false);
  });

  it("names every input so the browser can submit the form", () => {
    const html = renderPatientRegistrationFormHtml(blankDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    for (const fieldName of [
      "fullName",
      "documentType",
      "documentNumber",
      "birthDate",
      "sex",
      "phone",
      "email",
      "emergencyContactFullName",
      "emergencyContactPhone",
      "emergencyContactRelationship",
      "guardianFullName",
      "guardianDocumentNumber",
      "guardianPhone",
      "guardianRelationship",
    ]) {
      assert.equal(html.includes(`name="${fieldName}"`), true, `missing name attribute: ${fieldName}`);
    }
  });

  it("exposes the mandatory emergency contact block required by the API", () => {
    const html = renderPatientRegistrationFormHtml(blankDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    assert.match(html, /Contacto de Emergencia/);
    assert.match(html, /name="emergencyContactFullName"/);
    assert.match(html, /name="emergencyContactPhone"/);
    assert.match(html, /name="emergencyContactRelationship"/);
  });

  it("offers the sex catalog accepted by the clinical service (F|M|X)", () => {
    const html = renderPatientRegistrationFormHtml(blankDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    assert.match(html, /<select[^>]*name="sex"/);
    assert.match(html, /<option value="F"/);
    assert.match(html, /<option value="M"/);
    assert.match(html, /<option value="X"/);
  });

  it("always exposes the legal guardian block on the submittable form", () => {
    const html = renderPatientRegistrationFormHtml(blankDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    assert.match(html, /Tutor Legal Obligatorio/);
    assert.match(html, /name="guardianFullName"/);
    assert.match(html, /name="guardianDocumentNumber"/);
    assert.match(html, /name="guardianPhone"/);
    assert.match(html, /name="guardianRelationship"/);
  });

  it("keeps the submit button enabled on the submittable form", () => {
    const html = renderPatientRegistrationFormHtml(blankDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    assert.equal(/<button[^>]*\sdisabled(?:\s|>)/.test(html), false);
  });
});

describe("defect 14 — no false LEGAL_GUARDIAN_REQUIRED_FOR_MINOR on an untouched intake form", () => {
  const referenceDate = "2026-10-30";

  /** Same draft the patients page renders (app/(portal)/patients/page.tsx EMPTY_DRAFT). */
  const untouchedDraft = {
    documentType: "DNI" as const,
    documentNumber: "",
    fullName: "",
    birthDate: "",
    phone: "",
    email: "",
    sex: "F" as const,
    emergencyContact: { fullName: "", phone: "", relationship: "" },
    legalGuardian: { fullName: "", documentNumber: "", phone: "", relationship: "" },
    criticalAlerts: [],
  };

  it("renders no validation banner for an untouched draft", () => {
    const html = renderPatientRegistrationFormHtml(untouchedDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    assert.equal(html.includes("Error de validación"), false);
    assert.equal(html.includes("LEGAL_GUARDIAN_REQUIRED_FOR_MINOR"), false);
    assert.equal(html.includes('role="alert"'), false);
  });

  it("never reports an age of 0 years while the birth date is unknown", () => {
    const html = renderPatientRegistrationFormHtml(untouchedDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    assert.equal(html.includes("Edad calculada: 0 años"), false);
    assert.match(html, /Edad calculada: —/);
  });

  it("does not assert that the patient is a minor while the age is unknown", () => {
    const html = renderPatientRegistrationFormHtml(untouchedDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });

    assert.equal(html.includes("Paciente menor de 18 años"), false);
    assert.match(html, /Tutor Legal Obligatorio \(obligatorio si el paciente es menor de 18 años/);
  });

  it("treats a missing birth date as an unknown age, not as a 0-year-old minor", () => {
    const result = validatePatientRegistration(untouchedDraft, referenceDate);

    assert.equal(result.valid, false);
    assert.equal(result.ageKnown, false);
    assert.equal(result.isMinor, false);
    assert.equal(result.errorCode, "INVALID_PATIENT_FIELDS");
  });

  it("detects a pristine draft only while no identifying data was entered", () => {
    assert.equal(isPristinePatientDraft(untouchedDraft), true);
    assert.equal(
      isPristinePatientDraft({ ...untouchedDraft, fullName: "Ana Lucía Fernández" }),
      false
    );
    assert.equal(
      isPristinePatientDraft({ ...untouchedDraft, birthDate: "1990-04-12" }),
      false
    );
    assert.equal(
      isPristinePatientDraft({
        ...untouchedDraft,
        emergencyContact: { fullName: "Luis Fernández", phone: "+593999", relationship: "Padre" },
      }),
      false
    );
  });

  it("still reports the legal guardian error for a real minor without a guardian", () => {
    const minorDraft = {
      ...untouchedDraft,
      documentNumber: "52100999",
      fullName: "Lucas Mateo Díaz",
      birthDate: "2015-03-15",
      phone: "+593991234567",
      email: "familia@example.com",
    };

    const result = validatePatientRegistration(minorDraft, referenceDate);
    assert.equal(result.ageKnown, true);
    assert.equal(result.isMinor, true);
    assert.equal(result.errorCode, "LEGAL_GUARDIAN_REQUIRED_FOR_MINOR");

    const html = renderPatientRegistrationFormHtml(minorDraft, referenceDate, {
      formAction: "/patients/create",
      formMethod: "post",
    });
    assert.match(html, /Error de validación \(LEGAL_GUARDIAN_REQUIRED_FOR_MINOR\)/);
    assert.match(html, /Paciente menor de 18 años/);
  });

  it("only accepts complete YYYY-MM-DD birth dates as a computable age", () => {
    assert.equal(hasCompleteBirthDate("2015-03-15"), true);
    assert.equal(hasCompleteBirthDate(""), false);
    assert.equal(hasCompleteBirthDate("2015-03"), false);
    assert.equal(hasCompleteBirthDate("15/03/2015"), false);
    assert.equal(hasCompleteBirthDate("2015-13-01"), false);
  });
});
