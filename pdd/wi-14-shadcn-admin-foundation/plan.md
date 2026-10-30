# Plan de Work Item — `wi-14-shadcn-admin-foundation`

Feature: `F08` (`08-frontend-shadcn-admin-lists`) — Refactorización de Arquitectura Frontend (`shadcn-admin`) y Desacoplamiento de Pantallas de Lista.
Worktree: `wt/frontend` (`.worktrees/frontend`).
Rol activo: Planner → Implementer. `MaxCorrections`: `200`.
Estado: `plan_pending_approval`.
Base: commit `b4e60cf` (checkpoint pre-refactor). Spec: [`pdd/tasks/08-frontend-shadcn-admin-lists.md`](../tasks/08-frontend-shadcn-admin-lists.md) (`S1` de ese spec).

---

## 1. Descripción y alcance

El portal actual no tiene **ninguna** base de `shadcn/ui`: no existe `components.json`, no existe `frontend/src/components/ui/`, y `frontend/package.json` solo declara `next`, `react`, `react-dom` + Tailwind v4 y Vitest. Cero dependencias de Radix, TanStack, lucide, `class-variance-authority` o `tailwind-merge`. Todo el HTML de las pantallas se produce con funciones `render*Html(...)` que devuelven strings planos, inyectados con `dangerouslySetInnerHTML` en `patients/page.tsx:199,215`, `patients/[id]/page.tsx:177,232,266,298` y `agenda/page.tsx:294`.

Este work item **no migra ninguna pantalla**: instala el andamiaje (primitivos, shell, núcleo de `DataTable`) y **repara el arnés de tests**, que hoy es el bloqueo duro para poder testear React real. Los renderers siguen vivos hasta que `wi-16` los elimine. La entrega es un portal con el shell nuevo envolviendo las pantallas viejas, sin regresión funcional.

### Dentro del alcance (In Scope)

- **S1.** Dependencias del patrón `shadcn-admin` + `components.json` + lockfile versionado.
- **S2.** Tokens de diseño: extender el `@theme static` de `src/app/globals.css:21` con la paleta semántica de `shadcn/ui` **sin** alterar el contrato visual de `AGENTS.md §3`.
- **S3.** Arnés de tests: migrar el runner de `node --test` a `vitest run` con entorno `jsdom` y `Testing Library`; portar las 18 suites existentes sin tocar sus aserciones.
- **S4.** Primitivos `shadcn/ui` en `frontend/src/components/ui/`.
- **S5.** App Shell: `SidebarProvider` + `AppSidebar` + `Header` + `Breadcrumb` + `CommandMenu` (⌘K) + `ThemeSwitch` + `ProfileDropdown`.
- **S6.** Núcleo `DataTable` (`@tanstack/react-table`): tabla, toolbar con búsqueda y filtros facetados, selector de columnas, paginación y menú de acciones por fila.
- **S7.** Fuente única de verdad de navegación: `frontend/src/lib/nav-config.ts`, con grupos y filtrado por rol RBAC.
- **S8.** Montaje del nuevo shell en `src/app/(portal)/layout.tsx`, conservando las pantallas actuales sin cambios de comportamiento.

### Fuera del alcance (Out of Scope)

- Eliminar o reescribir los 14 renderers `render*Html` y sus suites: es `wi-16` (big bang, decisión tomada por el humano).
- Endpoints `GET` de listado del backend y corrección del ruteo del gateway: es `wi-15`.
- Las 12 pantallas de lista (`/treatments`, `/consents`, `/cash-registers`, `/users`, `/audit`, dashboard): son `wi-16` y `wi-17`.
- Persistencia real en PostgreSQL: los adaptadores `adapters/postgres/*` siguen siendo en memoria (verificado: `patients/adapters/postgres/patient_repository.go:20-26` usa `map` + `sync.RWMutex` y sólo conserva constantes SQL de referencia).
- Tema oscuro completo: `ThemeSwitch` existe y es funcional, pero la paleta oscura no se auditará contra contraste AA hasta `wi-17`.

---

## 2. Dependencias previas

- **`wi-13-frontend-next-port`** (mergeado en `0c689d6`): rutas reales de App Router, login MFA, cookie `sigo_session`, cliente de gateway server-side (`src/lib/gateway-server.ts:450`).
- **Checkpoint `b4e60cf`**: estado verde verificado antes de arrancar — frontend 111/111 tests, `next build` OK, y los tres servicios Go con `go test` + `go vet` limpios.
- **Worktree `wt/frontend`** creado desde `b4e60cf` en `.worktrees/frontend`. Requiere `pnpm install` dentro del worktree: no existe `pnpm-lock.yaml` en el repo y el worktree arranca sin `node_modules`.

---

## 3. Specs numeradas

- **S1 — Dependencias y scaffold.** `frontend/package.json` incorpora: `@tanstack/react-table`, `cmdk`, `lucide-react`, `class-variance-authority`, `clsx`, `tailwind-merge`, `next-themes`, `sonner`, `tw-animate-css`, los paquetes `@radix-ui/react-*` que exijan los primitivos de `S4`, y `jsdom` + `@testing-library/react` + `@testing-library/user-event` + `@testing-library/jest-dom` como devDependencies. Se genera `frontend/components.json` (estilo `new-york`, alias `@/*` → `./src/*`, `baseColor: slate`) y **se versiona `frontend/pnpm-lock.yaml`** para builds reproducibles. pnpm ≥ 12 aborta con `ERR_PNPM_IGNORED_BUILDS` salvo que los scripts de build estén permitidos: se declara `pnpm.onlyBuiltDependencies` para `esbuild`, `sharp` y `@tailwindcss/oxide` en `package.json`.
- **S2 — Tokens de diseño.** `src/app/globals.css` mantiene `@import "tailwindcss"` y su bloque `@theme static` actual, y suma las variables semánticas de `shadcn/ui` (`--background`, `--foreground`, `--card`, `--primary`, `--muted`, `--border`, `--ring`, `--destructive`, etc.) **derivadas de la escala `slate` existente**. Se respetan las tres invariantes de `AGENTS.md §3`: (a) `bg-slate-50` sigue siendo el fondo de aplicación; (b) `#FFFFFF` queda reservado al lienzo clínico activo (odontograma, periodontograma, gráficos); (c) los colores de alta saturación siguen siendo exclusivos de hallazgos clínicos `RF-03` (rojo = caries, azul = restauración) y de alertas médicas críticas `RF-02`. `--font-sans` y `--font-mono` no se tocan.
- **S3 — Arnés de tests.** `frontend/package.json` cambia el script `test` de `node --import ./vitest.config.ts --test <18 archivos>` a `vitest run`. `frontend/vitest.config.ts` se reduce a un `defineConfig` de Vitest con `environment: "jsdom"`, `globals: true`, `setupFiles` con `@testing-library/jest-dom/vitest` y `esbuild: { jsx: "automatic" }` (imprescindible: `tsconfig.json` usa `"jsx": "preserve"`, que Vitest no puede transformar). El `registerHooks`/`stripTypeScriptTypes` de `vitest.config.ts:6-23` se elimina, porque Vitest ya transpila TS/TSX con esbuild. Las 18 suites existentes cambian **sólo** el import `from "node:test"` → `from "vitest"`; `node:assert/strict` se conserva para no reescribir 111 aserciones. Los scripts `test:wi10..12` se unifican en `test`.
- **S4 — Primitivos `ui/`.** `frontend/src/components/ui/` con `button`, `input`, `label`, `badge`, `card`, `table`, `separator`, `sheet`, `dialog`, `alert-dialog`, `tabs`, `dropdown-menu`, `select`, `checkbox`, `popover`, `tooltip`, `scroll-area`, `avatar`, `collapsible`, `sonner`, `command`, `sidebar`. Cada primitivo se exporta como componente React real (no string). Se prohíben colores hardcodeados: todo color sale de los tokens de `S2`.
- **S5 — App Shell.** `frontend/src/components/shell/` entrega `SidebarProvider` + `AppSidebar` (colapsable a íconos, marca SIGO-Dental arriba, `NavGroup` por sección, `NavUser` abajo con el indicador vivo de los 15 min de inactividad de `RNF-05`), `Header` con `SidebarTrigger` + `Breadcrumb` por ruta activa, `CommandMenu` (⌘K) para saltar a paciente/turno/módulo, `ThemeSwitch` y `ProfileDropdown`. El shell consume la sesión del servidor (`src/app/(portal)/layout.tsx` ya resuelve el rol) y filtra los grupos por rol.
- **S6 — Núcleo `DataTable`.** `frontend/src/components/data-table/` con `DataTable`, `DataTableToolbar`, `DataTableSearch`, `DataTableFacetedFilter`, `DataTableViewOptions`, `DataTablePagination` y `DataTableRowActions`. Reglas: celdas numéricas (dinero, profundidades de sondaje, números de pieza FDI/Universal) con `tabular-nums`; ordenamiento y filtrado client-side sobre el set ya cargado por el servidor; estado de tabla (búsqueda, filtros, página, columnas visibles) serializado en la URL (`searchParams`) para que el estado sea compartible y el back/forward del navegador no lo pierda. La paginación client-side opera sobre el set completo entregado por el RSC: no se inventa paginación de servidor que el backend aún no expone (`wi-15` la habilitará donde aplique).
- **S7 — Navegación por rol.** Nuevo `frontend/src/lib/nav-config.ts` con la definición de grupos (**Clínica & Turnos**, **Caja & Operaciones**, **Administración & Seguridad**), sus ítems (ruta, etiqueta, ícono, rol mínimo) y `getAllowedNavigationModules(role)`. `PRIMARY_NAVIGATION_MODULES` y `getAllowedNavigationModules` migran desde `src/lib/session-idle-guard.ts:189,216` (que hoy mezcla timeout de sesión y configuración de navegación) a ese módulo; el test de filtrado por RBAC (`src/lib/session-idle-guard.test.ts`) se traslada a `src/lib/nav-config.test.ts`. `session-idle-guard.ts` queda sólo con la lógica de inactividad.
- **S8 — Montaje.** `src/app/(portal)/layout.tsx` monta `SidebarProvider` + `AppSidebar` + `Header` y renderiza `children` dentro del `main` con `bg-slate-50`. Las páginas existentes (`/patients`, `/patients/[id]`, `/agenda`, `/billing`, `/settlements`, `/inventory`) siguen produciendo exactamente el mismo HTML clínico: **cero cambios de comportamiento** en este work item.

---

## 4. Contrato de entrada

- **Variables de entorno**: ninguna nueva. Se conservan `GATEWAY_INTERNAL_URL`, `NEXT_PUBLIC_API_GATEWAY_URL` y `SESSION_COOKIE_NAME` (`src/lib/session-cookie.ts:10`).
- **Endpoints consumidos**: ninguno nuevo. El shell lee la sesión ya resuelta en el layout; no hay llamadas de red desde el shell.
- **Tipos reusados**: `UserRole` y `AppointmentStatus` de `src/lib/session-idle-guard.ts:5,58`; `PortalSession` de `src/lib/session.ts:14`. No se redefinen formas de datos ya tipadas.
- **Dependencia de toolchain**: Node ≥ 26 (soporta `stripTypeScriptTypes`, aunque deja de usarse), pnpm 12, Tailwind CSS v4.

---

## 5. Contrato de salida

- `pnpm test` ejecuta Vitest sobre las 18 suites portadas **más** las nuevas de primitivos, shell y `DataTable`; 0 fallos, 0 suites saltadas.
- `pnpm build` sale 0; `next build` lista las 11 rutas actuales sin errores.
- `pnpm lint` sin errores nuevos.
- `frontend/pnpm-lock.yaml` versionado; `pnpm install --frozen-lockfile` es reproducible.
- Las 11 rutas del portal siguen renderizando el mismo contenido clínico que antes del work item.

---

## 6. Seguridad y resiliencia

- **Sesión**: el shell no lee tokens ni cookies desde el cliente. El rol llega como prop de servidor desde `src/app/(portal)/layout.tsx`; el `CommandMenu` recibe sólo metadatos de navegación, nunca datos clínicos del paciente.
- **`CommandMenu` (⌘K)**: en esta fase indexa **módulos y rutas**, no pacientes. Indexar pacientes exigiría un endpoint de búsqueda y expondría datos clínicos en un buscador global; queda fuera de alcance y se evalúa en `wi-16` con el endpoint de lectura de `wi-15`.
- **RBAC**: `nav-config.ts` filtra grupos e ítems por rol; un rol sin permiso nunca recibe la ruta en el DOM del shell. La autorización real la sigue aplicando el gateway (`services/api-gateway/internal/ingress/domain/rbac_policy.go:88`), la navegación del shell es sólo affordance.
- **Inactividad `RNF-05`**: el indicador de 15 min del `NavUser` lee el estado del guard existente (`src/lib/session-idle-guard.ts`); no se reimplementa la cuenta regresiva.
- **Fuga de datos**: el shell no renderiza datos de pacientes ni importes. Ninguna cifra económica se pinta en el shell (`RN-02`).
- **Tema**: el `ThemeSwitch` usa `next-themes` con `attribute="class"`; si el DOM no soporta `class` en `<html>`, cae al modo sistema sin romper el render.

---

## 7. Archivos afectados

**Frontend — `wt/frontend`**
- `frontend/package.json` (deps, scripts `test`, `pnpm.onlyBuiltDependencies`), `frontend/pnpm-lock.yaml` (nuevo, versionado)
- `frontend/components.json` (nuevo)
- `frontend/vitest.config.ts` (reescrito como config de Vitest), `frontend/src/test-setup.ts` (nuevo)
- `frontend/src/app/globals.css` (tokens de `S2`)
- `frontend/src/components/ui/*` (~22 primitivos, nuevos)
- `frontend/src/components/shell/*` (`SidebarProvider`, `AppSidebar`, `Header`, `Breadcrumb`, `CommandMenu`, `ThemeSwitch`, `ProfileDropdown`)
- `frontend/src/components/data-table/*` (`DataTable` + 6 subcomponentes)
- `frontend/src/lib/nav-config.ts` (nuevo) + `frontend/src/lib/nav-config.test.ts` (nuevo)
- `frontend/src/lib/session-idle-guard.ts` (saca la config de navegación) + `frontend/src/lib/session-idle-guard.test.ts` (saca el test de navegación)
- `frontend/src/app/(portal)/layout.tsx` (monta el shell)
- Las 18 suites existentes: sólo el import `node:test` → `vitest`

**Fuera de `wt/frontend`**: ninguno. `wi-14` no toca los servicios Go ni el gateway.

---

## 8. Orden de ejecución (TDD)

| Task | Specs | Archivos | Prueba RED |
| :--- | :--- | :--- | :--- |
| T0 | S1 | `package.json`, lockfile | `pnpm install --frozen-lockfile` falla sin lockfile; `ERR_PNPM_IGNORED_BUILDS` |
| T1 | S3 | `vitest.config.ts`, setup, 18 suites | `vitest run` no encuentra suites (`node:test` no registra en Vitest) |
| T2 | S3 | `src/components/ui/button.test.tsx` | render + `@testing-library` sobre `<Button>` inexistente |
| T3 | S2, S4 | `globals.css`, primitivos restantes | cada primitivo con test de render + contrato de token |
| T4 | S7 | `nav-config.ts`, tests | grupos filtrados por rol; un rol sin permiso no recibe la ruta |
| T5 | S6 | `data-table/*`, tests | tabla con búsqueda, filtro facetado, columnas y paginación; `tabular-nums` en columnas numéricas |
| T6 | S5 | `shell/*`, tests | sidebar colapsable, breadcrumb por ruta, ⌘K abre el `CommandMenu` |
| T7 | S8 | `(portal)/layout.tsx`, test de ruta | el layout monta shell y conserva el contenido de la página |
| T8 | S1..S8 | todo | `pnpm test` + `pnpm build` + `pnpm lint` verdes |
| T9 | S1 | lockfile | `pnpm install --frozen-lockfile` en CI verde e idempotente |

**Criterio de salida del work item**: T9 verde, `review` de 4 lentes registrado en `pdd/wi-14-shadcn-admin-foundation/review.md` con citas `file:line`, y `apply.md` con los comandos ejecutados y sus salidas.
