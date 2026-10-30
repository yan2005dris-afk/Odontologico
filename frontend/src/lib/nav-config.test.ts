/**
 * Navigation tree contract (wi-14 S7, SRS §3, RNF-05).
 *
 * The sidebar is an affordance: it may only offer what the gateway actually
 * authorises. These tests pin the role matrix against the policy enforced in
 * `services/api-gateway/internal/ingress/domain/rbac_policy.go`, and they pin
 * the rule that a route which does not exist yet is never rendered.
 */

import { describe, expect, it } from "vitest";

import {
  IMPLEMENTED_ROUTES,
  NAVIGATION_GROUPS,
  getAllowedNavigationModules,
  getNavigationGroups,
} from "./nav-config.ts";
import { USER_ROLES } from "./roles.ts";

describe("wi-14 S7 — navigation groups", () => {
  it("organises the portal into the three shadcn-admin groups", () => {
    expect(NAVIGATION_GROUPS.map((group) => group.label)).toEqual([
      "Clínica & Turnos",
      "Caja & Operaciones",
      "Administración & Seguridad",
    ]);
  });

  it("declares every entry once and with a unique href", () => {
    const items = NAVIGATION_GROUPS.flatMap((group) => group.items);
    const ids = items.map((item) => item.id);
    const hrefs = items.map((item) => item.href);

    expect(new Set(ids).size).toBe(ids.length);
    expect(new Set(hrefs).size).toBe(hrefs.length);
  });

  it("gives every entry a label, an icon and at least one role", () => {
    for (const item of NAVIGATION_GROUPS.flatMap((group) => group.items)) {
      expect(item.label.length).toBeGreaterThan(0);
      expect(item.icon.length).toBeGreaterThan(0);
      expect(item.allowedRoles.length).toBeGreaterThan(0);
      for (const role of item.allowedRoles) {
        expect(USER_ROLES).toContain(role);
      }
    }
  });
});

describe("wi-14 S7 — role filter mirrors the gateway edge policy", () => {
  it("gives admin every implemented route", () => {
    expect(getAllowedNavigationModules("admin").map((item) => item.id)).toEqual([
      "dashboard",
      "agenda",
      "patients",
      "billing",
      "settlements",
      "inventory",
    ]);
  });

  it("keeps the clinical tree for dentist and assistant", () => {
    expect(getAllowedNavigationModules("dentist").map((item) => item.id)).toEqual([
      "dashboard",
      "agenda",
      "patients",
      "settlements",
      "inventory",
    ]);
    expect(getAllowedNavigationModules("assistant").map((item) => item.id)).toEqual([
      "dashboard",
      "agenda",
      "patients",
      "inventory",
    ]);
  });

  it("gives receptionist the agenda, the patients and the POS", () => {
    expect(getAllowedNavigationModules("receptionist").map((item) => item.id)).toEqual([
      "dashboard",
      "agenda",
      "patients",
      "billing",
    ]);
  });

  it("does not offer the auditor the patient record, which the gateway rejects with 403", () => {
    // rbac_policy.go returns false for `auditor` on /api/v1/patients, so the
    // previous navigation offered a link that could only fail.
    expect(getAllowedNavigationModules("auditor").map((item) => item.id)).toEqual([
      "dashboard",
      "billing",
      "settlements",
      "inventory",
    ]);
    expect(
      getAllowedNavigationModules("auditor").some((item) => item.href === "/patients"),
    ).toBe(false);
  });

  it("reserves the users and audit entries for admin and auditor respectively", () => {
    const all = getAllowedNavigationModules("admin", { onlyImplementedRoutes: false });
    expect(all.find((item) => item.id === "users")?.allowedRoles).toEqual(["admin"]);
    expect(all.find((item) => item.id === "audit")?.allowedRoles).toEqual([
      "admin",
      "auditor",
    ]);
  });
});

describe("wi-14 S7 — no dead links", () => {
  it("hides the entries whose route is not implemented yet", () => {
    const rendered = getAllowedNavigationModules("admin");
    for (const item of rendered) {
      expect(IMPLEMENTED_ROUTES).toContain(item.href);
    }
    expect(rendered.some((item) => item.id === "treatments")).toBe(false);
    expect(rendered.some((item) => item.id === "consents")).toBe(false);
    expect(rendered.some((item) => item.id === "users")).toBe(false);
    expect(rendered.some((item) => item.id === "audit")).toBe(false);
    expect(rendered.some((item) => item.id === "cash-registers")).toBe(false);
  });

  it("exposes the full target tree when the filter is disabled", () => {
    const ids = getAllowedNavigationModules("admin", { onlyImplementedRoutes: false }).map(
      (item) => item.id,
    );
    expect(ids).toEqual([
      "dashboard",
      "agenda",
      "patients",
      "treatments",
      "consents",
      "billing",
      "cash-registers",
      "settlements",
      "inventory",
      "users",
      "audit",
    ]);
  });

  it("drops the groups that end up empty for a role", () => {
    const groups = getNavigationGroups("auditor");
    expect(groups.map((group) => group.id)).toEqual(["clinical", "operations"]);
    for (const group of groups) {
      expect(group.items.length).toBeGreaterThan(0);
    }
  });
});
