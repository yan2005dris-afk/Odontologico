/**
 * Authenticated Portal Layout wrapping AppShell and 15-minute IdleSessionGuard (wi-10 S1).
 */

import { renderAppShellHtml, type AppShellProps } from "../../components/shell/AppShell.tsx";

export function renderPortalLayoutHtml(props: AppShellProps): string {
  return renderAppShellHtml(props);
}

export default renderPortalLayoutHtml;
