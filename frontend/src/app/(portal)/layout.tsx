/**
 * Authenticated portal layout (wi-13 S2, wi-14 S5/S8, RNF-05).
 *
 * Resolves the session from the httpOnly cookie, mounts the `shadcn-admin`
 * shell with the role of the session and redirects to the login screen when
 * there is no session. The role filters the navigation groups of the sidebar
 * (RBAC) on the client.
 *
 * The clinical screens are rendered unchanged inside the shell: wi-14 replaces
 * the frame, not the content.
 */

import type { ReactNode } from "react";
import { PortalShell } from "../../components/shell/portal-shell.tsx";
import { requirePortalSession } from "../../lib/session.ts";

export default async function PortalLayout({ children }: { children: ReactNode }) {
  const session = await requirePortalSession();

  return (
    <PortalShell role={session.role} userName={session.userName}>
      {children}
    </PortalShell>
  );
}
