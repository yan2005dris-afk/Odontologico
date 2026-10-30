/**
 * ESLint flat configuration of the SIGO-Dental portal (wi-14 T0).
 *
 * `next lint` was removed from the canonical command set because it prompts
 * for an interactive setup when the project has no configuration: `pnpm lint`
 * failed with a question instead of linting anything. The script now calls
 * ESLint directly, with this flat config.
 */

import { dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { FlatCompat } from "@eslint/eslintrc";

const compat = new FlatCompat({
  baseDirectory: dirname(fileURLToPath(import.meta.url)),
});

export default [
  ...compat.extends("next/core-web-vitals", "next/typescript"),
  {
    ignores: [
      ".next/**",
      "node_modules/**",
      "out/**",
      "dist/**",
      "coverage/**",
      "next-env.d.ts",
    ],
  },
];
