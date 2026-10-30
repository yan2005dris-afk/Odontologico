/**
 * Contract of the `shadcn/ui` primitives (wi-14 S2/S4).
 *
 * The primitives are vendored from the shadcn registry, so they are the part
 * of the frontend most likely to be overwritten by a future
 * `shadcn add --overwrite`. These tests pin the two properties the portal
 * depends on:
 *
 *   1. Every primitive is a real React component. The portal has no string
 *      renderer left in `ui/` (the legacy ones live in the feature folders
 *      and are removed by wi-16).
 *   2. Every colour comes from the semantic tokens of `globals.css`. A
 *      literal `#dc2626` or a `bg-slate-900` inside a primitive would bypass
 *      the palette derived from the slate scale and break the AGENTS.md §3
 *      visual contract.
 */

import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const UI_DIR = join(process.cwd(), "src", "components", "ui");
const GLOBALS_CSS = join(process.cwd(), "src", "app", "globals.css");

function uiFiles(): { name: string; source: string }[] {
  return readdirSync(UI_DIR)
    .filter((name) => name.endsWith(".tsx") && !name.includes(".test."))
    .map((name) => ({ name, source: readFileSync(join(UI_DIR, name), "utf8") }));
}

/** The semantic colour tokens the primitives are allowed to reference. */
const TOKEN_GROUPS = [
  "background",
  "foreground",
  "card",
  "card-foreground",
  "popover",
  "popover-foreground",
  "primary",
  "primary-foreground",
  "secondary",
  "secondary-foreground",
  "muted",
  "muted-foreground",
  "accent",
  "accent-foreground",
  "destructive",
  "border",
  "input",
  "ring",
  "sidebar",
  "sidebar-foreground",
  "sidebar-primary",
  "sidebar-primary-foreground",
  "sidebar-accent",
  "sidebar-accent-foreground",
  "sidebar-border",
  "sidebar-ring",
];

describe("wi-14 S4 — the shadcn/ui primitives", () => {
  it("ships the components the portal needs", () => {
    const names = uiFiles().map((file) => file.name);
    for (const expected of [
      "button.tsx",
      "input.tsx",
      "label.tsx",
      "badge.tsx",
      "card.tsx",
      "table.tsx",
      "separator.tsx",
      "sheet.tsx",
      "dialog.tsx",
      "alert-dialog.tsx",
      "tabs.tsx",
      "dropdown-menu.tsx",
      "select.tsx",
      "checkbox.tsx",
      "popover.tsx",
      "tooltip.tsx",
      "scroll-area.tsx",
      "avatar.tsx",
      "collapsible.tsx",
      "sonner.tsx",
      "command.tsx",
      "sidebar.tsx",
      "breadcrumb.tsx",
    ]) {
      expect(names, `falta el primitivo ${expected}`).toContain(expected);
    }
  });

  it("exports real React components and never a string renderer", () => {
    for (const file of uiFiles()) {
      // The registry files declare the components and export them either
      // inline (`export function X`) or through a trailing `export { ... }`.
      const declaresComponent =
        /export\s+(function|const)\s+\w+/.test(file.source) ||
        /export\s*\{[\s\S]*?\w+[\s\S]*?\}/.test(file.source);
      expect(declaresComponent, `${file.name} no exporta ningún componente`).toBe(true);
      // The legacy pattern that wi-16 removes from the feature folders: a
      // component that returns an HTML string and injects it with
      // `dangerouslySetInnerHTML`.
      expect(
        file.source.match(/export\s+function\s+render\w+Html/g),
        `${file.name} expone un renderer de HTML string`,
      ).toBeNull();
      expect(
        file.source.match(/dangerouslySetInnerHTML/g),
        `${file.name} inyecta HTML crudo`,
      ).toBeNull();
    }
  });

  it("never hardcodes a colour", () => {
    for (const file of uiFiles()) {
      expect(
        file.source.match(/#[0-9a-fA-F]{3,8}\b/g),
        `${file.name} contiene un color literal`,
      ).toBeNull();
      expect(
        file.source.match(/\b(oklch|rgba?|hsla?)\(/g),
        `${file.name} contiene un color literal`,
      ).toBeNull();
    }
  });

  it("never hardcodes a palette colour, only the semantic tokens", () => {
    const paletteUtility =
      /\b(?:bg|text|border|ring|fill|stroke|outline|decoration|shadow|from|to|via)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-[0-9]{2,3}\b/g;

    for (const file of uiFiles()) {
      expect(
        file.source.match(paletteUtility),
        `${file.name} usa una paleta literal en vez de un token`,
      ).toBeNull();
    }
  });

  it("only references tokens that the stylesheet actually defines", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    const usedTokens = new Set<string>();

    for (const file of uiFiles()) {
      const matches = file.source.match(
        /\b(?:bg|text|border|ring|fill|stroke|outline)-(background|foreground|card|card-foreground|popover|popover-foreground|primary|primary-foreground|secondary|secondary-foreground|muted|muted-foreground|accent|accent-foreground|destructive|input|ring|sidebar|sidebar-foreground|sidebar-primary|sidebar-primary-foreground|sidebar-accent|sidebar-accent-foreground|sidebar-border|sidebar-ring)\b/g,
      );
      for (const match of matches ?? []) {
        usedTokens.add(match.split("-").slice(1).join("-"));
      }
    }

    expect(usedTokens.size).toBeGreaterThan(0);
    for (const token of usedTokens) {
      expect(
        css,
        `el token "${token}" se usa en los primitivos pero no está en globals.css`,
      ).toContain(`--color-${token}:`);
    }
  });

  it("keeps the slate palette as the only source of the semantic tokens", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    // Every semantic token of the light theme is a slate value of the Tailwind
    // scale, except the destructive colour, which is the functional danger
    // red and is deliberately not a clinical finding colour.
    const root = css.slice(css.indexOf(":root {"), css.indexOf(".dark {"));
    const slateValues = new Set([
      "#f8fafc",
      "#ffffff",
      "#0f172a",
      "#f1f5f9",
      "#64748b",
      "#e2e8f0",
      "#94a3b8",
      "#475569",
      "#334155",
      "#1e293b",
      "#020617",
    ]);

    for (const line of root.split("\n")) {
      const match = line.match(/--[a-z-]+:\s*(#[0-9a-fA-F]{6});/);
      if (!match) {
        continue;
      }
      const [, value] = match;
      const isDestructive = line.includes("--destructive");
      if (isDestructive) {
        continue;
      }
      expect(
        slateValues.has(value),
        `el token "${line.trim()}" usa ${value}, que no pertenece a la escala slate`,
      ).toBe(true);
    }
  });

  it("declares every token the primitives need", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    for (const token of TOKEN_GROUPS) {
      expect(css, `falta el token --color-${token}`).toContain(`--color-${token}:`);
    }
  });
});
