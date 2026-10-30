/**
 * Vitest & Node 26 native TypeScript/TSX test loader configuration for SIGO-Dental Frontend.
 * When loaded via `node --import ./vitest.config.ts --test`, registers a transparent ESM hook
 * so `.tsx` files execute natively using `stripTypeScriptTypes` without external npm packages.
 */
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

registerHooks({
  load(url, context, nextLoad) {
    if (url.endsWith(".tsx")) {
      const source = readFileSync(fileURLToPath(url), "utf8");
      const stripped = stripTypeScriptTypes(source, { mode: "strip", sourceUrl: url });
      return {
        format: "module",
        shortCircuit: true,
        source: stripped,
      };
    }
    return nextLoad(url, context);
  },
});

export default {
  test: {
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    globals: true,
  },
};
