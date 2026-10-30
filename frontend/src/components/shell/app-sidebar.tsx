/**
 * Sidebar of the SIGO-Dental portal (wi-14 S5, SRS §3, RNF-05, RNF-07).
 *
 * The `shadcn-admin` pattern: a collapsible sidebar that shrinks to icons,
 * with the navigation grouped by operational area. The groups and their
 * entries come from `src/lib/nav-config.ts`, which filters them by the role
 * of the session and by the routes that actually exist.
 *
 * The idle-session indicator lives in the footer: the operator has to be able
 * to see, at a glance, how much of the 15-minute `RNF-05` window is left.
 */

"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { getNavigationGroups } from "@/lib/nav-config";
import type { UserRole } from "@/lib/roles";
import { NavIcon } from "./nav-icon.tsx";

export interface AppSidebarProps {
  role: UserRole;
  /** Remaining idle time in milliseconds; the footer shows it as a countdown. */
  idleRemainingMs?: number;
}

const IDLE_TIMEOUT_MS = 900_000;

export function AppSidebar({ role, idleRemainingMs }: AppSidebarProps) {
  const pathname = usePathname() ?? "";
  const groups = getNavigationGroups(role);
  const remainingSeconds = Math.max(
    0,
    Math.ceil((idleRemainingMs ?? IDLE_TIMEOUT_MS) / 1000),
  );
  const remainingMinutes = Math.floor(remainingSeconds / 60);

  return (
    // The shadcn `Sidebar` renders a plain `<div>`: the navigation landmark
    // that the previous shell exposed as `<aside role="navigation">` is added
    // here, so the operator can jump to it with the screen reader.
    // `display: contents` keeps the wrapper out of the layout.
    <nav aria-label="Navegación Principal SIGO-Dental" className="contents">
      <Sidebar collapsible="icon" className="border-r border-slate-200">
        <SidebarHeader>
          <div className="flex items-center gap-2 px-2 py-1">
            <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-slate-900 text-xs font-bold text-white">
              SD
            </span>
            <span className="truncate text-sm font-bold tracking-tight text-slate-900 group-data-[collapsible=icon]:hidden">
              SIGO-Dental
            </span>
          </div>
        </SidebarHeader>

        <SidebarContent>
          {groups.map((group) => (
            <SidebarGroup key={group.id}>
              <SidebarGroupLabel className="text-slate-500">
                {group.label}
              </SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {group.items.map((item) => {
                    const isActive =
                      item.href === "/"
                        ? pathname === "/"
                        : pathname === item.href ||
                          pathname.startsWith(`${item.href}/`);
                    return (
                      <SidebarMenuItem key={item.id}>
                        <SidebarMenuButton
                          asChild
                          isActive={isActive}
                          tooltip={item.label}
                          className="min-h-[44px]"
                        >
                          <Link
                            href={item.href}
                            aria-current={isActive ? "page" : undefined}
                          >
                            <NavIcon
                              name={item.icon}
                              className="size-4 shrink-0"
                            />
                            <span>{item.label}</span>
                          </Link>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    );
                  })}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ))}
        </SidebarContent>

        <SidebarFooter>
          <div className="px-2 py-1 text-xs text-slate-600 group-data-[collapsible=icon]:hidden">
            <p className="font-medium text-slate-800">Sesión activa</p>
            <p className="tabular-nums" data-testid="idle-guard-indicator">
              Timeout 15m · {remainingMinutes}m {remainingSeconds % 60}s
              restantes
            </p>
          </div>
        </SidebarFooter>
      </Sidebar>
    </nav>
  );
}
