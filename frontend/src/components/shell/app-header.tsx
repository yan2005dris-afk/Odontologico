/**
 * Header of the SIGO-Dental portal (wi-14 S5).
 *
 * The header carries the sidebar trigger, the breadcrumb of the active route,
 * the global command palette (⌘K), the theme switch and the profile menu.
 */

"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChevronRight } from "lucide-react";

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Separator } from "@/components/ui/separator";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { getNavigationGroups } from "@/lib/nav-config";
import type { UserRole } from "@/lib/roles";
import { CommandMenu } from "./command-menu.tsx";
import { ProfileDropdown } from "./profile-dropdown.tsx";
import { ThemeSwitch } from "./theme-switch.tsx";

export interface AppHeaderProps {
  role: UserRole;
  userName: string;
}

export function AppHeader({ role, userName }: AppHeaderProps) {
  const pathname = usePathname() ?? "/";
  const groups = getNavigationGroups(role);
  const active = groups
    .flatMap((group) => group.items)
    .find(
      (item) =>
        item.href === pathname || (item.href !== "/" && pathname.startsWith(`${item.href}/`)),
    );
  const groupLabel = groups.find((group) =>
    group.items.some((item) => item.id === active?.id),
  )?.label;

  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b border-slate-200 bg-white px-4">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-1 h-4 bg-slate-200" />

      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/">SIGO-Dental</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          {groupLabel ? (
            <>
              <BreadcrumbSeparator>
                <ChevronRight className="size-3.5" aria-hidden="true" />
              </BreadcrumbSeparator>
              <BreadcrumbItem>{groupLabel}</BreadcrumbItem>
            </>
          ) : null}
          {active ? (
            <>
              <BreadcrumbSeparator>
                <ChevronRight className="size-3.5" aria-hidden="true" />
              </BreadcrumbSeparator>
              <BreadcrumbItem>
                <BreadcrumbPage>{active.label}</BreadcrumbPage>
              </BreadcrumbItem>
            </>
          ) : null}
        </BreadcrumbList>
      </Breadcrumb>

      <div className="ml-auto flex items-center gap-1">
        <CommandMenu role={role} />
        <ThemeSwitch />
        <ProfileDropdown userName={userName} role={role} />
      </div>
    </header>
  );
}
