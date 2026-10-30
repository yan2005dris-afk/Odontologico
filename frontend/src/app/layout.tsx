/**
 * Root Layout for SIGO-Dental Web Portal & Chairside PWA (wi-13 S1, wi-14 S5, AGENTS.md §3).
 *
 * Next emits the HTML document, so this layout only provides the language, the
 * document metadata and the portal surface (cool off-white clinical canvas).
 *
 * The `ThemeProvider` of `next-themes` lives here rather than in the portal
 * shell so that the theme also covers the login screen; it writes the
 * `light`/`dark` class on the `<html>` element, which is what the
 * `@custom-variant dark` of `globals.css` reacts to.
 */

import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import { ThemeProvider } from "next-themes";

import "./globals.css";

export const metadata: Metadata = {
  title: "SIGO-Dental — Sistema Integral de Gestión Odontológica",
  description:
    "Portal clínico y administrativo de SIGO-Dental: ficha única de paciente, odontograma, periodontograma, evolución SOEP, agenda multirrecinto, caja e inventario.",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="es" suppressHydrationWarning>
      <body className="bg-slate-50 text-slate-900 antialiased selection:bg-blue-600 selection:text-white">
        <ThemeProvider attribute="class" defaultTheme="light" enableSystem>
          {children}
        </ThemeProvider>
      </body>
    </html>
  );
}
