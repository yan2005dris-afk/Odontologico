import { describe, it } from "vitest";
import assert from "node:assert/strict";
import { createAuthFlow, mapAuthError, type AuthFlowState } from "./auth-flow.ts";
import { GatewayError } from "./gateway-server.ts";

describe("wi-13 S3 — login & MFA authentication state machine (RNF-05)", () => {
  function gatewayWith(overrides: Partial<{
    login: () => Promise<unknown>;
    verifyMfa: () => Promise<unknown>;
  }> = {}) {
    const calls: { login: number; verifyMfa: number } = { login: 0, verifyMfa: 0 };
    const gateway = {
      async login(email: string, password: string) {
        calls.login += 1;
        if (overrides.login) {
          return (await overrides.login()) as never;
        }
        return { mfa_required: true, challenge_token: "challenge-abc", expires_in_seconds: 300 };
      },
      async verifyMfa(challengeToken: string, totpCode: string) {
        calls.verifyMfa += 1;
        if (overrides.verifyMfa) {
          return (await overrides.verifyMfa()) as never;
        }
        return {
          access_token: "jwt-token",
          session_id: "sess-1",
          user: {
            id: "usr-1",
            email: "dra.gomez@sigo-dental.com",
            full_name: "Dra. Valeria Gómez",
            role: "dentist",
            mfa_verified: true,
          },
          idle_timeout_seconds: 900,
        };
      },
    };
    return { gateway, calls, loginArgs: { email: "", password: "" } };
  }

  it("moves to the MFA step when the gateway answers with a challenge", async () => {
    const { gateway, calls } = gatewayWith();
    const flow = createAuthFlow(gateway);

    const state = await flow.submitCredentials({ step: "credentials" }, {
      email: "dra.gomez@sigo-dental.com",
      password: "ClinicalPass#2026",
    });

    assert.equal(state.step, "mfa");
    assert.equal(state.challengeToken, "challenge-abc");
    assert.equal(state.email, "dra.gomez@sigo-dental.com");
    assert.equal(state.error, undefined);
    assert.equal(calls.login, 1);
  });

  it("rejects an empty credential pair without calling the gateway", async () => {
    const { gateway, calls } = gatewayWith();
    const flow = createAuthFlow(gateway);

    const state = await flow.submitCredentials({ step: "credentials" }, {
      email: "",
      password: "",
    });

    assert.equal(state.step, "credentials");
    assert.equal(state.error?.code, "VALIDATION_ERROR");
    assert.equal(calls.login, 0);
  });

  it("surfaces invalid credentials as 401 INVALID_CREDENTIALS", async () => {
    const { gateway } = gatewayWith({
      login: async () => {
        throw new GatewayError("INVALID_CREDENTIALS", "invalid", 401);
      },
    });
    const flow = createAuthFlow(gateway);

    const state = await flow.submitCredentials({ step: "credentials" }, {
      email: "dra.gomez@sigo-dental.com",
      password: "wrong",
    });

    assert.equal(state.step, "credentials");
    assert.equal(state.error?.code, "INVALID_CREDENTIALS");
    assert.match(state.error?.message ?? "", /Credenciales inválidas/);
  });

  it("exchanges a valid TOTP for a session with role and idle timeout", async () => {
    const { gateway, calls } = gatewayWith();
    const flow = createAuthFlow(gateway);

    const state = await flow.submitTotp(
      { step: "mfa", challengeToken: "challenge-abc", email: "dra.gomez@sigo-dental.com" },
      { totpCode: "123456" }
    );

    assert.equal(state.step, "authenticated");
    assert.equal(state.session?.accessToken, "jwt-token");
    assert.equal(state.session?.role, "dentist");
    assert.equal(state.session?.userName, "Dra. Valeria Gómez");
    assert.equal(state.session?.idleTimeoutMs, 900_000);
    assert.equal(calls.verifyMfa, 1);
  });

  it("rejects a malformed TOTP locally without calling the gateway", async () => {
    const { gateway, calls } = gatewayWith();
    const flow = createAuthFlow(gateway);

    const state = await flow.submitTotp(
      { step: "mfa", challengeToken: "challenge-abc" },
      { totpCode: "12ab" }
    );

    assert.equal(state.step, "mfa");
    assert.equal(state.error?.code, "TOTP_INVALID_FORMAT");
    assert.equal(calls.verifyMfa, 0);
  });

  it("never reveals whether the account exists when the MFA code is wrong", async () => {
    const { gateway } = gatewayWith({
      verifyMfa: async () => {
        throw new GatewayError("INVALID_MFA_CODE", "invalid", 401);
      },
    });
    const flow = createAuthFlow(gateway);

    const state = await flow.submitTotp(
      { step: "mfa", challengeToken: "challenge-abc" },
      { totpCode: "000000" }
    );

    assert.equal(state.step, "mfa");
    assert.equal(state.error?.code, "INVALID_MFA_CODE");
    assert.doesNotMatch(state.error?.message ?? "", /no existe|inexistente|usuario/i);
  });

  it("reports an expired challenge instead of verifying without one", async () => {
    const { gateway, calls } = gatewayWith();
    const flow = createAuthFlow(gateway);

    const state = await flow.submitTotp({ step: "mfa" }, { totpCode: "123456" });

    assert.equal(state.step, "credentials");
    assert.equal(state.error?.code, "MFA_CHALLENGE_EXPIRED");
    assert.equal(calls.verifyMfa, 0);
  });

  it("maps an unreachable gateway to a retryable message", () => {
    const error = mapAuthError(new GatewayError("GATEWAY_UNREACHABLE", "boom", 0));

    assert.equal(error.code, "GATEWAY_UNREACHABLE");
    assert.match(error.message, /no está disponible/i);
  });

  it("never leaks the TOTP code into the returned state", async () => {
    const { gateway } = gatewayWith({
      verifyMfa: async () => {
        throw new GatewayError("INVALID_MFA_CODE", "invalid", 401);
      },
    });
    const flow = createAuthFlow(gateway);

    const state: AuthFlowState = await flow.submitTotp(
      { step: "mfa", challengeToken: "challenge-abc" },
      { totpCode: "654321" }
    );

    assert.doesNotMatch(JSON.stringify(state), /654321/);
  });
});

describe("wi-18 S2 — login without the MFA step (development mode)", () => {
  it("opens the session straight away when the identity service issues one", async () => {
    const flow = createAuthFlow({
      async login() {
        return {
          mfa_required: false,
          access_token: "jwt-token-dev",
          session_id: "sess-dev",
          user: {
            id: "usr-1",
            email: "recepcion@sigo-dental.com",
            full_name: "Lucía Fernández Ruiz",
            role: "receptionist",
            mfa_verified: true,
          },
          idle_timeout_seconds: 900,
        } as never;
      },
      async verifyMfa() {
        throw new Error("the MFA step must not be reached");
      },
    });

    const state = await flow.submitCredentials({ step: "credentials" }, {
      email: "recepcion@sigo-dental.com",
      password: "ReceptionPass#2026",
    });

    assert.equal(state.step, "authenticated");
    assert.equal(state.session?.accessToken, "jwt-token-dev");
    assert.equal(state.session?.role, "receptionist");
    assert.equal(state.session?.userName, "Lucía Fernández Ruiz");
    assert.equal(state.session?.idleTimeoutMs, 900_000);
    assert.equal(state.challengeToken, undefined);
  });

  it("keeps demanding the MFA challenge when the service asks for one", async () => {
    const flow = createAuthFlow({
      async login() {
        return { mfa_required: true, challenge_token: "challenge-abc" } as never;
      },
      async verifyMfa() {
        throw new Error("not reached");
      },
    });

    const state = await flow.submitCredentials({ step: "credentials" }, {
      email: "dra.gomez@sigo-dental.com",
      password: "ClinicalPass#2026",
    });

    assert.equal(state.step, "mfa");
    assert.equal(state.challengeToken, "challenge-abc");
  });
});
