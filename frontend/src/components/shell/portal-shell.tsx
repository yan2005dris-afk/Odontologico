/**
 * Portal shell of the SIGO-Dental portal (wi-14 S5/S8).
 *
 * This is the `shadcn-admin` frame: a collapsible sidebar, a header with the
 * breadcrumb and the global palette, and the content surface. The clinical
 * screens keep rendering exactly the same content; only the frame around them
 * changes in this work item.
 *
 * The shell is a client island because the sidebar state, the breadcrumb and
 * the command palette are interactive. The data of the screens is still
 * loaded by the server components that render inside it (wi-13 S4).
 */

"use client";

import type { ReactNode } from "react";

import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { UserRole } from "@/lib/roles";
import { AppHeader } from "./app-header.tsx";
import { AppSidebar } from "./app-sidebar.tsx";

export interface PortalShellProps {
  role: UserRole;
  userName: string;
  children?: ReactNode;
}

export function PortalShell({ role, userName, children }: PortalShellProps) {
  return (
    // The shadcn sidebar renders every collapsed entry as a Tooltip, so the
    // provider travels with the shell: the sidebar is self-sufficient and
    // does not depend on the root layout to be wrapped.
    <TooltipProvider delayDuration={200}>
      <SidebarProvider>
        <AppSidebar role={role} />
        <SidebarInset className="min-w-0 bg-slate-50">
          <AppHeader role={role} userName={userName} />
          <main role="main" className="min-w-0 flex-1 p-4 lg:p-6">
            {children}
          </main>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  );
}
