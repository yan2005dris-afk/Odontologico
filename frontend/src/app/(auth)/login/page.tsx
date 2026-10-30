/**
 * Login & 6-Digit TOTP MFA page (wi-13 S3, RNF-05).
 *
 * The form is a real client component driven by the `loginAction` server
 * action: credentials first, then the TOTP challenge. The page itself only
 * renders the shell and the idle-timeout notice.
 */

import LoginForm from "../../../components/auth/LoginForm.tsx";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const reason = typeof params.reason === "string" ? params.reason : undefined;

  return (
    <main role="main" className="min-h-screen bg-slate-50 flex items-center justify-center p-6">
      <div className="w-full max-w-md bg-white rounded-xl border border-slate-200 p-6 space-y-4 shadow-sm">
        <header>
          <h1 className="text-lg font-bold text-slate-900">Ingreso Clínico — SIGO-Dental</h1>
          <p className="text-xs text-slate-600">
            Autenticación segura con verificación MFA TOTP (RNF-05)
          </p>
        </header>

        {reason === "idle_timeout" ? (
          <div
            role="alert"
            className="p-3 rounded bg-amber-100 border border-amber-400 text-amber-950 text-xs font-semibold"
          >
            Sesión cerrada automáticamente tras 15 minutos de inactividad en estación clínica (RNF-05).
          </div>
        ) : null}

        <LoginForm />
      </div>
    </main>
  );
}
