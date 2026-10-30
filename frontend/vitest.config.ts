/**
 * Vitest configuration for the SIGO-Dental portal (wi-14 S3).
 *
 * The portal moved from the Node native test runner (`node --import
 * ./vitest.config.ts --test ...`) to real Vitest. The old harness could not
 * test a React component: the suites imported `describe`/`it` from
 * `node:test`, which does not register with Vitest ("No test suite found"),
 * and the environment was `node`, so there was no DOM for `@testing-library`
 * to render into.
 *
 * Three settings are load-bearing:
 *   - `environment: "jsdom"` gives the suites a DOM.
 *   - `esbuild: { jsx: "automatic" }` makes esbuild compile the JSX of the
 *     `.tsx` suites. `tsconfig.json` keeps `"jsx": "preserve"` because that is
 *     what Next.js/SWC expects, so the transform has to be forced here.
 *   - `resolve.alias` mirrors the `"@/*" -> "./src/*"` path of `tsconfig.json`,
 *     which Next.js applies on its own but Vitest does not.
 */

import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  esbuild: {
    jsx: "automatic",
    jsxImportSource: "react",
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    setupFiles: ["./src/test-setup.ts"],
  },
});
