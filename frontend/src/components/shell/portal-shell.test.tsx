/**
 * Portal shell contract (wi-14 S5, SRS §3, RNF-05, RNF-07).
 *
 * The shell is the frame every screen of the portal is rendered in, so these
 * tests pin what the operator must always find: the navigation of their role,
 * the route they are on, the global palette and the remaining idle time.
 */

import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

let pathname = "/patients";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => pathname,
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next-themes", () => ({
  useTheme: () => ({ theme: "light", setTheme: vi.fn(), themes: ["light", "dark"] }),
}));

import { PortalShell } from "./portal-shell.tsx";

function renderShell(role: "admin" | "receptionist" | "dentist" | "assistant" | "auditor" = "dentist") {
  return render(
    <PortalShell role={role} userName="Dra. Valeria Gómez">
      <p>Contenido clínico</p>
    </PortalShell>,
  );
}

describe("wi-14 S5 — PortalShell", () => {
  beforeEach(() => {
    pathname = "/patients";
  });

  it("renders the navigation of the session role", () => {
    renderShell("dentist");

    const nav = screen.getByRole("navigation", { name: "Navegación Principal SIGO-Dental" });
    expect(within(nav).getByRole("link", { name: /Pacientes/ })).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: /Agenda & Turnos/ })).toBeInTheDocument();
  });

  it("hides the modules the role cannot reach", () => {
    renderShell("dentist");

    const nav = screen.getByRole("navigation", { name: "Navegación Principal SIGO-Dental" });
    // The gateway rejects a dentist on /api/v1/billing (rbac_policy.go).
    expect(within(nav).queryByRole("link", { name: /Facturación POS/ })).not.toBeInTheDocument();
    expect(within(nav).queryByRole("link", { name: /Usuarios y Roles/ })).not.toBeInTheDocument();
  });

  it("gives the receptionist the billing entry", () => {
    renderShell("receptionist");

    const nav = screen.getByRole("navigation", { name: "Navegación Principal SIGO-Dental" });
    expect(within(nav).getByRole("link", { name: /Facturación POS/ })).toBeInTheDocument();
  });

  it("marks the active module in the navigation", () => {
    renderShell("dentist");

    const nav = screen.getByRole("navigation", { name: "Navegación Principal SIGO-Dental" });
    const active = within(nav).getByRole("link", { name: /Pacientes/ });
    expect(active).toHaveAttribute("aria-current", "page");
  });

  it("marks only the active entry with the data-active attribute", () => {
    renderShell("dentist");

    const nav = screen.getByRole("navigation", { name: "Navegación Principal SIGO-Dental" });
    const links = within(nav).getAllByRole("link");
    const active = links.filter((link) => link.hasAttribute("data-active"));

    // Tailwind v4 compiles `data-active:` to the attribute PRESENCE selector
    // `[data-active]`, so an inactive entry that still carried the attribute
    // would paint the active background too and the operator could not tell
    // which page they were on.
    expect(active).toHaveLength(1);
    expect(active[0]).toHaveAttribute("data-active", "true");
    expect(active[0]).toHaveTextContent("Pacientes");
  });

  it("shows the route in the breadcrumb", () => {
    renderShell("dentist");

    expect(screen.getByRole("link", { name: "SIGO-Dental" })).toBeInTheDocument();
    // The group label appears both in the sidebar and in the breadcrumb, so
    // the assertion is scoped to the breadcrumb landmark.
    const breadcrumb = screen.getByRole("navigation", { name: "breadcrumb" });
    expect(within(breadcrumb).getByText("Clínica & Turnos")).toBeInTheDocument();
    expect(within(breadcrumb).getByText("Pacientes")).toBeInTheDocument();
  });

  it("renders the children of the shell untouched", () => {
    renderShell("dentist");

    expect(screen.getByText("Contenido clínico")).toBeInTheDocument();
  });

  it("shows the remaining idle time of the session (RNF-05)", () => {
    renderShell("dentist");

    const indicator = screen.getByTestId("idle-guard-indicator");
    expect(indicator).toHaveTextContent("Timeout 15m");
    expect(indicator.textContent).toMatch(/\d+m \d+s restantes/);
  });

  it("opens the global palette with the keyboard shortcut", async () => {
    const user = userEvent.setup();
    renderShell("dentist");

    await user.keyboard("{Control>}k{/Control}");

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText(/Pacientes/)).toBeInTheDocument();
  });

  it("never indexes patients in the global palette", async () => {
    const user = userEvent.setup();
    renderShell("dentist");

    await user.keyboard("{Control>}k{/Control}");

    const dialog = await screen.findByRole("dialog");
    // The palette jumps to modules only: clinical records are searched in
    // their own screen, never in a box every role can open.
    expect(within(dialog).queryByText(/Ana Lucía/)).not.toBeInTheDocument();
    expect(
      within(dialog).getByPlaceholderText("Buscar módulo…"),
    ).toBeInTheDocument();
  });

  it("shows the signed-in operator and the role in the profile menu", async () => {
    const user = userEvent.setup();
    renderShell("dentist");

    await user.click(screen.getByRole("button", { name: /Dra\. Valeria Gómez/ }));

    const menu = await screen.findByRole("menu");
    expect(within(menu).getByText("Odontólogo")).toBeInTheDocument();
    expect(within(menu).getByRole("menuitem", { name: /Cerrar sesión/ })).toBeInTheDocument();
  });
});
