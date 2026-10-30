/**
 * Login & 6-Digit TOTP MFA Challenge Page (wi-10 S1, RNF-05).
 */

import { escapeHtml, validateTotpCode } from "../../../lib/session-idle-guard.ts";

export interface LoginPageProps {
  email?: string;
  mfaStep?: boolean;
  totpCode?: string;
  reason?: "idle_timeout" | "unauthorized";
}

export function renderLoginPageHtml(props: LoginPageProps = {}): string {
  const idleAlertHtml =
    props.reason === "idle_timeout"
      ? `<div role="alert" class="p-3 rounded bg-amber-100 border border-amber-400 text-amber-950 text-xs font-semibold">
          Sesión cerrada automáticamente tras 15 minutos de inactividad en estación clínica (RNF-05).
        </div>`
      : "";

  const totpCheck = props.totpCode ? validateTotpCode(props.totpCode) : { valid: true };

  return `<main role="main" class="min-h-screen bg-slate-50 flex items-center justify-center p-6">
    <div class="w-full max-w-md bg-white rounded-xl border border-slate-200 p-6 space-y-4 shadow-sm">
      <header>
        <h1 class="text-lg font-bold text-slate-900">Ingreso Clínico — SIGO-Dental</h1>
        <p class="text-xs text-slate-600">Autenticación segura con verificación MFA TOTP (RNF-05)</p>
      </header>
      ${idleAlertHtml}
      <form class="space-y-4">
        <div>
          <label class="block text-xs font-medium text-slate-700">Correo Profesional</label>
          <input type="email" required value="${escapeHtml(props.email ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-sm" />
        </div>
        <div>
          <label class="block text-xs font-medium text-slate-700">Código MFA TOTP (6 dígitos)</label>
          <input type="text" inputmode="numeric" maxlength="6" pattern="\\d{6}" value="${escapeHtml(props.totpCode ?? "")}" class="mt-1 w-full rounded border border-slate-300 px-3 py-2 text-center font-mono text-base tracking-widest tabular-nums" />
          ${!totpCheck.valid ? `<p role="alert" class="text-xs text-red-600 mt-1">${escapeHtml(totpCheck.error ?? "")}</p>` : ""}
        </div>
        <button type="submit" class="w-full min-h-[44px] rounded bg-slate-900 text-white text-sm font-semibold">
          Verificar e Ingresar
        </button>
      </form>
    </div>
  </main>`;
}

export default renderLoginPageHtml;
