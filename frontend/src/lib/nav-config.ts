/**
 * Navigation tree of the SIGO-Dental portal (wi-14 S7, SRS §3, RNF-05).
 *
 * This module is the single source of truth for the `shadcn-admin` sidebar:
 * the groups, the items, the icons and the role filter. It replaces the
 * navigation block that used to live inside `session-idle-guard.ts`, which
 * mixed the idle-timeout logic with the menu tree.
 *
 * The role matrix below is **aligned with the gateway's edge policy**
 * (`services/api-gateway/internal/ingress/domain/rbac_policy.go`), not with
 * what the portal would like to show. A menu entry the gateway would reject
 * with `403 INSUFFICIENT_ROLE_PERMISSION` is a broken link, so it is not
 * rendered. Two consequences of that alignment, worth stating explicitly:
 *
 *   - `auditor` does **not** see "Pacientes": the gateway answers `false` for
 *     `/api/v1/patients` for that role (rbac_policy.go, "Unified Patient
 *     Demographic Record"). The previous navigation did show it. Whether the
 *     auditor *should* read clinical records is a policy question that
 *     belongs to wi-15, which owns the gateway's RBAC matrix; until then the
 *     portal mirrors the policy that is actually enforced.
 *   - `admin` reaches every known route through the short-circuit at the top
 *     of `IsAllowed`, so it sees the complete tree.
 *
 * The icons are declared as names, not as components, so this module stays a
 * plain `.ts` file: it can be imported by server components and unit-tested
 * without React. `NavIcon` maps the names to the lucide glyphs.
 */

import type { UserRole } from "./roles.ts";

/** Names of the lucide glyphs used by the sidebar. */
export type NavIconName =
  | "layout-dashboard"
  | "calendar-days"
  | "users"
  | "clipboard-list"
  | "file-signature"
  | "receipt"
  | "wallet"
  | "coins"
  | "package"
  | "user-cog"
  | "scroll-text";

export type NavItemId =
  | "dashboard"
  | "agenda"
  | "patients"
  | "treatments"
  | "consents"
  | "billing"
  | "cash-registers"
  | "settlements"
  | "inventory"
  | "users"
  | "audit";

export interface NavItem {
  id: NavItemId;
  label: string;
  href: string;
  icon: NavIconName;
  allowedRoles: UserRole[];
}

export interface NavGroup {
  id: string;
  label: string;
  items: NavItem[];
}

/** Groups of the `shadcn-admin` sidebar, in display order. */
export const NAVIGATION_GROUPS: NavGroup[] = [
  {
    id: "clinical",
    label: "Clínica & Turnos",
    items: [
      {
        id: "dashboard",
        label: "Dashboard",
        href: "/",
        icon: "layout-dashboard",
        allowedRoles: ["admin", "receptionist", "dentist", "assistant", "auditor"],
      },
      {
        id: "agenda",
        label: "Agenda & Turnos",
        href: "/agenda",
        icon: "calendar-days",
        allowedRoles: ["admin", "receptionist", "dentist", "assistant"],
      },
      {
        id: "patients",
        label: "Pacientes",
        href: "/patients",
        icon: "users",
        allowedRoles: ["admin", "receptionist", "dentist", "assistant"],
      },
      {
        id: "treatments",
        label: "Planes y Presupuestos",
        href: "/treatments",
        icon: "clipboard-list",
        allowedRoles: ["admin", "receptionist", "dentist", "assistant"],
      },
      {
        id: "consents",
        label: "Consentimientos",
        href: "/consents",
        icon: "file-signature",
        allowedRoles: ["admin", "receptionist", "dentist", "assistant"],
      },
    ],
  },
  {
    id: "operations",
    label: "Caja & Operaciones",
    items: [
      {
        id: "billing",
        label: "Facturación POS",
        href: "/billing",
        icon: "receipt",
        allowedRoles: ["admin", "receptionist", "auditor"],
      },
      {
        id: "cash-registers",
        label: "Cierres de Caja",
        href: "/cash-registers",
        icon: "wallet",
        allowedRoles: ["admin", "receptionist", "auditor"],
      },
      {
        id: "settlements",
        label: "Liquidaciones",
        href: "/settlements",
        icon: "coins",
        allowedRoles: ["admin", "dentist", "auditor"],
      },
      {
        id: "inventory",
        label: "Inventario & Recetas",
        href: "/inventory",
        icon: "package",
        allowedRoles: ["admin", "assistant", "dentist", "auditor"],
      },
    ],
  },
  {
    id: "administration",
    label: "Administración & Seguridad",
    items: [
      {
        id: "users",
        label: "Usuarios y Roles",
        href: "/users",
        icon: "user-cog",
        allowedRoles: ["admin"],
      },
      {
        id: "audit",
        label: "Auditoría Clínica",
        href: "/audit",
        icon: "scroll-text",
        allowedRoles: ["admin", "auditor"],
      },
    ],
  },
];

/**
 * Routes that actually exist in the portal today.
 *
 * A menu entry whose route is not implemented yet would render a link to a
 * 404, which is worse than an absent entry. wi-16 adds `/treatments` and
 * `/consents`; wi-17 adds `/cash-registers`, `/users` and `/audit`. Each of
 * them adds one line here.
 */
export const IMPLEMENTED_ROUTES: readonly string[] = [
  "/",
  "/agenda",
  "/patients",
  "/billing",
  "/settlements",
  "/inventory",
];

export interface NavigationOptions {
  /** When true (default) only routes present in `IMPLEMENTED_ROUTES` are returned. */
  onlyImplementedRoutes?: boolean;
}

/** Returns the navigation groups visible to a role, empty groups removed. */
export function getNavigationGroups(
  role: UserRole,
  options: NavigationOptions = {},
): NavGroup[] {
  const onlyImplemented = options.onlyImplementedRoutes ?? true;
  return NAVIGATION_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter(
      (item) =>
        item.allowedRoles.includes(role) &&
        (!onlyImplemented || IMPLEMENTED_ROUTES.includes(item.href)),
    ),
  })).filter((group) => group.items.length > 0);
}

/** Flat list of the navigation entries visible to a role. */
export function getAllowedNavigationModules(
  role: UserRole,
  options: NavigationOptions = {},
): NavItem[] {
  return getNavigationGroups(role, options).flatMap((group) => group.items);
}
