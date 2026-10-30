/**
 * Regression guard for the Tailwind CSS v4 pipeline (wi-13 S1, AGENTS.md §3).
 *
 * The portal was rendered with fully inert utility classes: the root layout
 * carried `class="bg-slate-50 ..."` but no stylesheet was ever imported, no
 * PostCSS plugin ran and Next.js emitted zero CSS. Nothing failed — the app
 * just looked unstyled.
 *
 * This suite pins the three links of that chain so it cannot silently break:
 * the global stylesheet exists and pulls Tailwind in, the root layout imports
 * it, and the PostCSS plugin that compiles Tailwind is wired.
 */
import { describe, it } from "vitest";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const read = (relativePath: string): string =>
  readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), "utf8");

describe("wi-13 S1 — Tailwind CSS pipeline emits the portal stylesheet (AGENTS.md §3)", () => {
  it("declares the global stylesheet with the Tailwind v4 entrypoint", () => {
    const css = read("../app/globals.css");

    assert.match(css, /@import\s+"tailwindcss"/, "globals.css must import tailwindcss");
  });

  it("imports the global stylesheet from the root layout", () => {
    const layout = read("../app/layout.tsx");

    assert.match(
      layout,
      /import\s+["']\.\/globals\.css["']/,
      "src/app/layout.tsx must import ./globals.css"
    );
  });

  it("wires the Tailwind PostCSS plugin so the utilities are compiled", () => {
    const postcss = read("../../postcss.config.mjs");

    assert.match(postcss, /@tailwindcss\/postcss/, "postcss.config.mjs must load @tailwindcss/postcss");
  });
});
