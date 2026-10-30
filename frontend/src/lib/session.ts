/**
 * Server-side portal session helpers (wi-13 S2/S4, RNF-05).
 *
 * The session lives in an httpOnly cookie; the browser never sees the access
 * token. Server components read it here and forward the bearer token to the
 * API Gateway over the internal network.
 */

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import type { UserRole } from "./session-idle-guard.ts";
import { SESSION_COOKIE_NAME, parseSessionCookie } from "./session-cookie.ts";

export interface PortalSession {
  accessToken: string;
  role: UserRole;
  userName: string;
}

/** Reads the portal session from the request cookies; null when absent. */
export async function readPortalSession(): Promise<PortalSession | null> {
  const store = await cookies();
  const raw = store.get(SESSION_COOKIE_NAME)?.value;
  if (!raw) {
    return null;
  }

  const parsed = parseSessionCookie(`${SESSION_COOKIE_NAME}=${raw}`);
  if (!parsed) {
    return null;
  }

  return {
    accessToken: parsed.accessToken,
    role: parsed.role,
    userName: parsed.userName ?? "Sesión clínica",
  };
}

/** Reads the portal session or redirects to the login screen. */
export async function requirePortalSession(): Promise<PortalSession> {
  const session = await readPortalSession();
  if (!session) {
    redirect("/login");
  }
  return session;
}
