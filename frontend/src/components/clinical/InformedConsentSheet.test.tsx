import { describe, it } from "vitest";
import assert from "node:assert/strict";
import {
  validateInformedConsentSubmission,
  renderInformedConsentSheetHtml,
} from "./InformedConsentSheet.tsx";

describe("wi-11 S4 — Digital Informed Consent Sheet (Biometric Touch Pad & 6-Digit OTP RF-06)", () => {
  it("blocks submission in BIOMETRIC_PAD mode when SVG stroke points are empty or insufficient (S4)", () => {
    const emptyStrokes = validateInformedConsentSubmission({
      consentId: "cons-01",
      mode: "BIOMETRIC_PAD",
      svgStrokes: [],
      otpCode: "",
    });
    assert.equal(emptyStrokes.valid, false);
    assert.equal(emptyStrokes.errorCode, "BIOMETRIC_SIGNATURE_REQUIRED");

    const validStrokes = validateInformedConsentSubmission({
      consentId: "cons-01",
      mode: "BIOMETRIC_PAD",
      svgStrokes: ["M 10 10 L 45 60 L 90 35"],
      otpCode: "",
    });
    assert.equal(validStrokes.valid, true);
  });

  it("blocks submission in OTP_CODE mode unless OTP has exactly 6 numeric digits (S4)", () => {
    const shortOtp = validateInformedConsentSubmission({
      consentId: "cons-01",
      mode: "OTP_CODE",
      svgStrokes: [],
      otpCode: "1234",
    });
    assert.equal(shortOtp.valid, false);
    assert.equal(shortOtp.errorCode, "INVALID_OTP_CODE");

    const validOtp = validateInformedConsentSubmission({
      consentId: "cons-01",
      mode: "OTP_CODE",
      svgStrokes: [],
      otpCode: "654321",
    });
    assert.equal(validOtp.valid, true);
  });

  it("renders SlideOverSheet dialog with touch target >= 44x44px and mode switcher (RNF-07, S4)", () => {
    const html = renderInformedConsentSheetHtml({
      isOpen: true,
      consentId: "cons-01",
      patientName: "María Fernanda Gómez",
      procedureTitle: "Exodoncia Quirúrgica Pieza 38",
      mode: "BIOMETRIC_PAD",
      svgStrokes: [],
      otpCode: "",
    });

    assert.match(html, /role="dialog"/);
    assert.match(html, /min-h-\[44px\]/);
    assert.match(html, /disabled/);
  });
});
