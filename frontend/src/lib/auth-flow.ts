/**
 * Login & MFA authentication state machine (wi-13 S3, RNF-05).
 *
 * The machine is pure: it never touches the network, it only orchestrates the
 * `AuthGateway` port. The server action injects the real gateway client, the
 * unit tests inject a fake one.
 *
 * Security notes (plan §6):
 * - The TOTP code is forwarded once and never stored or logged.
 * - An invalid MFA code never reveals whether the account exists.
 */

import { isUserRole, type UserRole } from "./roles.ts";
import type {
  LoginWireResponse,
  VerifyMfaWireResponse,
} from "./gateway-server.ts";

export type AuthStep = "credentials" | "mfa" | "authenticated";

export interface AuthFlowError {
  code: string;
  message: string;
}

export interface AuthenticatedSession {
  accessToken: string;
  role: UserRole;
  userName: string;
  email: string;
  /** Idle timeout reported by the gateway; drives the session cookie Max-Age. */
  idleTimeoutMs: number;
}

export interface AuthFlowState {
  step: AuthStep;
  /** Echoed back to the form so the operator does not retype the email. */
  email?: string;
  /** Short-lived MFA challenge; never rendered into the HTML document. */
  challengeToken?: string;
  error?: AuthFlowError;
  session?: AuthenticatedSession;
}

export interface AuthGateway {
  login(email: string, password: string): Promise<LoginWireResponse>;
  verifyMfa(challengeToken: string, totpCode: string): Promise<VerifyMfaWireResponse>;
}

export interface AuthGatewayErrorLike {
  code?: string;
  status?: number;
  message?: string;
}

export const DEFAULT_IDLE_TIMEOUT_MS = 900_000;

/** Maps a gateway failure onto a safe, operator-facing message. */
export function mapAuthError(error: unknown): AuthFlowError {
  const candidate = (error ?? {}) as AuthGatewayErrorLike;
  const code = candidate.code ?? "AUTH_ERROR";
  const status = candidate.status ?? 0;

  switch (code) {
    case "INVALID_CREDENTIALS":
      return {
        code,
        message: "Credenciales inválidas. Verifique el correo profesional y la contraseña.",
      };
    case "INVALID_MFA_CODE":
      // Deliberately identical for unknown accounts and wrong codes.
      return {
        code,
        message: "Código MFA inválido. Verifique los 6 dígitos e intente nuevamente.",
      };
    case "ACCOUNT_LOCKED":
      return {
        code,
        message: "La cuenta está bloqueada temporalmente por intentos fallidos.",
      };
    case "VALIDATION_ERROR":
      return {
        code,
        message: "Faltan datos obligatorios: correo y contraseña.",
      };
    case "RATE_LIMIT_EXCEEDED":
      return {
        code,
        message: "Demasiados intentos de acceso. Espere unos segundos e intente nuevamente.",
      };
    case "GATEWAY_UNREACHABLE":
      return {
        code,
        message: "El servicio de autenticación no está disponible. Reintente en unos segundos.",
      };
    default:
      if (status >= 500) {
        return {
          code: "AUTH_UNAVAILABLE",
          message: "El servicio de autenticación no está disponible. Reintente en unos segundos.",
        };
      }
      return {
        code,
        message: candidate.message ?? "No se pudo completar el acceso.",
      };
  }
}

function toUserRole(value: unknown): UserRole {
  return isUserRole(value) ? value : "receptionist";
}

/** Builds the session payload from a successful MFA verification. */
export function toAuthenticatedSession(
  response: VerifyMfaWireResponse
): AuthenticatedSession {
  const idleTimeoutSeconds =
    typeof response?.idle_timeout_seconds === "number" && response.idle_timeout_seconds > 0
      ? response.idle_timeout_seconds
      : DEFAULT_IDLE_TIMEOUT_MS / 1000;

  return {
    accessToken: response?.access_token ?? "",
    role: toUserRole(response?.user?.role),
    userName: response?.user?.full_name ?? "",
    email: response?.user?.email ?? "",
    idleTimeoutMs: idleTimeoutSeconds * 1000,
  };
}

export interface AuthFlow {
  submitCredentials(
    state: AuthFlowState,
    input: { email: string; password: string }
  ): Promise<AuthFlowState>;
  submitTotp(
    state: AuthFlowState,
    input: { totpCode: string }
  ): Promise<AuthFlowState>;
}

/**
 * Creates the authentication state machine over the `AuthGateway` port.
 *
 * `submitCredentials` moves to the MFA step when the gateway answers with a
 * challenge; `submitTotp` exchanges the challenge for a session.
 */
export function createAuthFlow(gateway: AuthGateway): AuthFlow {
  return {
    async submitCredentials(state, input) {
      const email = input.email.trim();
      if (email === "" || input.password === "") {
        return {
          step: "credentials",
          email,
          error: {
            code: "VALIDATION_ERROR",
            message: "Ingrese su correo profesional y su contraseña.",
          },
        };
      }

      try {
        const response = await gateway.login(email, input.password);

        if (!response?.mfa_required) {
          return {
            step: "credentials",
            email,
            error: {
              code: "MFA_REQUIRED",
              message:
                "El servicio de identidad no habilitó la verificación MFA para esta cuenta.",
            },
          };
        }

        return {
          step: "mfa",
          email,
          challengeToken: response.challenge_token ?? "",
        };
      } catch (error) {
        return { step: "credentials", email, error: mapAuthError(error) };
      }
    },

    async submitTotp(state, input) {
      const challengeToken = state.challengeToken ?? "";
      const code = input.totpCode.trim();

      if (challengeToken === "") {
        return {
          step: "credentials",
          email: state.email,
          error: {
            code: "MFA_CHALLENGE_EXPIRED",
            message: "El desafío MFA expiró. Vuelva a iniciar sesión.",
          },
        };
      }

      if (!/^\d{6}$/.test(code)) {
        return {
          step: "mfa",
          email: state.email,
          challengeToken,
          error: {
            code: "TOTP_INVALID_FORMAT",
            message: "El código MFA debe tener exactamente 6 dígitos.",
          },
        };
      }

      try {
        const response = await gateway.verifyMfa(challengeToken, code);
        const session = toAuthenticatedSession(response);

        if (session.accessToken === "") {
          return {
            step: "mfa",
            email: state.email,
            challengeToken,
            error: {
              code: "AUTH_UNAVAILABLE",
              message: "El servicio de identidad no devolvió un token de sesión.",
            },
          };
        }

        return { step: "authenticated", email: state.email, session };
      } catch (error) {
        return { step: "mfa", email: state.email, challengeToken, error: mapAuthError(error) };
      }
    },
  };
}
