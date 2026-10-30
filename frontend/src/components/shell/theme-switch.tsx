/**
 * Theme switch of the portal (wi-14 S5).
 *
 * The portal ships with the light theme: the cool off-white canvas and the
 * pure white clinical surface are part of the AGENTS.md §3 visual contract.
 * The dark palette exists, but its contrast audit is part of `wi-17`.
 */

"use client";

import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import { Moon, Sun } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export function ThemeSwitch() {
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  // The theme is only known on the client: rendering the icon before mount
  // would produce a hydration mismatch.
  useEffect(() => setMounted(true), []);

  if (!mounted) {
    return (
      <Button variant="ghost" size="icon" aria-label="Cambiar el tema" disabled>
        <Sun className="size-4" aria-hidden="true" />
      </Button>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Cambiar el tema">
          {theme === "dark" ? (
            <Moon className="size-4" aria-hidden="true" />
          ) : (
            <Sun className="size-4" aria-hidden="true" />
          )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={() => setTheme("light")}>Tema claro</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => setTheme("dark")}>Tema oscuro</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => setTheme("system")}>Tema del sistema</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
