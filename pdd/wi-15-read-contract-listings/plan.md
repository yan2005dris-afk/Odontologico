# Plan de Work Item — `wi-15-read-contract-listings`

Feature: `F08` (`08-frontend-shadcn-admin-lists`) — `S2` del spec: contrato de lectura.
Worktrees: `wt/clinical-agenda`, `wt/billing-stock`, `wt/gateway` (`.worktrees/<svc>`).
Rol activo: Planner → Implementer. `MaxCorrections`: `200`.
Estado: `plan_pending_approval`.
Base: commit `b4e60cf` (checkpoint pre-refactor). Spec: [`pdd/tasks/08-frontend-shadcn-admin-lists.md`](../tasks/08-frontend-shadcn-admin-lists.md) (`S2`).

---

## 1. Descripción y alcance

Verificación de los endpoints reales (no de la documentación):

| Lectura | Estado real en el código | Evidencia |
| :--- | :--- | :--- |
| `GET /api/v1/patients` | ✅ existe | `clinical-agenda/internal/patients/adapters/http/patient_handler.go:24` |
| `GET /api/v1/audit/logs` | ✅ existe | `clinical-agenda/internal/audit/adapters/http/audit_handler.go:24` |
| `GET /api/v1/scheduling/appointments` | ✅ existe | `clinical-agenda/internal/scheduling/adapters/http/scheduling_handler.go:24` |
| `GET /api/v1/patients/{id}/consents` | ⚠️ el repo ya lista (`consent.port.go:12`), **sin handler HTTP** | `clinical-agenda/internal/clinical/adapters/http/soep_consent_handler.go:30-31` sólo expone `POST` |
| `GET /api/v1/treatment-plans` | ❌ | `treatment_handler.go:22-25` sólo expone `POST` |
| `GET /api/v1/users` | ❌ | `auth_handler.go:22-25` sólo expone login/MFA/heartbeat/logout |
| `GET /api/v1/billing/invoices` | ❌ | `billing_handler.go:22-26` sólo expone `POST` |
| `GET /api/v1/billing/cash-registers` | ❌ (sólo `{id}`) | `billing_handler.go:23` |
| `GET /api/v1/settlements`, `/rules` | ❌ | `settlement_handler.go:22-23` |
| `GET /api/v1/inventory/items`, `/recipes` | ❌ | `inventory_handler.go:22-24` |

Además, este work item corrige **tres defectos del gateway** que hoy hacen que parte del SRS sea inalcanzable:

1. **Ruteo roto (`RF-08` inalcanzable).** `ResolveUpstream` enruta por el prefijo `/api/v1/treatments` (`gateway.service.go:112`), pero el servicio registra `/api/v1/treatment-plans` (`treatment_handler.go:23-25`). `POST /api/v1/treatment-plans/{id}/approve`, `.../items/{itemId}/complete` y `DELETE .../items/{itemId}` caen en `UpstreamUnknown` → **404 `ROUTE_NOT_FOUND`** (`proxy_router.go:174-177`). La aprobación de presupuestos no existe de punta a punta.
2. **MFA evadido.** `RequiresMFA` cubre `/api/v1/treatments` (`rbac_policy.go:68`), no `/api/v1/treatment-plans`: si se arreglara sólo el ruteo, la aprobación de presupuestos quedaría fuera de la exigencia de MFA de `RNF-05`.
3. **OpenAPI ficticio.** `defaultOpenAPIJSON` (`proxy_router.go:255-280`) publica rutas que **no existen** en los servicios (`/api/v1/clinical/odontograms`, `/api/v1/treatments/plans`, `/api/v1/settlements/commissions`, `/api/v1/inventory/supplies`) y omite las que sí existen. Además `buildOpenAPIJSONFromYAML` (`proxy_router.go:282-288`) ignora el YAML y devuelve siempre el default.

### Dentro del alcance (In Scope)

- **S1.** `clinical-agenda`: 5 endpoints `GET` de listado (usuarios, planes de tratamiento globales y por paciente, consentimientos globales y por paciente).
- **S2.** `billing-stock`: 6 endpoints `GET` de listado (comprobantes, cajas, liquidaciones, reglas de comisión, insumos, recetas).
- **S3.** Puertos de repositorio + métodos de lectura nuevos, con paginación `limit`/`offset`, filtro `q` y filtros por dominio, reutilizando la convención de respuesta `{items, total}` de `handleSearchPatients` (`patient_handler.go:94-97`).
- **S4.** Gateway: corrección del ruteo, de `RequiresMFA`, de la matriz RBAC y de `isKnownAPIRoute` para las rutas nuevas y para `/api/v1/treatment-plans`.
- **S5.** Gateway: `defaultOpenAPIJSON` regenerado desde las rutas **reales** de los tres servicios.
- **S6.** Traza de auditoría `RNF-06` en los listados de datos clínicos y financieros.

### Fuera del alcance (Out of Scope)

- Cambios en el frontend: este work item no toca `frontend/`. Los consumidores salen en `wi-16`.
- Persistencia real en PostgreSQL: los adaptadores `adapters/postgres/*` siguen siendo en memoria (`patients/adapters/postgres/patient_repository.go:20-26`); los listados nuevos leen de esas mismas estructuras en memoria.
- Paginación de servidor en el frontend: `wi-14` entrega paginación client-side; `wi-16` cableará estos `GET` a las `DataTable`.
- `SRS §7.1` integraciones externas (facturación electrónica del SRI, MinIO/RustFS, SMS/email): fuera de alcance.
- Corrección del `openapi/` (artefacto YAML) de la raíz: se regenera el JSON que expone el gateway; el YAML queda para `wi-17`.

---

## 2. Dependencias previas

- **`wi-01-gateway-ingress`** y **`wi-02-clinical-identity-audit`**: matriz RBAC y bitácora inmutable ya implementadas.
- **`wi-03`…`wi-09`**: los contextos de dominio y sus adaptadores `postgres` (en memoria) ya existen.
- **Checkpoint `b4e60cf`**: los tres servicios con `go test` + `go vet` limpios.
- **Worktrees** `wt/clinical-agenda`, `wt/billing-stock`, `wt/gateway` creados desde `b4e60cf`.

> **Nota de presupuesto de corrección.** Si la revisión de 4 lentes (`MaxCorrections = 200`) agota el presupuesto, este work item se subdivide en `wi-15a` (clinical-agenda + gateway, que desbloquea las pantallas clínicas) y `wi-15b` (billing-stock). El orden de `S1`/`S2` ya está escrito para permitir esa corte sin reordenar trabajo.

---

## 3. Specs numeradas

- **S1 — `clinical-agenda`: 5 listados.**
  - `GET /api/v1/users` — personal de la clínica: `id`, `email`, `full_name`, `role`, `mfa_enabled`, `is_active`. Sólo rol `admin`.
  - `GET /api/v1/treatment-plans` — listados global con filtros `patient_id`, `status` (`DRAFT` | `PARTIALLY_APPROVED` | `APPROVED` | `COMPLETED`), `q`; devuelve `total_approved_cents` y `total_planned_cents` por plan.
  - `GET /api/v1/patients/{id}/treatment-plans` — planes de un paciente, con sus ítems (código de procedimiento, descripción, precio, estado de aprobación).
  - `GET /api/v1/consents` — listados global con filtros `patient_id`, `status` (`PENDING` | `SIGNED_BIOMETRIC` | `SIGNED_OTP`), `procedure_code`.
  - `GET /api/v1/patients/{id}/consents` — consentimientos de un paciente. El método de repositorio **ya existe** (`consent.port.go:12`); falta el handler HTTP y el método de servicio.
- **S2 — `billing-stock`: 6 listados.**
  - `GET /api/v1/billing/invoices` — comprobantes: `id`, `patient_id`, `issued_at`, `total_cents`, `fiscal_status`, `payment_method`. Filtros `patient_id`, `fiscal_status`, `from`, `to`.
  - `GET /api/v1/billing/cash-registers` — cajas abiertas y cerradas con `id`, `clinic_id`, `opened_at`, `closed_at`, `status`, `declared_cents`, `discrepancy_cents`. **`theoretical_cents` sólo se incluye si la caja está `CLOSED`** (ver `S6`).
  - `GET /api/v1/settlements` — liquidaciones a profesionales: `dentist_id`, `period_from`, `period_to`, `procedure_count`, `gross_cents`, `commission_cents`, `net_cents`.
  - `GET /api/v1/settlements/rules` — reglas de comisión por profesional y procedimiento.
  - `GET /api/v1/inventory/items` — insumos: `sku`, `name`, `current_stock`, `min_stock`, `clinic_id`, `unit`. Campo derivado `low_stock` (`current_stock <= min_stock`).
  - `GET /api/v1/inventory/recipes` — recetas estándar por procedimiento: `procedure_code`, `items[] {sku, quantity}`.
- **S3 — Puertos y repositorios.** Se añaden a cada puerto los métodos de lectura con la firma `List...(ctx context.Context, filter <X>Filter) ([]<domain.T>, int, error)`, devolviendo el total para la paginación: `UserRepository.ListUsers`, `TreatmentRepository.ListPlans`, `ConsentRepository.ListConsents`, `BillingRepositoryPort.ListInvoices` + `ListCashRegisters`, `SettlementRepositoryPort.ListSettlements` + `ListCommissionRules`, `InventoryRepositoryPort.ListSupplies` + `ListRecipes`. Los adaptadores en memoria los implementan con orden determinista (por `created_at` descendente, y por `id` como desempate) y `limit`/`offset` por defecto `limit=50`, `offset=0`, con techo de `limit=200`.
- **S4 — Gateway: ruteo, MFA y RBAC.**
  - `ResolveUpstream` (`gateway.service.go:106`) añade los prefijos `/api/v1/treatment-plans`, `/api/v1/consents` y `/api/v1/users` al grupo `clinical-agenda`.
  - `RequiresMFA` (`rbac_policy.go:62`) exige MFA para `/api/v1/treatment-plans` (paridad con `/api/v1/treatments`) y para la **firma** de consentimiento (`POST /api/v1/patients/{id}/consents/{consentId}/sign`). Los `GET` de listado no exigen MFA adicional más allá de la sesión vigente.
  - `RBACPolicy.IsAllowed` (`rbac_policy.go:88`) incorpora reglas para las tres rutas nuevas: `/api/v1/users` → sólo `admin`; `/api/v1/treatment-plans` → `dentist` pleno, `receptionist`/`assistant` sólo lectura (paridad con `/api/v1/treatments`); `/api/v1/consents` → `dentist` y `receptionist` pleno, `assistant` sólo lectura, `auditor` sin acceso (paridad con los datos clínicos de `/api/v1/patients`).
  - `isKnownAPIRoute` (`rbac_policy.go:184`) incluye los tres prefijos nuevos para que `admin` no quede bloqueado.
  - `normalizeRouteMetricLabel` (`proxy_router.go:230`) añade las etiquetas de métrica correspondientes, evitando cardinalidad de ruta sin agregar.
- **S5 — OpenAPI real.** `defaultOpenAPIJSON` (`proxy_router.go:255`) se regenera listando **sólo** rutas que los tres servicios exponen realmente, con el verbo correcto y su `operationId`. `buildOpenAPIJSONFromYAML` deja de ignorar el YAML: si el archivo existe y es OpenAPI 3.0 válido, se sirve tal cual; si no, se sirve el default.
- **S6 — `RN-02` y `RNF-06`.**
  - El listado de cajas **nunca** expone el total teórico mientras la caja esté `OPEN`: el campo se omite del DTO (no va en `null` ni en `0`). El total teórico sólo existe cuando `CashRegister.Status == CLOSED` (`billing/domain/cash_register.go:190,220`). Un `receptionist` no ve el monto teórico en ningún caso; un `auditor` ve el de cajas ya cerradas, que es su propósito de auditoría.
  - Cada listado de datos clínicos o financial deja traza inmutable `RNF-06` con `actor_id`, `action`, `resource_id`, `timestamp_iso8601` y `source_ip`. Las acciones disponibles son `CREATE`, `READ`, `UPDATE` y `EXPORT` (`audit/domain/audit_entry.go:20-23`): los listados que **extraen** datos en volumen usan `EXPORT`; la lectura de un registro puntual usa `READ`. La escritura de la traza no puede tumbar el listado: si el `Recorder` falla, el listado responde `200` y el error de auditoría queda registrado en el log del servicio (mismo patrón `_, _ = s.audit.Record(...)` ya usado en `patient.service.go:238`).

---

## 4. Contrato de entrada

- **Variables de entorno**: ninguna nueva.
- **Endpoints consumidos**: ninguno (este work item sólo expone).
- **Convención de respuesta**: `{ "items": [...], "total": <int> }`, igual que `handleSearchPatients` (`patient_handler.go:94-97`) y `handleListLogs` (`audit_handler.go`). `total` es el total **antes** de `limit`/`offset`.
- **Convención de query**: `limit`, `offset`, `q`, más filtros por dominio. `limit` por defecto 50, máximo 200.
- **Errores**: `writeErr`/`mapDomainError` existentes; `400 INVALID_JSON` en query inválida, `401`/`403` resueltos en el gateway.
- **Tipos**: se reusan los tipos de `domain` de cada contexto. No se definen formas nuevas en la capa HTTP: el handler mapea `domain` → DTO JSON.

---

## 5. Contrato de salida

- `go test ./...` y `go vet ./...` verdes en `clinical-agenda`, `billing-stock` y `api-gateway`.
- Los 11 endpoints nuevos responden `200` con `{items, total}` y registran traza de auditoría.
- `POST /api/v1/treatment-plans/{id}/approve` deja de responder 404 y llega al servicio: **es el criterio de aceptación de `RF-08`**.
- Un `GET /api/v1/billing/cash-registers` sobre una caja `OPEN` no contiene la clave `theoretical_cents` en ninguna respuesta.
- `GET /openapi.json` no lista ninguna ruta inexistente y lista las 11 nuevas.
- Un `dentist` recibe `403 INSUFFICIENT_ROLE` en `/api/v1/users`; un `admin` recibe `200`.

---

## 6. Seguridad y resiliencia

- **MFA `RNF-05`**: la aprobación de presupuestos y la firma de consentimiento exigen `mfa_verified=true`. El arreglo de `S4` cierra la laguna por la que `/api/v1/treatment-plans` se habría quedado sin MFA.
- **RBAC `SRS §3`**: las tres rutas nuevas tienen reglas explícitas; `auditor` no accede a `/api/v1/users` ni a `/api/v1/consents`.
- **`RN-02` (cierre ciego)**: el listado de cajas omite el total teórico mientras la caja está abierta. Este es el punto de control más sensible del work item y exige test de regresión explícito.
- **`RNF-06`**: todo listado deja traza con `source_ip` real, tomada del header `X-Source-IP` que inyecta el gateway (`proxy_router.go:165`).
- **Sin fuga por error**: un fallo del `Recorder` no expone el error interno al cliente ni invalida la respuesta.
- **Límite de recursos**: `limit` con techo duro de 200 evita que un cliente agote la memoria del servicio en memoria.
- **Orden determinista**: los listados ordenan por fecha descendente con desempate por `id`, de modo que la paginación no duplica ni omite registros entre páginas.

---

## 7. Archivos afectados

**`wt/clinical-agenda`**
- `internal/identity/adapters/http/auth_handler.go` (+ test) — `GET /api/v1/users`
- `internal/identity/application/auth.port.go`, `internal/identity/application/auth.service.go`, `internal/identity/adapters/postgres/user_repository.go` — `ListUsers`
- `internal/treatments/adapters/http/treatment_handler.go` (+ test) — `GET /api/v1/treatment-plans`, `GET /api/v1/patients/{id}/treatment-plans`
- `internal/treatments/application/treatment.port.go`, `.../treatment.service.go`, `internal/treatments/adapters/postgres/treatment_repository.go` — `ListPlans`
- `internal/clinical/adapters/http/soep_consent_handler.go` (+ test) — `GET /api/v1/consents`, `GET /api/v1/patients/{id}/consents`
- `internal/clinical/application/consent.port.go`, `.../consent.service.go`, `internal/clinical/adapters/postgres/consent_repository.go` — `ListConsents`

**`wt/billing-stock`**
- `internal/billing/adapters/http/billing_handler.go` (+ test) — `GET /api/v1/billing/invoices`, `GET /api/v1/billing/cash-registers`
- `internal/billing/application/billing.port.go`, `.../billing.service.go`, `internal/billing/adapters/postgres/billing_repository.go` — `ListInvoices`, `ListCashRegisters`
- `internal/settlements/adapters/http/settlement_handler.go` (+ test) — `GET /api/v1/settlements`, `GET /api/v1/settlements/rules`
- `internal/settlements/application/settlement.port.go`, `.../settlement.service.go`, `internal/settlements/adapters/postgres/settlement_repository.go` — `ListSettlements`, `ListCommissionRules`
- `internal/inventory/adapters/http/inventory_handler.go` (+ test) — `GET /api/v1/inventory/items`, `GET /api/v1/inventory/recipes`
- `internal/inventory/application/inventory.port.go`, `.../inventory.service.go`, `internal/inventory/adapters/postgres/inventory_repository.go` — `ListSupplies`, `ListRecipes`

**`wt/gateway`**
- `internal/ingress/application/gateway.service.go` (+ test) — `ResolveUpstream`
- `internal/ingress/domain/rbac_policy.go` (+ test) — `RequiresMFA`, `IsAllowed`, `isKnownAPIRoute`
- `internal/ingress/adapters/http/proxy_router.go` (+ test) — `normalizeRouteMetricLabel`, `defaultOpenAPIJSON`, `buildOpenAPIJSONFromYAML`

**PDD**: `pdd/wi-15-read-contract-listings/apply.md`, `review.md`.

---

## 8. Orden de ejecución (TDD)

| Task | Specs | Archivos | Prueba RED |
| :--- | :--- | :--- | :--- |
| T1 | S4 | `rbac_policy.go` + test | `/api/v1/treatment-plans/{id}/approve` con `mfa_verified=false` no debe pasar; `/api/v1/users` con rol no admin debe dar 403 |
| T2 | S4 | `gateway.service.go` + test | `ResolveUpstream("/api/v1/treatment-plans/x/approve")` devuelve `UpstreamUnknown` hoy |
| T3 | S3, S1 | identity repo + service + handler | `GET /api/v1/users` → 404 hoy; el test RED exige `{items,total}` |
| T4 | S3, S1 | treatments repo + service + handler | `GET /api/v1/treatment-plans` → 404 hoy |
| T5 | S3, S1 | consents repo + service + handler | `GET /api/v1/patients/{id}/consents` → 404 hoy (el repo ya lista) |
| T6 | S3, S2 | billing repo + service + handler | `GET /api/v1/billing/invoices` → 404 hoy |
| T7 | S6 | `billing_handler.go` + test | caja `OPEN`: la respuesta **no** contiene `theoretical_cents` |
| T8 | S3, S2 | settlements repo + service + handler | `GET /api/v1/settlements` → 404 hoy |
| T9 | S3, S2 | inventory repo + service + handler | `GET /api/v1/inventory/items` → 404 hoy |
| T10 | S6 | servicios que listan | cada listado deja traza `EXPORT` con `actor_id`, `source_ip` y timestamp ISO-8601 |
| T11 | S5 | `proxy_router.go` + test | `GET /openapi.json` no debe listar rutas inexistentes ni omitir las nuevas |
| T12 | S1..S6 | todo | `go test ./...` + `go vet ./...` verdes en los 3 servicios |

**Criterio de salida**: T12 verde, `review` de 4 lentes en `pdd/wi-15-read-contract-listings/review.md` con citas `file:line`, y `apply.md` con los comandos ejecutados y sus salidas.
