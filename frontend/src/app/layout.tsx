/**
 * Root Layout for SIGO-Dental Web Portal & Chairside PWA (wi-10, AGENTS.md §3).
 */

export interface RootLayoutProps {
  children?: string;
}

export function renderRootLayoutHtml(props: RootLayoutProps): string {
  return `<!DOCTYPE html>
<html lang="es">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>SIGO-Dental — Sistema Integral de Gestión Odontológica</title>
  </head>
  <body class="bg-slate-50 text-slate-900 antialiased selection:bg-blue-600 selection:text-white">
    ${props.children ?? ""}
  </body>
</html>`;
}

export default renderRootLayoutHtml;
