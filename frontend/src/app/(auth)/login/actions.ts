/**
 * Server actions of the clinical login (wi-13 S3, RNF-05).
 *
 * The action never exposes the access token to the browser: it stores it in an
 * httpOnly cookie and redirects. The MFA challenge token travels through the
 * React state of the form, never through a URL query string.
 */

"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import {
  createAuthFlow,
  type AuthenticatedSession,
  type AuthFlowState,
} from "../../../lib/auth-flow.ts";
import { createGatewayServerClient } from "../../../lib/gateway-server.ts";
import {
  SESSION_COOKIE_NAME,
  buildSessionCookieAttributes,
  shouldUseSecureCookie,
} from "../../../lib/session-cookie.ts";

const INITIAL_STATE: AuthFlowState = { step: "credentials" };

async function persistSession(session: AuthenticatedSession): Promise<void> {
  const store = await cookies();
  const attributes = buildSessionCookieAttributes(
    session.idleTimeoutMs,
    shouldUseSecureCookie()
  );

  store.set(
    SESSION_COOKIE_NAME,
    encodeURIComponent(
      JSON.stringify({
        accessToken: session.accessToken,
        role: session.role,
        userName: session.userName,
      })
    ),
    {
      httpOnly: attributes.httpOnly,
      sameSite: attributes.sameSite,
      path: attributes.path,
      maxAge: attributes.maxAge,
      secure: attributes.secure,
    }
  );
}

/**
 * Handles both steps of the login: credentials first, then the 6-digit TOTP.
 * The returned state drives the form; a successful MFA redirects to the
 * patient list with the session cookie already written.
 */
export async function loginAction(
  previousState: AuthFlowState = INITIAL_STATE,
  formData: FormData
): Promise<AuthFlowState> {
  const email = String(formData.get("email") ?? "").trim();
  const password = String(formData.get("password") ?? "");
  const totpCode = String(formData.get("totpCode") ?? "").trim();
  const challengeToken = String(formData.get("challengeToken") ?? "").trim();

  const flow = createAuthFlow(createGatewayServerClient());

  const state =
    challengeToken !== ""
      ? await flow.submitTotp(
          { step: "mfa", challengeToken, email: previousState.email ?? email },
          { totpCode }
        )
      : await flow.submitCredentials({ step: "credentials", email }, { email, password });

  if (state.step === "authenticated" && state.session) {
    await persistSession(state.session);
    redirect("/patients");
  }

  return state;
}
