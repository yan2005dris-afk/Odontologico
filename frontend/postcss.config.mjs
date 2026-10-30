/**
 * PostCSS pipeline for the SIGO-Dental portal (AGENTS.md §3).
 *
 * Tailwind CSS v4 ships its own PostCSS plugin (`@tailwindcss/postcss`) and
 * requires no `tailwind.config.js`: the design tokens live in
 * `src/app/globals.css` under `@theme`.
 *
 * Without this file Next.js never runs Tailwind, no stylesheet is emitted and
 * every utility class in the portal (`bg-slate-50`, `tabular-nums`, ...) is
 * inert markup.
 */
const config = {
  plugins: {
    "@tailwindcss/postcss": {},
  },
};

export default config;
