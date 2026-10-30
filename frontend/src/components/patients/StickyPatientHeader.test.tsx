import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  buildStickyPatientHeaderModel,
  renderStickyPatientHeaderHtml,
} from "./StickyPatientHeader.tsx";

describe("wi-10 S3 — StickyPatientHeader with High-Visibility Critical Alerts (RF-02)", () => {
  it("renders sticky top-0 banner with role='alert' high-contrast badges when criticalAlerts is non-empty", () => {
    const patient = {
      id: "pat-001",
      hcNumber: "HC-2026-0001",
      fullName: "María Fernanda Gómez",
      age: 64,
      isMinor: false,
      insurancePlan: {
        providerName: "OSDE",
        planName: "Plan 310",
      },
      criticalAlerts: [
        { code: "PENICILLIN_ALLERGY", label: "Alergia Severa a Penicilina", severity: "CRITICAL" as const },
        { code: "ANTICOAGULANTS", label: "Paciente Anticoagulado (Warfarina)", severity: "HIGH" as const },
      ],
    };

    const model = buildStickyPatientHeaderModel(patient);
    assert.equal(model.isSticky, true);
    assert.equal(model.hasCriticalAlerts, true);
    assert.equal(model.alertBadges.length, 2);
    assert.equal(model.alertBadges[0].ariaRole, "alert");

    const html = renderStickyPatientHeaderHtml(patient);
    assert.match(html, /sticky top-0/);
    assert.match(html, /role="alert"/);
    assert.match(html, /Alergia Severa a Penicilina/);
    assert.match(html, /Paciente Anticoagulado \(Warfarina\)/);
    assert.match(html, /tabular-nums/);
  });

  it("escapes untrusted characters in alert labels to prevent XSS injection (Section 6 Security)", () => {
    const patient = {
      id: "pat-002",
      hcNumber: "HC-2026-0002",
      fullName: "Juan <script>alert(1)</script>",
      age: 12,
      isMinor: true,
      criticalAlerts: [
        { code: "ALLERGY_LATEX", label: "Alergia Látex <img src=x onerror=alert(1)>", severity: "CRITICAL" as const },
      ],
    };

    const html = renderStickyPatientHeaderHtml(patient);
    assert.equal(html.includes("<script>"), false);
    assert.equal(html.includes("<img"), false);
    assert.match(html, /&lt;script&gt;/);
  });
});
