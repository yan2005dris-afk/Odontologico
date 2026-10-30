import { describe, it } from "vitest";
import assert from "node:assert/strict";
import {
  SESSION_COOKIE_NAME,
  buildSessionCookieAttributes,
  parseSessionCookie,
  serializeSessionCookie,
  serializeSessionClearCookie,
} from "./session-cookie.ts";

describe("wi-13 S4 — httpOnly session cookie (RNF-05)", () => {
  const payload = {
    accessToken: "jwt-abc.def.ghi",
    role: "dentist" as const,
  };

  it("names the cookie sigo_session and marks it httpOnly, SameSite=Lax, Path=/", () => {
    const cookie = serializeSessionCookie(payload, { maxAgeMs: 900_000, secure: false });

    assert.ok(cookie.startsWith(`${SESSION_COOKIE_NAME}=`), cookie);
    assert.equal(SESSION_COOKIE_NAME, "sigo_session");
    assert.match(cookie, /; HttpOnly/);
    assert.match(cookie, /; SameSite=Lax/);
    assert.match(cookie, /; Path=\//);
  });

  it("converts the gateway idle timeout from ms to Max-Age seconds", () => {
    const cookie = serializeSessionCookie(payload, { maxAgeMs: 900_000, secure: false });

    assert.match(cookie, /; Max-Age=900(;|$)/);
  });

  it("adds Secure in production and omits it otherwise", () => {
    assert.match(
      serializeSessionCookie(payload, { maxAgeMs: 900_000, secure: true }),
      /; Secure/
    );
    assert.doesNotMatch(
      serializeSessionCookie(payload, { maxAgeMs: 900_000, secure: false }),
      /; Secure/
    );
  });

  it("round-trips the payload through the Cookie request header", () => {
    const cookie = serializeSessionCookie(payload, { maxAgeMs: 900_000, secure: false });
    const header = `theme=dark; ${cookie.split(";")[0]}; other=1`;

    assert.deepEqual(parseSessionCookie(header), payload);
  });

  it("keeps the display name used by the AppShell sidebar", () => {
    const cookie = serializeSessionCookie(
      { ...payload, userName: "Dra. Valeria Gómez" },
      { maxAgeMs: 900_000, secure: false }
    );

    assert.deepEqual(parseSessionCookie(cookie.split(";")[0]), {
      ...payload,
      userName: "Dra. Valeria Gómez",
    });
  });

  it("drops a tampered display name instead of trusting a non-string value", () => {
    const tampered = `${SESSION_COOKIE_NAME}=${encodeURIComponent(
      JSON.stringify({ accessToken: "jwt-abc.def.ghi", role: "dentist", userName: 42 })
    )}`;

    assert.deepEqual(parseSessionCookie(tampered), payload);
  });

  it("returns null for a missing or malformed cookie instead of throwing", () => {
    assert.equal(parseSessionCookie(undefined), null);
    assert.equal(parseSessionCookie(""), null);
    assert.equal(parseSessionCookie("theme=dark"), null);
    assert.equal(parseSessionCookie(`${SESSION_COOKIE_NAME}=not-json`), null);
  });

  it("clears the cookie with Max-Age=0", () => {
    const cookie = serializeSessionClearCookie({ secure: false });

    assert.ok(cookie.startsWith(`${SESSION_COOKIE_NAME}=`));
    assert.match(cookie, /; Max-Age=0/);
    assert.match(cookie, /; HttpOnly/);
  });

  it("exposes the cookie attributes used by the Next cookie store", () => {
    const attributes = buildSessionCookieAttributes(900_000, true);

    assert.deepEqual(attributes, {
      httpOnly: true,
      sameSite: "lax",
      path: "/",
      maxAge: 900,
      secure: true,
    });
  });

  it("never hands a negative Max-Age to the cookie store", () => {
    assert.equal(buildSessionCookieAttributes(-1, false).maxAge, 0);
  });
});
