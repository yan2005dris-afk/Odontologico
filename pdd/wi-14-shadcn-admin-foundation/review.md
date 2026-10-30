# Review — `wi-14-shadcn-admin-foundation`

Revisión 4-lentes sobre el cambio de `wt/frontend` contra el checkpoint `b4e60cf`.
Lentes: `review-risk`, `review-resilience`, `review-readability`, `review-reliability`.
Alcance: 89 archivos, +13 326 / −268 líneas.
Estado: **aprobado con observaciones**. Las 4 observaciones abiertas están
anotadas en §5 y ninguna invalida el work item.

---

## 1. Lente `review-risk` — superficie de riesgo y cumplimiento normativo

### 1.1 El gateway y el `auditor` — hallazgo cerrado con desviación declarada

La navegación anterior ofrecía "Pacientes" al rol `auditor`, pero la matriz
RBAC del borde devuelve `false` para ese rol sobre `/api/v1/patients`
(`services/api-gateway/internal/ingress/domain/rbac_policy.go:115-123`). El
enlace era un 403 disfrazado de opción de menú.

`src/lib/nav-config.ts:15-19` documenta la decisión y fija la matriz por
rol; `src/lib/nav-config.test.ts` la fija con un test explícito
(*"does not offer the auditor the patient record, which the gateway rejects
with 403"*). La pregunta de fondo —si el `auditor` *debe* leer fichas clínicas—
es de política del gateway y queda asignada a `wi-15` (`S4`), que es el
work item dueño de la matriz RBAC.

**Riesgo residual: bajo.** La desviación está documentada, es visible en el
diff y es reversible con una línea.

### 1.2 La paleta global — hallazgo cerrado

El scaffold de `shadcn` proponía dos cosas que violaban `AGENTS.md §3` y se
rechazaron:

- **Webfont `Geist` de Google Fonts** (`next/font/google`): convertía una
  descarga de red en una dependencia dura del build de la imagen, y
  contradecía el contrato de "system/geometric sans". `src/app/layout.tsx`
  quedó restaurado sin import de fuentes; `--font-sans` sigue siendo la pila
  del sistema en `src/app/globals.css`.
- **Paleta neutra (acromática)**: el scaffold la derivó de `baseColor:
  neutral`, que rompe el contrato "cool off-white (`slate-50`)". Se reescribió
  `:root` y `.dark` **enteros** con los valores de la escala `slate`
  (`src/app/globals.css`), y `components.json` quedó con `baseColor: slate`.

`src/components/ui/primitives.test.ts` blinda las dos cosas: ningún
primitivo puede contener un color literal (`#`, `rgb()`, `oklch()`), ninguna
utilidad de paleta (`bg-slate-900`, `text-red-600`…) y todo token semántico
usado tiene que estar definido en `globals.css`.

### 1.3 El `CommandMenu` no indexa pacientes — hallazgo cerrado

La paleta global (⌘K) indexa **módulos y rutas**, nunca pacientes: un buscador
global con información clínica protegida sería accesible a todos los roles que
pueden abrirlo. `src/components/shell/command-menu.tsx` lo dice en su
cabecera y `portal-shell.test.tsx` lo fija con el test *"never indexes
patients in the global palette"*.

### 1.4 Import muerto y paquete en runtime — hallazgo cerrado

`@import "shadcn/tailwind.css"` apuntaba a un archivo **inexistente**: el
paquete `shadcn` (4.23.0) no contiene ningún `.css`. Era un import que no
resolvía y que además arrastraba el CLI de scaffolding a `dependencies`. Se
eliminó el import y el paquete se retiró de las dependencias
(`src/app/globals.css`, `package.json`).

### 1.5 El shell no expone datos clínicos — hallazgo cerrado

`src/components/shell/portal-shell.tsx` no recibe ni renderiza fichas,
importes ni datos clínicos: recibe `role` y `userName` del servidor y compone
la navegación, el breadcrumb, la paleta y el menú de perfil. El token de
sesión sigue siendo `httpOnly` y sólo lo usa el servidor.

---

## 2. Lente `review-resilience` — fallos, estados degradados y recuperación

### 2.1 El harness de tests no podía testear React — hallazgo cerrado

Las 18 suites importaban `describe`/`it` de `node:test`, que no registra con
Vitest ("No test suite found"), y el entorno era `node`, sin DOM. La
migración a `vitest run` + `jsdom` + Testing Library quedó en
`frontend/vitest.config.ts` y `frontend/src/test-setup.ts`.

Dos detalles load-bearing:

- `esbuild: { jsx: "automatic" }` es **obligatorio**: `tsconfig.json` mantiene
  `"jsx": "preserve"` porque es lo que espera el SWC de Next.js, así que
  Vitest tiene que forzar la transformación.
- `resolve.alias` replica el alias `"@/*" → "./src/*"` de `tsconfig.json`,
  que Next sí aplica y Vitest no.

### 2.2 APIs del navegador ausentes en jsdom — hallazgo cerrado

Los primitivos de Radix (`Popover`, `DropdownMenu`, `Select`, `Tooltip`) usan
`ResizeObserver`, que jsdom no implementa: sin el stub, abrir un popover en
un test lanzaba `ResizeObserver is not defined`. `src/test-setup.ts` instala
`ResizeObserver`, `hasPointerCapture`, `setPointerCapture`,
`releasePointerCapture`, `scrollIntoView` y `matchMedia`.

### 2.3 Error de bucle en la sincronización con la URL — hallazgo cerrado

El efecto que escribe el estado de la tabla en la query string dependía de
`searchParams`, que cambia después de cada `router.replace`: el componente
podía reemplazarse a sí mismo indefinidamente. `src/components/data-table/data-table.tsx:177`
compara el destino con la URL actual antes de reemplazar, y
`data-table.test.tsx` lo fija con *"does not rewrite the URL when the state did
not change"*.

### 2.4 Filtrado multi-selección — hallazgo cerrado

Un facet guarda un **array**, pero el filtro de serie `equalsString` espera un
string: `filterValue.toLowerCase is not a function`.
`src/components/data-table/column-helpers.ts:27` define
`createMultiSelectFilter<TData>()` y `data-table.tsx:130` lo instala en toda
columna con facet declarado, de modo que una pantalla declara el facet y no
repite la semántica del filtro.

### 2.5 Estado vacío y de error — sin hallazgo

La `DataTable` declara un estado vacío explícito (*"No hay registros para
mostrar."*) y la paginación muestra el rango real. Ninguna pantalla fabrica
cifras ni listas.

---

## 3. Lente `review-readability` — claridad del código y del contrato

### 3.1 La lista de roles estaba duplicada en cuatro archivos — hallazgo cerrado

`USER_ROLES` estaba copiado en `session-idle-guard.ts`, `auth-flow.ts`,
`gateway-server.ts` y `session-cookie.ts` (este último con la validación
escrita a mano). Un rol añadido a un archivo podía desaparecer en otro.
Ahora hay **una** fuente: `src/lib/roles.ts:20` (`USER_ROLES`), con
`UserRole` derivado por tipo e `isUserRole` como único guard.

### 3.2 La navegación no tenía dueña — hallazgo cerrado

`PRIMARY_NAVIGATION_MODULES` y `getAllowedNavigationModules` vivían dentro
del módulo de inactividad de sesión: dos responsabilidades en un archivo.
`src/lib/nav-config.ts` es ahora la fuente única de la estructura de menús
(grupos, ítems, iconos, roles) y `session-idle-guard.ts` conserva sólo la
lógica de inactividad.

### 3.3 Los iconos como nombres, no como componentes — decisión, sin hallazgo

`nav-config.ts` declara los iconos como nombres (`"layout-dashboard"`) y
`src/components/shell/nav-icon.tsx` los traduce a los glifos de lucide. Así el
árbol de navegación sigue siendo un módulo `.ts` plano: lo importan los
server components y se testea sin React.

### 3.4 El `nav-config.ts` documenta las decisiones — sin hallazgo

Cada entrada del árbol lleva el motivo de sus roles, y el módulo explica en
su cabecera por qué la matriz refleja al gateway y no la aspiración del
producto (`nav-config.ts:12-23`).

---

## 4. Lente `review-reliability` — reproducibilidad, build y cadena de herramientas

### 4.1 No había lockfile — hallazgo cerrado

El repositorio no tenía `pnpm-lock.yaml`: la imagen del contenedor resolvía
rangos semver en cada build. Ahora el lockfile está versionado y el
`Dockerfile` usa `pnpm install --frozen-lockfile` con `corepack` fijando
pnpm 12.16.1.

### 4.2 pnpm ≥ 12 abortaba la instalación — hallazgo cerrado

`ERR_PNPM_IGNORED_BUILDS` detenía `pnpm install`. El ajuste ya no vive en el
campo `pnpm` del `package.json` (pnpm 12 lo ignora) sino en
`frontend/pnpm-workspace.yaml`, con el mapa `allowBuilds` (`esbuild`,
`sharp`, `@tailwindcss/oxide`, `unrs-resolver`).

### 4.3 `pnpm lint` no lintaba — hallazgo cerrado

`next lint` está deprecado en Next 15 y, sin configuración en el proyecto,
**preguntaba** en vez de lintar: el comando canónico de `AGENTS.md §5` fallaba
con una pregunta interactiva. Ahora hay `eslint.config.mjs` (flat config),
`eslint@^9` + `eslint-config-next@^15` —versiones coherentes con Next
15.2— y el script es `eslint .`.

> Nota: `pnpm add -D eslint eslint-config-next` sin versión resolvió
> `eslint-config-next@16.4.3`, que corresponde a **Next 16** y es incompatible
> con este proyecto. Se fijó `^15`.

### 4.4 Next infería mal la raíz del workspace — hallazgo cerrado

Next detectaba un `package-lock.json` ajeno (`/home/yan2005dris-afk/`) y lo
usaba como raíz del workspace, con el aviso correspondiente. Como
`output: "standalone"` decide **qué copia** en la imagen final, una raíz
equivocida podía dejar archivos fuera. `next.config.ts:22` fija
`outputFileTracingRoot: process.cwd()`.

### 4.5 El landmark de navegación se había perdido — hallazgo cerrado

El `Sidebar` de shadcn renderiza un `<div>`: el `role="navigation"` que el
shell anterior exponía como `<aside role="navigation">` desaparecía, y con
él la posibilidad de saltar a la navegación con el lector de pantalla.
`src/components/shell/app-sidebar.tsx:56` añade un `<nav
aria-label="Navegación Principal SIGO-Dental">` propio, con `display:
contents` para no alterar el layout.

### 4.6 El `TooltipProvider` es una dependencia del shell — hallazgo cerrado

El CLI de shadcn lo advirtió al instalar `tooltip` y no lo complied: sin él,
**toda** la subárbol del portal fallaba en el servidor con
``Tooltip` must be used within `TooltipProvider``. Seúlvó dentro de
`PortalShell` (`portal-shell.tsx:35`) y no en el root layout, para que el
shell sea autocontenido y su test no dependa de la raíz de la app.

---

## 5. Observaciones abiertas (no bloquean el work item)

| # | Observación | Ubicación | Riesgo | Mitigación |
| :--- | :--- | :--- | :--- | :--- |
| 1 | El fix del `<Command>` wrapper vive en un archivo vendorizado: un `shadcn add --overwrite` lo revierte y la paleta vuelve a romperse | `src/components/ui/command.tsx:66` | medio | La nota en el propio archivo lo advierte; `portal-shell.test.tsx` falla si vuelve a pasar |
| 2 | `IMPLEMENTED_ROUTES` es una lista manual: si `wi-16`/`wi-17` crean una ruta y no la anotan, el módulo no aparece en la navegación | `src/lib/nav-config.ts:179` | bajo | `nav-config.test.ts` fija el árbol completo con `onlyImplementedRoutes: false`; la omisión se ve en el diff |
| 3 | `formatCents` depende del ICU de Node (`toLocaleString("es-EC")`) | `src/components/data-table/column-helpers.ts:52` | bajo | Node incluye ICU completo desde la v13; el test fija la salida exacta |
| 4 | La `DataTable` usa `useSearchParams()`, que exige una página dinámica o un `Suspense` | `src/components/data-table/data-table.tsx:56` | bajo | Las páginas del portal son dinámicas (leen la cookie de sesión); `wi-16` debe mantener esa condición al añadir rutas |

---

## 6. Verificación de salida

| Comando | Resultado |
| :--- | :--- |
| `pnpm test` | 23 archivos, **158/158** tests, `EXIT=0` |
| `npx tsc --noEmit` | sin errores |
| `pnpm build` | compila, 12/12 páginas, `EXIT=0` |
| `pnpm lint` | **0 errores**, 7 warnings preexistentes, `EXIT=0` |
| Login end-to-end contra el gateway en marcha | `login → 200`, `mfa/verify → 303` con cookie `httpOnly`, `GET /patients → 200` |
| HTML del portal con el shell nuevo | `data-sidebar="sidebar"`, `<header>`×2, `<main>`×2, `<nav>`×1, paleta e indicador de inactividad presentes, contenido clínico intacto |
| Errores en el log del servidor | **0** |

---

## 7. Veredicto

**Aprobado.** Las cuatro lentes no dejan hallazgos abiertos de severidad alta
ni media sin corregir. Las cuatro observaciones de §5 están mitigadas por
tests y son el principal riesgo que hereda `wi-16`.
