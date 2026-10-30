/**
 * Profile menu of the portal (wi-14 S5, RNF-05).
 *
 * Shows who is signed in and offers the session actions. The role is shown
 * because it decides what the operator can reach; the portal does not offer
 * actions the role is not allowed to perform.
 */

"use client";

import { useRouter } from "next/navigation";
import { LogOut, UserRound } from "lucide-react";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { UserRole } from "@/lib/roles";

const ROLE_LABELS: Record<UserRole, string> = {
  admin: "Administrador",
  receptionist: "Recepción",
  dentist: "Odontólogo",
  assistant: "Asistente",
  auditor: "Auditor",
};

export interface ProfileDropdownProps {
  userName: string;
  role: UserRole;
}

export function ProfileDropdown({ userName, role }: ProfileDropdownProps) {
  const router = useRouter();
  const initials = userName
    .split(/\s+/)
    .filter((part) => part !== "")
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("");

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" className="h-9 gap-2 px-2">
          <Avatar className="size-7">
            <AvatarFallback className="bg-slate-900 text-[11px] font-semibold text-white">
              {initials || "SD"}
            </AvatarFallback>
          </Avatar>
          <span className="hidden max-w-40 truncate text-sm text-slate-700 lg:inline">
            {userName}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="font-normal">
          <p className="text-sm font-medium text-slate-900">{userName}</p>
          <p className="text-xs text-slate-500">{ROLE_LABELS[role]}</p>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          <DropdownMenuItem disabled>
            <UserRound className="size-4" aria-hidden="true" />
            <span>Mi perfil</span>
          </DropdownMenuItem>
          <DropdownMenuItem
            onSelect={() => {
              router.push("/login");
            }}
          >
            <LogOut className="size-4" aria-hidden="true" />
            <span>Cerrar sesión</span>
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
