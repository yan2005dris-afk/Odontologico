/**
 * Root Layout for SIGO-Dental Web Portal & Chairside PWA (wi-13 S1, AGENTS.md §3).
 *
 * Next emits the HTML document, so this layout only provides the language, the
 * document metadata and the portal surface (cool off-white clinical canvas).
 */

import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";

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
    <html lang="es">
      <body className="bg-slate-50 text-slate-900 antialiased selection:bg-blue-600 selection:text-white">
        {children}
      </body>
    </html>
  );
}
