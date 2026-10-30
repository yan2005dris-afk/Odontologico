import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  DEFAULT_IDLE_TIMEOUT_MS,
  createSessionIdleGuard,
  validateTotpCode,
  getAllowedNavigationModules,
  type CreatePatientInput,
} from "./session-idle-guard.ts";

describe("wi-10 S1 — Session Idle Guard (15-min timeout RNF-05) & RBAC Navigation", () => {
  it("exposes default 15-minute (900,000 ms) idle timeout constant per RNF-05", () => {
    assert.equal(DEFAULT_IDLE_TIMEOUT_MS, 900_000);
  });

  it("triggers onIdleTimeout exactly when 900,000 ms elapse without user interaction", () => {
    let now = 1_000_000;
    let loggedOutReason: string | null = null;
    let redirectedTo: string | null = null;

    const guard = createSessionIdleGuard({
      timeoutMs: DEFAULT_IDLE_TIMEOUT_MS,
      nowFn: () => now,
      onIdleTimeout: () => {
        loggedOutReason = "SESSION_IDLE_TIMEOUT";
        redirectedTo = "/login?reason=idle_timeout";
      },
    });

    guard.start();

    // Advance 14m 59.999s -> still active
    now += 899_999;
    assert.equal(guard.checkIdle(), false);
    assert.equal(guard.getRemainingMs(), 1);
    assert.equal(loggedOutReason, null);

    // Advance 1ms to reach 15m (900,000 ms) -> locks session and redirects
    now += 1;
    assert.equal(guard.checkIdle(), true);
    assert.equal(guard.isLocked(), true);
    assert.equal(loggedOutReason, "SESSION_IDLE_TIMEOUT");
    assert.equal(redirectedTo, "/login?reason=idle_timeout");
  });

  it("resets countdown on clinical workstation events (pointerdown, keydown, touchstart)", () => {
    let now = 5_000_000;
    let timeoutFired = false;

    const guard = createSessionIdleGuard({
      timeoutMs: DEFAULT_IDLE_TIMEOUT_MS,
      nowFn: () => now,
      onIdleTimeout: () => {
        timeoutFired = true;
      },
    });

    guard.start();
    now += 600_000; // 10 minutes elapsed
    guard.recordActivity("touchstart"); // dentist touches chairside tablet

    now += 600_000; // 10 minutes after touchstart (20m total)
    assert.equal(guard.checkIdle(), false);
    assert.equal(timeoutFired, false);
    assert.equal(guard.getRemainingMs(), 300_000);
  });

  it("ignores further activity once session is locked (adversarial replay attempt)", () => {
    let now = 10_000;
    let fireCount = 0;

    const guard = createSessionIdleGuard({
      timeoutMs: 900_000,
      nowFn: () => now,
      onIdleTimeout: () => {
        fireCount += 1;
      },
    });

    guard.start();
    now += 900_000;
    assert.equal(guard.checkIdle(), true);
    assert.equal(fireCount, 1);

    // Attempting to record activity after lock must not unlock without re-authentication
    guard.recordActivity("keydown");
    assert.equal(guard.isLocked(), true);
    assert.equal(guard.checkIdle(), true);
    assert.equal(fireCount, 1);
  });

  it("validates 6-digit tabular-nums TOTP MFA code strictly (RNF-05)", () => {
    assert.equal(validateTotpCode("123456").valid, true);
    assert.equal(validateTotpCode("12345").valid, false);
    assert.equal(validateTotpCode("12345a").valid, false);
    assert.equal(validateTotpCode("").valid, false);
  });

  it("filters collapsible sidebar modules strictly by RBAC role (S1)", () => {
    const adminModules = getAllowedNavigationModules("admin").map((m) => m.id);
    assert.deepEqual(adminModules, ["agenda", "patients", "billing-inventory", "reports"]);

    const dentistModules = getAllowedNavigationModules("dentist").map((m) => m.id);
    assert.deepEqual(dentistModules, ["agenda", "patients", "reports"]);

    const receptionistModules = getAllowedNavigationModules("receptionist").map((m) => m.id);
    assert.deepEqual(receptionistModules, ["agenda", "patients", "billing-inventory"]);

    const auditorModules = getAllowedNavigationModules("auditor").map((m) => m.id);
    assert.deepEqual(auditorModules, ["patients", "billing-inventory", "reports"]);
  });
});

describe("wi-13 S6 — patient intake payload contract", () => {
  it("keeps sex and emergency contact optional so existing registrations stay valid", () => {
    const legacyDraft: CreatePatientInput = {
      documentType: "DNI",
      documentNumber: "28999888",
      fullName: "Roberto Carlos Pérez",
      birthDate: "1985-07-20",
      phone: "+5491122334455",
      criticalAlerts: [],
    };

    assert.equal(legacyDraft.sex, undefined);
    assert.equal(legacyDraft.emergencyContact, undefined);
  });

  it("carries the sex catalog and the emergency contact required by the clinical API", () => {
    const draft: CreatePatientInput = {
      documentType: "DNI",
      documentNumber: "28999888",
      fullName: "Roberto Carlos Pérez",
      birthDate: "1985-07-20",
      sex: "M",
      phone: "+5491122334455",
      emergencyContact: {
        fullName: "Ana Pérez",
        phone: "+5491100001111",
        relationship: "Pareja",
      },
      criticalAlerts: [],
    };

    assert.equal(draft.sex, "M");
    assert.equal(draft.emergencyContact?.relationship, "Pareja");
  });
});
