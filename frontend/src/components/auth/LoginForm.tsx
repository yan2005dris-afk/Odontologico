/**
 * Clinical login form: credentials + optional 6-digit TOTP MFA (wi-13 S3,
 * RNF-05; wi-18 S3 adds the development role picker).
 *
 * The component is a client component because the MFA challenge lives in the
 * React state of the form; the TOTP code is forwarded once to the server
 * action and never stored.
 *
 * The role picker is a development convenience: it fills the credentials of
 * one of the seeded RBAC accounts and submits the form. It is only rendered
 * when the portal was built with `NEXT_PUBLIC_DEV_ROLE_LOGIN=true`.
 */

"use client";

import { useActionState, useRef } from "react";
import { loginAction } from "../../app/(auth)/login/actions.ts";
import type { AuthFlowState } from "../../lib/auth-flow.ts";
import { DEV_ACCOUNTS, isDevRoleLoginEnabled } from "../../lib/dev-accounts.ts";

const INITIAL_STATE: AuthFlowState = { step: "credentials" };

export default function LoginForm() {
  const [state, formAction, isPending] = useActionState(loginAction, INITIAL_STATE);
  const formRef = useRef<HTMLFormElement>(null);
  const requiresTotp = state.step === "mfa";
  const showRolePicker = isDevRoleLoginEnabled();

  /**
   * Fills the credentials of a seeded account and submits the form, so that
   * switching between roles does not need a TOTP code (wi-18 S3).
   */
  function loginAs(email: string, password: string) {
    const form = formRef.current;
    if (!form) {
      return;
    }
    const emailField = form.elements.namedItem("email");
    const passwordField = form.elements.namedItem("password");
    if (emailField instanceof HTMLInputElement) {
      emailField.value = email;
    }
    if (passwordField instanceof HTMLInputElement) {
      passwordField.value = password;
    }
    form.requestSubmit();
  }

  return (
    <form ref={formRef} action={formAction} className="space-y-4">
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

      {showRolePicker ? (
        <section
          aria-label="Acceso rápido por rol (desarrollo)"
          data-testid="dev-role-picker"
          className="rounded-lg border border-amber-300 bg-amber-50/60 p-3 space-y-2"
        >
          <p className="text-xs font-semibold text-amber-950">
            Modo desarrollo — ingresar como
          </p>
          <div className="grid gap-1.5">
            {DEV_ACCOUNTS.map((account) => (
              <button
                key={account.role}
                type="button"
                onClick={() => loginAs(account.email, account.password)}
                disabled={isPending}
                className="rounded border border-amber-300 bg-white px-3 py-2 text-left text-xs hover:bg-amber-100 disabled:opacity-60"
              >
                <span className="font-semibold text-slate-900">{account.displayName}</span>
                <span className="ml-1 rounded bg-slate-900 px-1.5 py-0.5 font-mono text-[10px] uppercase text-white">
                  {account.role}
                </span>
                <span className="mt-0.5 block text-[11px] text-slate-600">{account.scope}</span>
              </button>
            ))}
          </div>
        </section>
      ) : null}
    </form>
  );
}
