# Apply — `wi-14-shadcn-admin-foundation`

Feature: `F08` (`08-frontend-shadcn-admin-lists`) — `S1` del spec.
Worktree: `wt/frontend` (`.worktrees/frontend`).
Base: commit `b4e60cf`.
Estado: **implementado**, pendiente de `review`.

---

## 1. Resumen de lo ejecutado

| Spec | Qué se hizo | Resultado |
| :--- | :--- | :--- |
| **S1** | Dependencias `shadcn/ui`, `components.json`, lockfile versionado, `pnpm-workspace.yaml` con `allowBuilds`, ESLint + flat config, `Dockerfile` con pnpm y `--frozen-lockfile` | OK |
| **S2** | Tokens semánticos derivados de la escala `slate`; paleta neutra del scaffold y webfont `Geist` rechazados | OK |
| **S3** | Arnés de tests migrado de `node --test` a `vitest run` con `jsdom` + Testing Library; 18 suites portadas | OK |
| **S4** | 26 primitivos en `src/components/ui/` + test de contrato de tokens | OK |
| **S5** | App Shell: `PortalShell`, `AppSidebar`, `AppHeader`, `CommandMenu`, `ThemeSwitch`, `ProfileDropdown`, `NavIcon` | OK |
| **S6** | Núcleo `DataTable`: tabla, toolbar, filtro facetado, selector de columnas, paginación y estado en la URL | OK |
| **S7** | `nav-config.ts` (grupos + filtro RBAC) y `roles.ts` (vocabulario único de roles) | OK |
| **S8** | Shell montado en `(portal)/layout.tsx`; las pantallas clínicas conservan su contenido | OK |

**Cambio total:** 89 archivos, **+13 326 / −268** líneas. 55 archivos nuevos, 32 modificados, 2 eliminados (`AppShellView.tsx` y `SidebarNav.tsx`, React legacy superseded por el shell nuevo).

---

## 2. Comandos ejecutados y sus salidas

### 2.1 Línea base (antes de tocar nada)

```
$ pnpm test
  Test Files  18 passed (18)
       Tests  111 passed (111)

$ pnpm build
  ✓ Compiled successfully
  Route (app)  11 rutas

$ cd services/{api-gateway,clinical-agenda,billing-stock} && go test ./... && go vet ./...
  (sin salida: todo verde)
```

### 2.2 Toolchain (S1)

```
$ pnpm dlx shadcn@latest init -t next -b radix --preset nova --yes -c . --force
  ✔ Writing components.json.
  ✔ Created 2 files: src/components/ui/button.tsx, src/lib/utils.ts
  ✔ Updating src/app/globals.css

$ pnpm dlx shadcn@latest add button input label badge card table separator sheet dialog \
    alert-dialog tabs dropdown-menu select checkbox popover tooltip scroll-area avatar \
    collapsible sonner command sidebar breadcrumb --yes -c . --overwrite
  ✔ Created 26 files in src/components/ui/

$ pnpm add @tanstack/react-table@^8        # 8.21.3
$ pnpm add -D jsdom @testing-library/react @testing-library/user-event @testing-library/jest-dom
$ pnpm add -D eslint@^9 eslint-config-next@^15 @eslint/eslintrc
```

**Desviación documentada — `@tanstack/react-table`:** `pnpm add @tanstack/react-table` resolvió **v9.3.1**, cuya API es incompatible (`useTable` en vez de `useReactTable`, `createCoreRowModel` en vez de `getCoreRowModel`, features en vez de row models). El subpath `./legacy` de v9 expone los row models pero **no** `useReactTable`, `flexRender` ni `createColumnHelper`, así que no es un sustituto de v8. Se fijó **v8.21.3**, que es la API a la que apunta todo el ecosistema `shadcn` (incluido su bloque `data-table`).

### 2.3 Tests (S3)

```
$ pnpm test
  Test Files  23 passed (23)
       Tests  160 passed (160)
```

Las 18 suites existentes sólo cambiaron el import `from "node:test"` → `from "vitest"`; sus 111 aserciones (`node:assert/strict`) se conservaron intactas. Se añadieron 49 tests nuevos: `nav-config` (11), `table-state` (8), `data-table` (13), `portal-shell` (11), `primitives` (7).

```
$ npx tsc --noEmit
  (sin salida)

$ pnpm build
  ✓ Compiled successfully in 6.3s
  ✓ Generating static pages (12/12)

$ pnpm lint
  ✖ 7 problems (0 errors, 7 warnings)
  EXIT=0
```

### 2.4 Verificación end-to-end contra el stack en marcha

Se levantó el build nuevo en el puerto 3100 (`GATEWAY_INTERNAL_URL=http://localhost:8080`) y se ejecutó el login completo contra el **gateway real**:

```
POST /api/v1/auth/login      {"email":"dra.gomez@sigo-dental.com","password":"ClinicalPass#2026"}
  → 200 {mfa_required: true, challenge_token: "..."}
POST /api/v1/auth/mfa/verify {"challenge_token":"...","totp_code":"341529"}
  → 303 See Other · Location: /patients
  → Set-Cookie: sigo_session=...; Path=/; Max-Age=900; HttpOnly; SameSite=lax
GET /patients  (con la cookie)
  → 200
```

Inspección del HTML servido con el shell nuevo:

```
data-sidebar          SI      <aside>:0  <header>:2  <main>:2  <nav>:1
data-sidebar="sidebar" SI
"Buscar módulo"       SI      (CommandMenu ⌘K)
"Timeout 15m"         SI      (indicador de inactividad RNF-05)
"Alta de Paciente"    SI      (la pantalla clínica conserva su contenido)
"Ana Lucía" / "HC-2026" SI    (los datos del gateway llegan intactos)
```

Errores en el log del servidor: **0**.

### 2.5 Verificación con navegador real

Se condujo el portal con Chromium (Playwright, `playwright-core` fuera del repo) siguiendo el flujo real de la interfaz: rellenar el correo y la contraseña, esperar el paso MFA, escribir el TOTP, y navegar.

Capturas en `/tmp/shot/` (`01-login.png`, `02-mfa.png`, `03-portal-pacientes.png`, `04-command-menu.png`, `05-agenda.png`).

La inspección del DOM computado encontró el defecto 13: los cinco ítems del menú tenían `background: rgb(241, 245, 249)`. Corregido y reverificado:

```
Dashboard          data-active=null   bg=rgba(0, 0, 0, 0)
Agenda & Turnos    data-active=null   bg=rgba(0, 0, 0, 0)
Pacientes          data-active="true" bg=rgb(241, 245, 249)
Liquidaciones      data-active=null   bg=rgba(0, 0, 0, 0)
Inventario & Recetas data-active=null  bg=rgba(0, 0, 0, 0)
```

El rol `dentist` **no** ve "Facturación POS": el filtro RBAC de la navegación coincide con la política del gateway.

---

## 3. Defectos encontrados y corregidos durante la ejecución

Cada uno de estos salió de una prueba o de una verificación real, no de una lectura:

| # | Defecto | Origen | Corrección |
| :--- | :--- | :--- | :--- |
| 1 | `readTableState` devolvía `sorting: undefined` con la query vacía: el `?.` cortocircuitaba toda la cadena de `.map()` | `table-state.test.ts` | `?? []` al final de la cadena (`table-state.ts:70-83`) |
| 2 | `writeTableState` escribía siempre `?patients.ps=10`, ensuciando la URL | `table-state.test.ts` | el tamaño de página sólo se escribe cuando difiere del default (`table-state.ts:143-147`) |
| 3 | El filtro facetado guardaba un **array** y `equalsString` esperaba un string → `filterValue.toLowerCase is not a function` | `data-table.test.tsx` | `createMultiSelectFilter<TData>()` instalado automáticamente en toda columna con facet declarado (`column-helpers.ts`, `data-table.tsx:117-140`) |
| 4 | El `CommandDialog` del scaffold `radix-nova` **no envolvía sus hijos en `<Command>`** → `Cannot read properties of undefined (reading 'subscribe')` al abrir la paleta | test de `portal-shell` + log del servidor | wrapper `<Command>` añadido en `ui/command.tsx`, con nota de que un `shadcn add --overwrite` lo revierte |
| 5 | `Tooltip` must be used within `TooltipProvider` — el sidebar de shadcn usa tooltips internos | log del servidor (`⨯ Error`) | `TooltipProvider` dentro de `PortalShell`, para que el shell sea autocontenido |
| 6 | El `Sidebar` de shadcn renderiza un `<div>`: **perdía el landmark `role="navigation"`** que el shell anterior sí tenía | `portal-shell.test.tsx` | `<nav aria-label="Navegación Principal SIGO-Dental">` propio en `app-sidebar.tsx` |
| 7 | El efecto de sincronización con la URL podía encadenar `router.replace` sin fin (dependía de `searchParams`, que cambia tras cada reemplazo) | revisión propia de `data-table.tsx` | guarda de igualdad URL actual vs. destino + test de regresión `does not rewrite the URL when the state did not change` |
| 8 | `@import "shadcn/tailwind.css"` apuntaba a un archivo **inexistente** (el paquete `shadcn` no trae ningún CSS): import muerto que además exigiría `shadcn` en runtime | inspección de `node_modules/shadcn` | import eliminado; el paquete `shadcn` (CLI de scaffolding) retirado de las dependencias |
| 9 | `next lint` estaba roto: pedía configuración interactiva en vez de lintar | `pnpm lint` | ESLint 9 + `eslint-config-next` 15 + `eslint.config.mjs` (flat config) y script `eslint .` |
| 10 | Next infería el workspace root desde un `package-lock.json` ajeno y advierte; el `output: "standalone"` podía copiar de menos | `pnpm build` | `outputFileTracingRoot: process.cwd()` en `next.config.ts` |
| 11 | pnpm ≥ 12 aborta con `ERR_PNPM_IGNORED_BUILDS`: el campo `pnpm` del `package.json` ya no se lee | `pnpm install` | `pnpm-workspace.yaml` con el mapa `allowBuilds` (la lista `onlyBuiltDependencies` quedó obsoleta) |
| 12 | Dos errores de lint **preexistentes** (`<a>` en vez de `<Link>`) en las páginas de pacientes, heredados de wi-13, impedían que `pnpm lint` terminara en 0 | `pnpm lint` | `<Link>` importado y usado en `patients/page.tsx:142` y `patients/[id]/page.tsx:120` |
| 13 | **El ítem activo de la sidebar era indistinguible**: Tailwind v4 compila el variant `data-active:` al selector de *presencia* `[data-active]`, y React renderiza `data-active="false"` en los ítems inactivos → la regla `data-active:bg-sidebar-accent` pintaba el fondo activo en **los 5** ítems del menú | inspección del DOM computado con navegador real (`getComputedStyle`) | `data-active={isActive ? "true" : undefined}` en `ui/sidebar.tsx` (2 ocurrencias) + test de regresión en `portal-shell.test.tsx` |
| 14 | **Banner de validación falso en `/patients`**: `LEGAL_GUARDIAN_REQUIRED_FOR_MINOR` al cargar la página. `calculatePatientAge("")` devolvía `0`, de modo que el borrador intacto se interpretaba como «paciente de 0 años» y el validador exigía tutor legal | reporte del usuario + `git log -S` (el defecto viene del scaffolding de wi-10, no de wi-14) | `hasCompleteBirthDate()` distingue «edad desconocida» de «0 años»; `validatePatientRegistration()` ya no marca `isMinor` sin fecha válida y expone `ageKnown`; `isPristinePatientDraft()` suprime el banner, el «0 años» y la afirmación de menor de edad mientras el formulario está intacto. La validación real del backend no se toca |

### 3.1 Defecto 14 — verificación end-to-end del banner falso

El fix se validó contra el portal real, con Chromium y el login completo (correo + MFA), comprobando **las dos** rutas:

| Comprobación | Resultado |
| :--- | :--- |
| `/patients` sin banner «Error de validación» ni código `LEGAL_GUARDIAN_REQUIRED_FOR_MINOR` | PASS |
| `/patients` sin «Edad calculada: 0 años»; muestra «Edad calculada: —» | PASS |
| `/patients` no afirma «Paciente menor de 18 años»; la leyenda queda condicional | PASS |
| El bloque «Tutor Legal Obligatorio» sigue disponible para el caso menor | PASS |
| El alta real de un menor sin tutor redirige a `/patients?error=LEGAL_GUARDIAN_REQUIRED_FOR_MINOR` | PASS |
| El banner **real** (`data-banner="intake-error"`) sí se muestra tras ese rechazo | PASS |

11/11 comprobaciones OK. Capturas: `/tmp/shot/06-pacientes-sin-banner-falso.png` y `/tmp/shot/07-error-real-menor-sin-tutor.png`.

Nota operativa: el servicio clínico **rechaza la reutilización del mismo TOTP dentro de la misma ventana de 30 s** (`auth.service.go`, anti-replay S2). Un script de verificación debe esperar a una ventana nueva antes de calcular el código.

---

## 4. Correcciones al plan aprobadas

| Plan original | realidad | dónde |
| :--- | :--- | :--- |
| S1: «declarar `pnpm.onlyBuiltDependencies` en `package.json`» | pnpm 12 movió el ajuste a `pnpm-workspace.yaml` y lo renombró al mapa `allowBuilds` | `pnpm-workspace.yaml` |
| S1: «`@tanstack/react-table`» | se fijó v8: la v9 tiene otra API y no es compatible con el bloque `data-table` de shadcn | `package.json` |
| S3: «`node:assert/strict` se conserva» | confirmado: sólo cambia el import de `describe`/`it` | las 18 suites |
| S4: «22 primitivos» | son 26 (el scaffold de `radix-nova` añade `input-group`, `skeleton` y `textarea`) | `src/components/ui/` |
| S5: «el shell consume la sesión del servidor» | se resolvió igual, pero el `TooltipProvider` tiene que vivir **dentro** del shell: el sidebar de shadcn lo exige | `portal-shell.tsx` |
| S7: «`session-idle-guard.ts` queda sólo con la lógica de inactividad» | además se creó `roles.ts`: la lista de roles estaba **duplicada en cuatro archivos** (`session-idle-guard`, `auth-flow`, `gateway-server`, `session-cookie`) | `src/lib/roles.ts` |

---

## 5. Desviación de comportamiento que hay que revisar

**El `auditor` ya no ve "Pacientes" en la barra lateral.** La navegación anterior se lo mostraba, pero el gateway le responde `403 INSUFFICIENT_ROLE_PERMISSION` sobre `/api/v1/patients` (`rbac_policy.go`, "Unified Patient Demographic Record": el `auditor` cae en `return false`). Es decir, el enlace era un 403 disfrazado de opción de menú.

El portal ahora refleja la política que **realmente** se aplica. Si el negocio quiere que el auditor lea fichas clínicas, el cambio corresponde a la matriz RBAC del gateway, que es de `wi-15` (`S4`), no a la navegación. Queda anotado en `nav-config.ts` y en `pdd/wi-15-read-contract-listings/plan.md`.

---

## 6. Lo que queda fuera de este work item

- Los 14 renderers `render*Html` y sus 12 suites siguen vivos: los borra `wi-16` (big bang). `AppShell.tsx` (renderer del shell) queda **sin uso** desde este work item y se elimina en `wi-16`.
- Las rutas `/treatments`, `/consents`, `/cash-registers`, `/users` y `/audit` no existen todavía; `nav-config.ts` las declara pero `IMPLEMENTED_ROUTES` las oculta, así que **no hay enlaces muertos**.
- Las pantallas de lista de cada entidad (`wi-16`, `wi-17`) todavía no usan el `DataTable`: el núcleo está construido y probado, pero su primer consumidor real llega en `wi-16`.
- `pnpm-lock.yaml` quedó versionado; `Dockerfile` usa `pnpm install --frozen-lockfile`. La imagen del stack en marcha (`deploy/compose.yaml`) sigue con la imagen anterior: para ver el shell nuevo hay que reconstruir (`docker compose up -d --build`).

---

## 7. Comandos para reproducir la verificación

```bash
cd .worktrees/frontend/frontend
pnpm install --frozen-lockfile
pnpm test          # 167/167, EXIT=0
npx tsc --noEmit   # sin errores
pnpm build         # EXIT=0
pnpm lint          # 0 errores, EXIT=0
```

Verificación en navegador real (contenedor reconstruido, stack 7/7 healthy):

```bash
cd deploy && docker compose up -d --build frontend
cd /tmp/shot && node verify-banner.js   # 11/11 comprobaciones OK
```
