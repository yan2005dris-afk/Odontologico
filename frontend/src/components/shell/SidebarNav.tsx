/**
 * Client-side navigation of the SIGO-Dental portal sidebar (wi-13 S2, RNF-07).
 *
 * The module list is filtered by RBAC with the same pure helper used by the
 * string renderer (`getAllowedNavigationModules`), so both views expose exactly
 * the same modules for a role.
 */

"use client";

import { usePathname } from "next/navigation";
import { getAllowedNavigationModules, type UserRole } from "../../lib/session-idle-guard.ts";

export interface SidebarNavProps {
  role: UserRole;
  collapsed?: boolean;
}

export default function SidebarNav({ role, collapsed = false }: SidebarNavProps) {
  const pathname = usePathname() ?? "";
  const modules = getAllowedNavigationModules(role);

  return (
    <ul className="space-y-1">
      {modules.map((item) => {
        const isActive = pathname === item.href || pathname.startsWith(`${item.href}/`);
        const activeClass = isActive
          ? "bg-slate-900 text-white font-semibold"
          : "text-slate-700 hover:bg-slate-100";

        return (
          <li key={item.id}>
            <a
              href={item.href}
              className={`flex items-center min-h-[44px] px-3 py-2 rounded-md text-sm ${activeClass}`}
              aria-current={isActive ? "page" : undefined}
            >
              <span>{item.label}</span>
            </a>
          </li>
        );
      })}
    </ul>
  );
}
