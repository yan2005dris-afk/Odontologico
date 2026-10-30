/**
 * Authenticated portal layout (wi-13 S2, RNF-05).
 *
 * Resolves the session from the httpOnly cookie, mounts the AppShell with the
 * role of the session and redirects to the login screen when there is no
 * session. The role filters the sidebar modules (RBAC) on the client.
 */

import type { ReactNode } from "react";
import AppShellView from "../../components/shell/AppShellView.tsx";
import { requirePortalSession } from "../../lib/session.ts";

export default async function PortalLayout({ children }: { children: ReactNode }) {
  const session = await requirePortalSession();

  return (
    <AppShellView userRole={session.role} userName={session.userName}>
      {children}
    </AppShellView>
  );
}
