/**
 * AppShell Component (wi-10 S1, RNF-05, RNF-07, AGENTS.md §3).
 * Collapsible left sidebar filtered by RBAC role, slate-50 surface, and 15-min idle session banner.
 */

import { DEFAULT_IDLE_TIMEOUT_MS, escapeHtml } from "../../lib/session-idle-guard.ts";
import {
  getAllowedNavigationModules,
  type NavItem,
} from "../../lib/nav-config.ts";
import type { UserRole } from "../../lib/roles.ts";

export interface AppShellProps {
  userRole: UserRole;
  userName: string;
  activePath: string;
  sidebarCollapsed?: boolean;
  idleRemainingMs?: number;
  childrenHtml?: string;
}

export interface AppShellViewModel {
  role: UserRole;
  allowedModules: NavItem[];
  sidebarCollapsed: boolean;
  surfaceClass: string;
  sidebarWidthClass: string;
  idleTimeoutSeconds: number;
}

export function buildAppShellModel(props: AppShellProps): AppShellViewModel {
  const allowedModules = getAllowedNavigationModules(props.userRole);
  const collapsed = Boolean(props.sidebarCollapsed);
  const remainingMs = props.idleRemainingMs ?? DEFAULT_IDLE_TIMEOUT_MS;

  return {
    role: props.userRole,
    allowedModules,
    sidebarCollapsed: collapsed,
    surfaceClass: "min-h-screen bg-slate-50 text-slate-900 flex",
    sidebarWidthClass: collapsed ? "w-16" : "w-64",
    idleTimeoutSeconds: Math.ceil(remainingMs / 1000),
  };
}

export function renderAppShellHtml(props: AppShellProps): string {
  const model = buildAppShellModel(props);
  const navItemsHtml = model.allowedModules
    .map((item) => {
      const isActive = props.activePath.startsWith(item.href);
      const activeClass = isActive
        ? "bg-slate-900 text-white font-semibold"
        : "text-slate-700 hover:bg-slate-100";
      return `<li key="${item.id}">
        <a href="${item.href}" class="flex items-center min-h-[44px] px-3 py-2 rounded-md text-sm ${activeClass}" aria-current="${isActive ? "page" : "false"}">
          <span>${escapeHtml(item.label)}</span>
        </a>
      </li>`;
    })
    .join("\n");

  return `<div class="${model.surfaceClass}" data-role="${escapeHtml(model.role)}">
    <aside role="navigation" aria-label="Navegación Principal SIGO-Dental" class="${model.sidebarWidthClass} bg-white border-r border-slate-200 flex flex-col justify-between shrink-0">
      <div class="p-4">
        <div class="flex items-center justify-between mb-6">
          <span class="font-bold tracking-tight text-slate-900">SIGO-Dental</span>
          <button type="button" aria-label="Colapsar barra lateral" aria-expanded="${!model.sidebarCollapsed}" class="min-h-[44px] min-w-[44px] rounded border border-slate-200 text-xs text-slate-600">
            ${model.sidebarCollapsed ? "»" : "«"}
          </button>
        </div>
        <ul class="space-y-1">
          ${navItemsHtml}
        </ul>
      </div>
      <div class="p-4 border-t border-slate-200 text-xs text-slate-600">
        <p class="font-medium text-slate-800">${escapeHtml(props.userName)}</p>
        <p class="tabular-nums" data-testid="idle-guard-indicator">Sesión activa · Timeout 15m (${model.idleTimeoutSeconds}s)</p>
      </div>
    </aside>
    <main role="main" class="flex-1 min-w-0 overflow-y-auto">
      ${props.childrenHtml ?? ""}
    </main>
  </div>`;
}

export default renderAppShellHtml;
