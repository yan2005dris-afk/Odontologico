/**
 * Global command palette of the portal (wi-14 S5).
 *
 * The palette indexes **modules and routes**, never patients. Indexing
 * clinical records would put protected health information inside a global
 * search box that every role can open; the patient directory is reached
 * through its own screen, and the search endpoint that would make it safe is
 * still to be built (`wi-15`).
 */

"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { CornerDownLeft, Moon, Search, Sun } from "lucide-react";

import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from "@/components/ui/command";
import { getNavigationGroups } from "@/lib/nav-config";
import type { UserRole } from "@/lib/roles";
import { NavIcon } from "./nav-icon.tsx";

export interface CommandMenuProps {
  role: UserRole;
}

export function CommandMenu({ role }: CommandMenuProps) {
  const router = useRouter();
  const { setTheme } = useTheme();
  const [open, setOpen] = useState(false);
  const groups = useMemo(() => getNavigationGroups(role), [role]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setOpen((value) => !value);
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="inline-flex h-9 items-center gap-2 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-500 transition-colors hover:bg-slate-50"
        aria-label="Abrir la búsqueda global"
      >
        <Search className="size-4" aria-hidden="true" />
        <span className="hidden md:inline">Buscar módulo…</span>
        <kbd className="hidden rounded border border-slate-200 bg-slate-50 px-1.5 font-mono text-[10px] text-slate-500 md:inline">
          ⌘K
        </kbd>
      </button>

      <CommandDialog
        open={open}
        onOpenChange={setOpen}
        title="Búsqueda global"
        description="Salta a un módulo del portal. Los pacientes se buscan en su propia pantalla."
      >
        <CommandInput placeholder="Buscar módulo…" />
        <CommandList>
          <CommandEmpty>No hay coincidencias.</CommandEmpty>
          {groups.map((group) => (
            <CommandGroup key={group.id} heading={group.label}>
              {group.items.map((item) => (
                <CommandItem
                  key={item.id}
                  value={`${group.label} ${item.label}`}
                  onSelect={() => {
                    setOpen(false);
                    router.push(item.href);
                  }}
                >
                  <NavIcon name={item.icon} className="size-4 text-slate-500" />
                  <span>{item.label}</span>
                  <CommandShortcut>
                    <CornerDownLeft className="size-3" aria-hidden="true" />
                  </CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          ))}
          <CommandSeparator />
          <CommandGroup heading="Apariencia">
            <CommandItem
              value="tema claro"
              onSelect={() => {
                setTheme("light");
                setOpen(false);
              }}
            >
              <Sun className="size-4 text-slate-500" />
              <span>Tema claro</span>
            </CommandItem>
            <CommandItem
              value="tema oscuro"
              onSelect={() => {
                setTheme("dark");
                setOpen(false);
              }}
            >
              <Moon className="size-4 text-slate-500" />
              <span>Tema oscuro</span>
            </CommandItem>
          </CommandGroup>
        </CommandList>
      </CommandDialog>
    </>
  );
}
