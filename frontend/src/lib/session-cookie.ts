/**
 * httpOnly Session Cookie for the SIGO-Dental portal (wi-13 S4, RNF-05).
 *
 * The browser never sees the access token: server components and server actions
 * read this cookie and call the API Gateway over the internal network.
 */

import { isUserRole, type UserRole } from "./roles.ts";

export const SESSION_COOKIE_NAME = "sigo_session";

export interface SessionCookiePayload {
  accessToken: string;
  role: UserRole;
  /** Display name shown in the AppShell sidebar; omitted when unknown. */
  userName?: string;
}

export interface SessionCookieOptions {
  /** Idle timeout reported by the gateway; the cookie expires with the session. */
  maxAgeMs: number;
  /** Only true when the portal is served over HTTPS (production). */
  secure: boolean;
}

/** Cookie attributes shared by the Set-Cookie string and the Next cookie store. */
export interface SessionCookieAttributes {
  httpOnly: true;
  sameSite: "lax";
  path: "/";
  /** Max-Age in seconds. */
  maxAge: number;
  secure: boolean;
}

/** Builds the cookie attributes; the single source of truth for the session cookie. */
export function buildSessionCookieAttributes(
  maxAgeMs: number,
  secure: boolean
): SessionCookieAttributes {
  return {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    maxAge: Math.max(0, Math.floor(maxAgeMs / 1000)),
    secure,
  };
}

/**
 * Builds the Set-Cookie value for a fresh session. `Max-Age` is expressed in
 * seconds, so the millisecond idle timeout is floored.
 */
export function serializeSessionCookie(
  payload: SessionCookiePayload,
  options: SessionCookieOptions
): string {
  const cookieAttributes = buildSessionCookieAttributes(options.maxAgeMs, options.secure);
  const attributes = [
    `${SESSION_COOKIE_NAME}=${encodeURIComponent(JSON.stringify(payload))}`,
    `Path=${cookieAttributes.path}`,
    `Max-Age=${cookieAttributes.maxAge}`,
    "HttpOnly",
    "SameSite=Lax",
  ];

  if (cookieAttributes.secure) {
    attributes.push("Secure");
  }

  return attributes.join("; ");
}

/** Builds the Set-Cookie value that expires the session immediately. */
export function serializeSessionClearCookie(options: { secure: boolean }): string {
  const attributes = [
    `${SESSION_COOKIE_NAME}=`,
    "Path=/",
    "Max-Age=0",
    "HttpOnly",
    "SameSite=Lax",
  ];

  if (options.secure) {
    attributes.push("Secure");
  }

  return attributes.join("; ");
}

/**
 * Reads the session payload from a request `Cookie` header. Returns null when
 * the cookie is absent, undecodable, or does not carry a known role, so callers
 * treat a tampered cookie exactly like a missing session.
 */
export function parseSessionCookie(cookieHeader: string | undefined): SessionCookiePayload | null {
  if (!cookieHeader) {
    return null;
  }

  for (const part of cookieHeader.split(";")) {
    const separator = part.indexOf("=");
    if (separator === -1) {
      continue;
    }
    if (part.slice(0, separator).trim() !== SESSION_COOKIE_NAME) {
      continue;
    }

    const rawValue = part.slice(separator + 1).trim();
    try {
      const parsed: unknown = JSON.parse(decodeURIComponent(rawValue));
      if (typeof parsed !== "object" || parsed === null) {
        return null;
      }

      const candidate = parsed as Record<string, unknown>;
      if (typeof candidate.accessToken !== "string" || candidate.accessToken === "") {
        return null;
      }
      if (!isUserRole(candidate.role)) {
        return null;
      }

      const session: SessionCookiePayload = {
        accessToken: candidate.accessToken,
        role: candidate.role,
      };
      if (typeof candidate.userName === "string" && candidate.userName !== "") {
        session.userName = candidate.userName;
      }

      return session;
    } catch {
      return null;
    }
  }

  return null;
}

/** True when the portal is served over TLS, so the cookie must carry Secure. */
export function shouldUseSecureCookie(env: Record<string, string | undefined> = process.env): boolean {
  if (env.SESSION_COOKIE_SECURE === "true") {
    return true;
  }
  if (env.SESSION_COOKIE_SECURE === "false") {
    return false;
  }
  return env.NODE_ENV === "production";
}
