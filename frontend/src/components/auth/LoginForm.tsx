/**
 * Clinical login form: credentials + 6-digit TOTP MFA (wi-13 S3, RNF-05).
 *
 * The component is a client component because the MFA challenge lives in the
 * React state of the form; the TOTP code is forwarded once to the server
 * action and never stored.
 */

"use client";

import { useActionState } from "react";
import { loginAction } from "../../app/(auth)/login/actions.ts";
import type { AuthFlowState } from "../../lib/auth-flow.ts";

const INITIAL_STATE: AuthFlowState = { step: "credentials" };

export default function LoginForm() {
  const [state, formAction, isPending] = useActionState(loginAction, INITIAL_STATE);
  const requiresTotp = state.step === "mfa";

  return (
    <form action={formAction} className="space-y-4">
      {state.error ? (
        <div
          role="alert"
          className="p-3 rounded border border-red-300 bg-red-50 text-xs text-red-800 font-medium"
        >
          {state.error.message}
        </div>
      ) : null}

      <input type="hidden" name="challengeToken" value={state.challengeToken ?? ""} />

      <div>
        <label htmlFor="email" className="block text-xs font-medium text-slate-700">
          Correo Profesional
        </label>
        <input
          id="email"
          name="email"
          type="email"
          required
          readOnly={requiresTotp}
          defaultValue={state.email ?? ""}
          className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm"
        />
      </div>

      {requiresTotp ? null : (
        <div>
          <label htmlFor="password" className="block text-xs font-medium text-slate-700">
            Contraseña
          </label>
          <input
            id="password"
            name="password"
            type="password"
            required
            className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm"
          />
        </div>
      )}

      {requiresTotp ? (
        <div>
          <label htmlFor="totpCode" className="block text-xs font-medium text-slate-700">
            Código MFA TOTP (6 dígitos)
          </label>
          <input
            id="totpCode"
            name="totpCode"
            type="text"
            inputMode="numeric"
            maxLength={6}
            pattern="\d{6}"
            required
            autoComplete="one-time-code"
            className="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-center font-mono text-base tracking-widest tabular-nums"
          />
        </div>
      ) : null}

      <button
        type="submit"
        disabled={isPending}
        className="w-full min-h-[44px] rounded bg-slate-900 text-white text-sm font-semibold disabled:opacity-60"
      >
        {requiresTotp ? "Verificar e Ingresar" : "Iniciar Sesión"}
      </button>
    </form>
  );
}
