import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  calculatePatientAge,
  validatePatientRegistration,
  renderPatientRegistrationFormHtml,
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
