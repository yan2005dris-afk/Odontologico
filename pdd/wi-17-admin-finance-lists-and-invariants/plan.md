# Plan de Work Item — `wi-17-admin-finance-lists-and-invariants`

Feature: `F08` (`08-frontend-shadcn-admin-lists`) — pantallas de lista de finanzas y administración, dashboard, verificación de invariantes y cierre documental.
Worktrees: `wt/frontend` (`.worktrees/frontend`) + `wt/gateway` (`.worktrees/gateway`, sólo para el OpenAPI) + `deploy/`.
Rol activo: Planner → Implementer. `MaxCorrections`: `200`.
Estado: `plan_pending_approval`.
Base: commit `b4e60cf` (checkpoint pre-refactor). Spec: [`pdd/tasks/08-frontend-shadcn-admin-lists.md`](../tasks/08-frontend-shadcn-admin-lists.md) (`S3`, `S9`..`S12`).

---

## 1. Descripción y alcance

`wi-16` deja el portal con React real y las pantallas clínicas funcionando. Este work item cierra el círculo: construye las **pantallas de lista financieras y de administración**, el **dashboard**, y ejecuta la **verificación explícita de las invariantes no funcionales** que `AGENTS.md §4` declara no negociables y que hasta ahora nadie ha medido.

Estado verificado que este work item toma como punto de partida:

- Tres pantallas muestran "Endpoint de lectura pendiente" (`billing/page.tsx:35`, `settlements/page.tsx:35`, `inventory/page.tsx:35`), con el modelo compartido en `portal-views.ts:304`.
- El cierre de caja ciego **ya tiene** endpoint (`GET /api/v1/billing/cash-registers/{id}`, `GET .../audit-report`, `billing_handler.go:23,25`) pero **no tiene pantalla**: no existe ruta `/cash-registers`.
- La bitácora de auditoría **ya tiene** endpoint (`GET /api/v1/audit/logs`, `audit_handler.go:24`) pero **no tiene pantalla**: no existe ruta `/audit`.
- No existe `GET /api/v1/users` (`wi-15` lo añade), ni ruta `/users`.
- `GET /openapi.json` publica rutas ficticias (`proxy_router.go:255-280`) y `buildOpenAPIJSONFromYAML` ignora el YAML (`proxy_router.go:282-288`).

### Dentro del alcance (In Scope)

- **S1.** `/billing` como lista de comprobantes (`DataTable`) + `PosCheckoutSheet` como acción de cobro.
- **S2.** `/cash-registers` (ruta nueva) como lista de aperturas y cierres + `BlindCashCloseSheet` con la garantía de `RN-02`.
- **S3.** `/settlements` como lista de liquidaciones a profesionales + `Sheet` de reglas de comisión.
- **S4.** `/inventory` como lista de insumos clínicos con alerta de stock bajo + `Sheet` de alta/reposición y de receta por procedimiento.
- **S5.** `/users` (ruta nueva) como lista de personal con rol RBAC y estado de MFA.
- **S6.** `/audit` (ruta nueva) como bitácora inmutable con filtros por actor, acción, recurso y rango de fechas.
- **S7.** `/` como dashboard operativo.
- **S8.** Verificación de las invariantes `RN-01`, `RN-02`, `RN-03`, `RNF-01`, `RNF-06` y `RNF-07`, con prueba o medición automatizada de cada una.
- **S9.** Cierre documental: OpenAPI real served por el gateway, `pdd/board.md` con F08 y sus 4 work items cerrados, `pdd/context/project-context.md` actualizado, y `deploy/compose.yaml` verificado con el portal migrado.

### Fuera del alcance (Out of Scope)

- Persistencia real en PostgreSQL: los repos siguen siendo en memoria. Es una limitación conocida y **documentada** en el entregable, no un defecto introducido aquí.
- Facturación electrónica con el SRI, SMS/email de recordatorios (`RF-11`) e integraciones externas de `SRS §7.1`.
- Exportaciones a Excel/PDF.
- Multi-idioma (el portal es en español; los datos del dominio no llevan i18n).
- Interactividad clínica profunda del odontograma 2D (clic en pieza → formulario), si `wi-16` la dejó como pendiente.

---

## 2. Dependencias previas

- **`wi-14`** (fundación), **`wi-15`** (contrato de lectura), **`wi-16`** (migración a React) — los tres merged.
- **Checkpoint `b4e60cf`**: línea base verde verificada.
- **Contenedores**: `postgres`, `redis`, `rustfs` disponibles según `deploy/compose.yaml`; el portal migrado debe levantar con `docker compose up -d --build` y los servicios quedan `Up (healthy)`.

---

## 3. Specs numeradas

- **S1 — `/billing`.** `DataTable` de comprobantes: paciente, fecha de emisión, método de pago, total (`tabular-nums`), estado fiscal. Filtros facetados por estado fiscal y método de pago; búsqueda por paciente. Acción primaria "+ Emitir Comprobante" → `PosCheckoutSheet` con pago mixto y autorización de la pasarela de pago (`PaymentGatewayPort`).
- **S2 — `/cash-registers` con `RN-02`.** Ruta nueva con `DataTable` de cajas: sede, apertura, cierre, estado, declarado y diferencia. Filtro por estado. La acción de cierre abre `BlindCashCloseSheet`, que pide **sólo** el conteo físico de la receptora: nunca pinta el total teórico del sistema antes de enviar el arqueo, y la caja `OPEN` ni siquiera recibe ese campo desde el servidor (`wi-15`, `S6`). El total teórico aparece recién después de cerrar, en la vista de auditoría de la caja.
- **S3 — `/settlements`.** `DataTable` de liquidaciones: profesional, período, procedimientos, bruto, comisión y neto (los tres importes con `tabular-nums`). Acción para generar liquidación y `Sheet` de configuración de reglas de comisión por profesional y procedimiento. Sólo `dentist` y `auditor` ven esta pantalla (`wi-15`, `S4`); el resto ve un estado de acceso denegado, no un 404.
- **S4 — `/inventory`.** `DataTable` de insumos: SKU, nombre, stock actual, stock mínimo, consultorio y estado de alerta (`low_stock` cuando `current_stock <= min_stock`). La alerta usa icono + texto, no sólo color. `Sheet` de alta/reposición y de receta estándar por procedimiento. El descuento automático por procedimiento completado (`RF-14`) se conserva intacto.
- **S5 — `/users`.** Ruta nueva con `DataTable` del personal: nombre, correo, rol RBAC, estado MFA y estado de la cuenta. Sólo `admin` la ve. La tabla **no** muestra el hash ni ningún secreto; MFA se representa como estado booleano.
- **S6 — `/audit`.** Ruta nueva con `DataTable` de la bitácora inmutable: timestamp ISO-8601, actor, rol, acción, recurso e IP de origen — los seis campos que exige `RNF-06`. Filtros por actor, acción, tipo de recurso y rango de fechas. Sólo `auditor` y `admin` acceden. La tabla es **de sólo lectura**: no hay acción de edición ni de borrado, y el backend no expone ninguno (`audit_repository.go` lanza `ErrAuditLogImmutable`).
- **S7 — Dashboard `/`.** Tarjetas de métricas operativas reales: citas del día, pacientes en sala de espera, alertas de stock crítico y presupuestos pendientes de aprobación. Cada tarjeta es un enlace a su módulo. La lista de próximos turnos por sillón reutiliza el `DataTable`. Las cifras provienen de los `GET` reales: si un módulo no tiene contrato, la tarjeta se declara como no disponible en lugar de mostrar `0` (que sería un dato falso).
- **S8 — Verificación de invariantes.** Cada invariante de `AGENTS.md §4` queda con una comprobación automatizada o una medición reproducible:
  - `RN-01` — una nota SOEP y un hallazgo de odontograma fuera de la ventana de 24 h no pueden modificarse ni borrarse; el backend responde `409`/`ErrClinicalRecordImmutable` y la UI ofrece la vía del addendum.
  - `RN-02` — el listado de cajas `OPEN` no contiene `theoretical_cents`; la pantalla de cierre ciego no lo muestra antes del arqueo; el total teórico sólo aparece tras el cierre.
  - `RN-03` — dos citas solapadas en el mismo sillón se rechazan con `409 CHAIR_OVERLAP_CONFLICT`, incluso con dentistas distintos.
  - `RNF-01` — carga del expediente + render del odontograma por debajo de 1,5 s con ≥ 15 Mbps; se mide y se registra el número.
  - `RNF-06` — cada acción CRUD sobre un registro clínico deja entrada de bitácora con los cinco campos; se comprueba con un caso de extremo a extremo por contexto.
  - `RNF-07` — el portal es usable en 1920×1080, 1366×768 y tablet ≥ 10,5"; las tablas hacen scroll horizontal y la barra lateral colapsa a íconos sin romper el lienzo clínico.
- **S9 — Cierre documental y de despliegue.** `GET /openapi.json` sirve el OpenAPI real, sincronizado con las rutas que los tres servicios exponen (paridad con `wi-15`, `S5`, revalidada aquí porque el frontend es el consumidor final). `pdd/board.md` refleja F08 y el estado cerrado de `wi-14`..`wi-17`. `pdd/context/project-context.md` refleja la arquitectura final (App Shell `shadcn-admin`, RSC + islas cliente, `DataTable`). `deploy/compose.yaml` levanta el portal migrado y los 7 servicios quedan `Up (healthy)`.

---

## 4. Contrato de entrada

- **Endpoints consumidos**: `GET /api/v1/billing/invoices` · `GET /api/v1/billing/cash-registers` · `GET /api/v1/billing/cash-registers/{id}` · `GET /api/v1/billing/cash-registers/{id}/audit-report` · `POST /api/v1/billing/cash-registers` · `POST /api/v1/billing/cash-registers/{id}/blind-close` · `POST /api/v1/billing/invoices` · `GET /api/v1/settlements` · `GET /api/v1/settlements/rules` · `POST /api/v1/settlements/rules` · `POST /api/v1/settlements/generate` · `GET /api/v1/inventory/items` · `GET /api/v1/inventory/recipes` · `POST /api/v1/inventory/items` · `PUT /api/v1/inventory/recipes/{procedureCode}` · `GET /api/v1/users` · `GET /api/v1/audit/logs` · `GET /api/v1/scheduling/appointments` · `GET /api/v1/patients` · `GET /api/v1/treatment-plans`.
- **Variables**: `GATEWAY_INTERNAL_URL`, `SESSION_COOKIE_NAME`, `SESSION_COOKIE_SECURE`.
- **Tipos**: los DTO de `src/lib/api-types.ts` creados en `wi-16`, ampliados aquí con los de listados financieros y de administración.
- **Restricción**: toda cifra en pantalla proviene de un `GET` real. Ningún total, saldo ni stock se calcula en el cliente a partir de datos parciales.

---

## 5. Contrato de salida

- `pnpm test`, `pnpm build` y `pnpm lint` verdes.
- Las 6 pantallas nuevas o rehechas (`/billing`, `/cash-registers`, `/settlements`, `/inventory`, `/users`, `/audit`) y el dashboard muestran datos reales o un estado vacío honesto; ninguna dice "Endpoint de lectura pendiente".
- El texto "Endpoint de lectura pendiente" desaparece de `frontend/src`.
- `docker compose up -d --build` deja los 7 servicios `Up (healthy)` con el portal migrado.
- El informe de verificación de invariantes queda en `pdd/wi-17-admin-finance-lists-and-invariants/apply.md` con la medición concreta de `RNF-01` y el resultado de cada comprobación de `RN-01`, `RN-02`, `RN-03`, `RNF-06` y `RNF-07`.
- `pdd/board.md` y `pdd/context/project-context.md` al día.

---

## 6. Seguridad y resiliencia

- **`RN-02` (cierre ciego)** — el punto de control más sensible del portal. El sistema no revela el total teórico de la caja antes del arqueo físico. Se verifica en dos capas: el contrato de datos (`wi-15`, `S6`, la caja `OPEN` no trae el campo) y la vista (`BlindCashCloseSheet` no lo pinta). Un `receptionist` no tiene ninguna ruta por la que obtenerlo; un `auditor` sí lo ve, pero sólo de cajas ya cerradas.
- **RBAC `SRS §3`** — `/users` es de `admin`; `/audit` de `auditor` y `admin`; `/settlements` de `dentist` y `auditor`; `/billing` de `receptionist` y `auditor` (lectura); `/inventory` de `assistant`, `dentist` y `auditor` (lectura). El backend aplica la autorización; la UI refleja el permiso, no lo sustituye.
- **PII y datos financieros** — la bitácora expone actor, rol, acción, recurso e IP: no expone el contenido clínico ni importes. La tabla de usuarios no expone hashes ni secretos.
- **Superficie de la bitácora** — la pantalla `/audit` no ofrece edición ni borrado, y el backend rechaza el borrado con `ErrAuditLogImmutable`.
- **`RNF-01` (rendimiento)** — las listas del portal cargan en el servidor y entregan el conjunto ya filtrado; el `DataTable` filtra y pagina en cliente. Se evita el patrón N+1 de llamadas por fila.
- **Resiliencia** — un módulo sin contrato o con error se declara como no disponible con reintento; nunca se fabrica un `0` ni una lista vacía presentada como "sin datos" cuando en realidad hubo un fallo.
- **Sin fuga de tokens** — el bundle del navegador no contiene el token de sesión; las islands cliente reciben sólo props serializables.

---

## 7. Archivos afectados

**Frontend — `wt/frontend`**
- `frontend/src/app/(portal)/billing/page.tsx` — `DataTable` + `PosCheckoutSheet`
- `frontend/src/app/(portal)/cash-registers/page.tsx` (nueva) — `DataTable` + `BlindCashCloseSheet`
- `frontend/src/app/(portal)/settlements/page.tsx` — `DataTable` + `Sheet` de reglas
- `frontend/src/app/(portal)/inventory/page.tsx` — `DataTable` + `Sheet` de insumo y receta
- `frontend/src/app/(portal)/users/page.tsx` (nueva) — `DataTable` de personal
- `frontend/src/app/(portal)/audit/page.tsx` (nueva) — `DataTable` de bitácora
- `frontend/src/app/(portal)/page.tsx` — dashboard
- `frontend/src/components/billing/*`, `components/inventory/*`, `components/admin/*` (nuevo) — hojas y columnas de tabla
- `frontend/src/lib/api-types.ts` — DTO de finanzas y administración
- `frontend/src/lib/nav-config.ts` — alta de los grupos "Caja & Operaciones" y "Administración & Seguridad"
- Tests nuevos por pantalla y por invariante

**Gateway — `wt/gateway`**
- `services/api-gateway/internal/ingress/adapters/http/proxy_router.go` — OpenAPI real (revalidación de `wi-15`, `S5`)

**Despliegue y PDD**
- `deploy/compose.yaml` (verificación, sin cambios de contrato salvo los estrictamente necesarios)
- `pdd/board.md`, `pdd/context/project-context.md`, `pdd/tasks/08-frontend-shadcn-admin-lists.md` (estado → `implemented`)
- `pdd/wi-17-admin-finance-lists-and-invariants/apply.md`, `review.md`

---

## 8. Orden de ejecución (TDD)

| Task | Specs | Archivos | Prueba RED |
| :--- | :--- | :--- | :--- |
| T1 | S1 | `billing/page.tsx` + suite | la lista de comprobantes hoy dice "Endpoint de lectura pendiente" |
| T2 | S2 | `cash-registers/page.tsx`, `BlindCashCloseSheet` + suite | la ruta no existe; la caja `OPEN` no expone total teórico ni en datos ni en pantalla |
| T3 | S3 | `settlements/page.tsx` + suite | listado con bruto/comisión/neto y `tabular-nums`; reglas de comisión por profesional |
| T4 | S4 | `inventory/page.tsx` + suite | alerta de stock bajo con icono y texto; receta por procedimiento |
| T5 | S5 | `users/page.tsx` + suite | la ruta no existe; la tabla no expone hash ni secretos |
| T6 | S6 | `audit/page.tsx` + suite | la ruta no existe; los 6 campos de `RNF-06` visibles; sin acción de borrado |
| T7 | S7 | `(portal)/page.tsx` + suite | las métricas provienen de `GET` reales; sin contrato ⇒ "no disponible", nunca `0` |
| T8 | S8 | suites de invariantes | `RN-01`: nota fuera de 24 h bloqueada; `RN-03`: solapamiento en el mismo sillón rechazado con dentistas distintos |
| T9 | S8 | medición de `RNF-01` y `RNF-07` | se registra el número medido de carga del expediente y del render del odontograma |
| T10 | S9 | `proxy_router.go` + test | `GET /openapi.json` sin rutas ficticias y con todas las reales |
| T11 | S9 | `deploy/compose.yaml` | `docker compose up -d --build` deja 7 servicios `Up (healthy)` |
| T12 | S1..S9 | todo | `pnpm test` + `pnpm build` + `pnpm lint` verdes; sin "Endpoint de lectura pendiente" en `frontend/src` |

**Criterio de salida**: T12 verde, `review` de 4 lentes en `pdd/wi-17-admin-finance-lists-and-invariants/review.md` con citas `file:line`, `apply.md` con la medición de `RNF-01` y el resultado de cada comprobación de invariantes, y el board actualizado con F08 cerrado.
