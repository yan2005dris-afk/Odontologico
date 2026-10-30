/**
 * Development accounts of the seeded RBAC roles (wi-18 S3).
 *
 * These are the 5 canonical accounts that `clinical-agenda` seeds on every
 * boot (`cmd/api/main.go`, `canonicalSeedAccounts`). They exist so that the
 * role gated screens can be exercised without an out-of-band provisioning
 * channel.
 *
 * The list is only rendered when the portal is built with
 * `NEXT_PUBLIC_DEV_ROLE_LOGIN=true`, which the development stack in
 * `deploy/compose.yaml` does and nothing else does. When the flag is off this
 * module is never imported by a rendered component, so the passwords do not
 * reach the browser bundle.
 *
 * These credentials are for development only: the seed runs on every restart
 * and the repositories are in memory.
 */

import type { UserRole } from "./roles.ts";

export interface DevAccount {
  /** Canonical RBAC role of SRS §3. */
  role: UserRole;
  email: string;
  password: string;
  /** Human name shown in the picker. */
  displayName: string;
  /** What this role is allowed to see in the portal. */
  scope: string;
}

/** The 5 seeded accounts, in the order the role picker shows them. */
export const DEV_ACCOUNTS: readonly DevAccount[] = [
  {
    role: "admin",
    email: "admin@sigo-dental.com",
    password: "AdminPass#2026",
    displayName: "Andrés Vega Salinas",
    scope: "Configuración, personal de la clínica y acceso completo",
  },
  {
    role: "dentist",
    email: "dra.gomez@sigo-dental.com",
    password: "ClinicalPass#2026",
    displayName: "Dra. Valeria Gómez",
    scope: "Atención clínica: odontograma, presupuestos, consentimientos",
  },
  {
    role: "receptionist",
    email: "recepcion@sigo-dental.com",
    password: "ReceptionPass#2026",
    displayName: "Lucía Fernández Ruiz",
    scope: "Recepción: agenda, registro de pacientes, cobro y caja",
  },
  {
    role: "assistant",
    email: "asistente@sigo-dental.com",
    password: "AssistantPass#2026",
    displayName: "Marcelo Ríos Cabrera",
    scope: "Soporte en consultorio: signos, insumos y notas",
  },
  {
    role: "auditor",
    email: "auditor@sigo-dental.com",
    password: "AuditorPass#2026",
    displayName: "Paula Herrera Ortiz",
    scope: "Auditoría financiera: caja, liquidaciones e inventario",
  },
] as const;

/**
 * Reports whether the role picker may be rendered.
 *
 * The flag is a `NEXT_PUBLIC_` variable, so it is inlined at build time: a
 * build without it can never show the picker, whatever the runtime env says.
 */
export function isDevRoleLoginEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DEV_ROLE_LOGIN === "true";
}
