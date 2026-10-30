/**
 * React AppShell for the SIGO-Dental portal (wi-13 S1/S2, RNF-05, RNF-07).
 *
 * The string renderer of the shell lives in `AppShell.tsx` and stays untouched;
 * this module is the React view used by the App Router portal layout. It reuses
 * the same pure model (`buildAppShellModel`) so the RBAC navigation filter, the
 * surface classes and the idle-session indicator cannot drift between both.
 */

import type { ReactNode } from "react";
import { buildAppShellModel, type AppShellProps } from "./AppShell.tsx";
import SidebarNav from "./SidebarNav.tsx";

export interface AppShellViewProps extends Omit<AppShellProps, "activePath"> {
  children?: ReactNode;
}

export default function AppShellView(props: AppShellViewProps) {
  const model = buildAppShellModel({
    ...props,
    // The React view resolves the active module from the current pathname, so
    // the string renderer's `activePath` is not part of its contract.
    activePath: "",
  });

  return (
    <div className={model.surfaceClass} data-role={model.role}>
      <aside
        role="navigation"
        aria-label="Navegación Principal SIGO-Dental"
        className={`${model.sidebarWidthClass} bg-white border-r border-slate-200 flex flex-col justify-between shrink-0`}
      >
        <div className="p-4">
          <div className="flex items-center justify-between mb-6">
            <span className="font-bold tracking-tight text-slate-900">SIGO-Dental</span>
            <button
              type="button"
              aria-label="Colapsar barra lateral"
              aria-expanded={!model.sidebarCollapsed}
              className="min-h-[44px] min-w-[44px] rounded border border-slate-200 text-xs text-slate-600"
            >
              {model.sidebarCollapsed ? "»" : "«"}
            </button>
          </div>
          <SidebarNav role={model.role} collapsed={model.sidebarCollapsed} />
        </div>
        <div className="p-4 border-t border-slate-200 text-xs text-slate-600">
          <p className="font-medium text-slate-800">{props.userName}</p>
          <p className="tabular-nums" data-testid="idle-guard-indicator">
            Sesión activa · Timeout 15m ({model.idleTimeoutSeconds}s)
          </p>
        </div>
      </aside>
      <main role="main" className="flex-1 min-w-0 overflow-y-auto">
        {props.children}
      </main>
    </div>
  );
}
