/**
 * Canonical RBAC roles of SIGO-Dental (SRS §3, RNF-05).
 *
 * This module is the single source of truth for the role vocabulary of the
 * portal. Before wi-14 the same list was duplicated in three places
 * (`session-idle-guard.ts`, `auth-flow.ts` and `gateway-server.ts`), which
 * meant a role added to one file could silently disappear from another.
 *
 * The five roles mirror the gateway's edge policy
 * (`services/api-gateway/internal/ingress/domain/rbac_policy.go`):
 *
 *   - `admin`        — administrative and system route access.
 *   - `receptionist` — agenda, patients, billing, POS and blind cash close.
 *   - `dentist`      — clinical workspace, patients, treatments, scheduling.
 *   - `assistant`    — clinical workspace and scheduling, read-only elsewhere.
 *   - `auditor`      — read-only access to billing, settlements, inventory and
 *                      the immutable audit trail.
 */

export const USER_ROLES = [
  "admin",
  "receptionist",
  "dentist",
  "assistant",
  "auditor",
] as const;

export type UserRole = (typeof USER_ROLES)[number];

/** Narrows an unknown value to one of the five canonical roles. */
export function isUserRole(value: unknown): value is UserRole {
  return typeof value === "string" && (USER_ROLES as readonly string[]).includes(value);
}
