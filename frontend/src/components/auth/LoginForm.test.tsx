/**
 * Development role picker of the login form (wi-18 S3).
 *
 * The picker is a development convenience: it fills the credentials of one of
 * the seeded RBAC accounts so that switching roles does not need a TOTP code.
 * The contract that matters is that it only exists when the portal was built
 * with `NEXT_PUBLIC_DEV_ROLE_LOGIN=true`: a production build must not ship
 * the credentials of the seeded accounts.
 */

import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

let lastFormData: FormData | null = null;
const loginActionMock = vi.fn(async (previous: unknown, formData: FormData) => {
  lastFormData = formData;
  return previous;
});
vi.mock("../../app/(auth)/login/actions.ts", () => ({
  loginAction: (previous: unknown, formData: FormData) =>
    loginActionMock(previous, formData),
}));

import LoginForm from "./LoginForm.tsx";
import { DEV_ACCOUNTS } from "../../lib/dev-accounts.ts";

const ORIGINAL_FLAG = process.env.NEXT_PUBLIC_DEV_ROLE_LOGIN;

afterEach(() => {
  if (ORIGINAL_FLAG === undefined) {
    delete process.env.NEXT_PUBLIC_DEV_ROLE_LOGIN;
  } else {
    process.env.NEXT_PUBLIC_DEV_ROLE_LOGIN = ORIGINAL_FLAG;
  }
});

describe("wi-18 S3 — development role picker", () => {
  it("does not render the picker when the development flag is off", () => {
    delete process.env.NEXT_PUBLIC_DEV_ROLE_LOGIN;

    render(<LoginForm />);

    expect(screen.queryByTestId("dev-role-picker")).toBeNull();
  });

  it("renders one entry per seeded RBAC role when the flag is on", () => {
    process.env.NEXT_PUBLIC_DEV_ROLE_LOGIN = "true";

    render(<LoginForm />);

    const picker = screen.getByTestId("dev-role-picker");
    expect(picker).toBeTruthy();
    for (const account of DEV_ACCOUNTS) {
      expect(screen.getByRole("button", { name: new RegExp(account.displayName) })).toBeTruthy();
    }
  });

  it("submits the credentials of the chosen account", async () => {
    process.env.NEXT_PUBLIC_DEV_ROLE_LOGIN = "true";
    const user = userEvent.setup();

    render(<LoginForm />);

    await user.click(screen.getByRole("button", { name: /Lucía Fernández Ruiz/ }));

    // React resets the form once the action resolves, so the contract is
    // what was actually submitted, not what is left on screen.
    expect(lastFormData?.get("email")).toBe("recepcion@sigo-dental.com");
    expect(lastFormData?.get("password")).toBe("ReceptionPass#2026");
  });

  it("exposes exactly the 5 canonical roles of SRS §3", () => {
    expect(DEV_ACCOUNTS.map((account) => account.role)).toEqual([
      "admin",
      "dentist",
      "receptionist",
      "assistant",
      "auditor",
    ]);
  });
});
