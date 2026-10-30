# Apply — `wi-15-read-contract-listings`

Work item: `wi-15` de la feature `F08` (`08-frontend-shadcn-admin-lists`), sección `S2` del spec.
Base: `c865d15`. Ramas: `wt/clinical-agenda`, `wt/billing-stock`, `wt/gateway` → mergeadas en `main`.
Rol activo: Implementer → Reviewer. `MaxCorrections`: `200` (consumidas ~55 líneas de cambio real).
Estado: **implementado y verificado**.

Alcance del cambio: `services/` únicamente (60 archivos, +5 242 / −147). **No toca `frontend/`**: los
consumidores de estos `GET` salen en `wi-16` y `wi-17`.

---

## 1. Qué se implementó

### 1.1 `clinical-agenda` — 19 archivos, +1 824 / −62

| Spec | Qué | Dónde |
| :--- | :--- | :--- |
| **S0** | Seed idempotente de los **5 roles RBAC canónicos** (`admin`, `receptionist`, `dentist`, `assistant`, `auditor`), con MFA activo y secretos TOTP leídos de variables de entorno con valor por defecto determinista | `cmd/api/main.go:66` (`canonicalSeedAccounts`), `cmd/api/main.go:117` (`seedCanonicalUsers`), `cmd/api/main_test.go` |
| **S1/S3** | `GET /api/v1/users` — directorio del personal, sólo `admin`, con filtro `q` y `role` | `internal/identity/application/auth.service.go:233`, `internal/identity/adapters/postgres/user_repository.go`, `internal/identity/adapters/http/auth_handler.go` |
| **S1/S3** | `GET /api/v1/treatment-plans` y `GET /api/v1/patients/{id}/treatment-plans` — con `total_approved_cents` y `total_planned_cents` por plan, y el detalle de ítems | `internal/treatments/application/treatment.service.go:200`, `internal/treatments/adapters/http/treatment_handler.go` |
| **S1/S3** | `GET /api/v1/consents` y `GET /api/v1/patients/{id}/consents` | `internal/clinical/application/consent.service.go:108` y `:132`, `internal/clinical/adapters/http/soep_consent_handler.go` |
| **S6** | Traza `EXPORT` inmutable en los 5 listados; un `Recorder` caído **no** invalida el `200` y queda registrado en el log | `internal/identity/application/auth.service.go` (`recordAudit`), `internal/treatments/application/treatment.service.go`, `internal/clinical/application/consent.service.go` |

La cuenta `dentist` (`dra.gomez@sigo-dental.com` / `ClinicalPass#2026` / TOTP `JBSWY3DPEHPK3PXP`)
conserva sus credenciales: el portal y `deploy/README.md` dependen de ellas. Las otras cuatro están
documentadas en `deploy/README.md`, con su generador de TOTP y la advertencia de que son
**credenciales sólo de desarrollo**.

### 1.2 `billing-stock` — 31 archivos, +2 365 / −25

| Spec | Qué | Dónde |
| :--- | :--- | :--- |
| **S2/S3** | `GET /api/v1/billing/invoices` y `/api/v1/billing/cash-registers` | `internal/billing/application/billing.service.go`, `internal/billing/adapters/http/billing_handler.go` |
| **S2/S3** | `GET /api/v1/settlements` y `/api/v1/settlements/rules` | `internal/settlements/application/settlement.service.go`, `internal/settlements/adapters/http/settlement_handler.go` |
| **S2/S3** | `GET /api/v1/inventory/items` (con `low_stock` derivado) y `/api/v1/inventory/recipes` | `internal/inventory/application/inventory.service.go`, `internal/inventory/adapters/http/inventory_handler.go` |
| **S6** | **RN-02**: el total teórico y la discrepancia sólo se publican para `auditor`/`admin` y sólo con la caja cerrada | `internal/billing/domain/cash_register.go:245` (`ToListItemView(canSeeReconciliation bool)`), `internal/billing/application/billing.service.go:360` |
| **S6** | Contexto de auditoría propio (el servicio no tenía sumidero de trazas) + `EXPORT` en los 6 listados | `internal/audit/{domain,application,adapters/postgres}/` |

### 1.3 `api-gateway` — 10 archivos, +1 053 / −60

| Spec | Qué | Dónde |
| :--- | :--- | :--- |
| **S4** | `ResolveUpstream` enruta `/api/v1/users`, `/api/v1/treatment-plans` y `/api/v1/consents` a `clinical-agenda` | `internal/ingress/application/gateway.service.go:110-115` |
| **S4** | `RequiresMFAForMethod(método, ruta)`: MFA en la aprobación de presupuestos y en la firma de consentimiento; los `GET` de listado no exigen MFA adicional | `internal/ingress/domain/rbac_policy.go:73` |
| **S4** | Matriz RBAC de las 3 rutas nuevas, con el solapamiento `/patients/{id}/{treatment-plans,consents}` resuelto **antes** de la política de fichas clínicas | `internal/ingress/domain/rbac_policy.go:166`, `:181`, `:192` |
| **S4** | `isKnownAPIRoute` incluye los 3 prefijos nuevos (para que `admin` no quede bloqueado) | `internal/ingress/domain/rbac_policy.go:271-279` |
| **S4** | Etiquetas de métrica de las rutas nuevas, sin cardinalidad por recurso | `internal/ingress/adapters/http/proxy_router.go:237-242` |
| **S5** | OpenAPI **real**: registro de las 49 rutas / 62 operaciones que los tres servicios exponen de verdad; `buildOpenAPIJSONFromYAML` ya no ignora el YAML | `internal/ingress/adapters/http/openapi_spec.go` |

---

## 2. Verificación

### 2.1 Comandos canónicos en `main`

```
cd services/clinical-agenda && go build ./... && go vet ./... && go test ./... -count=1
  → build OK · vet OK · 20 paquetes ok · 0 fallos · 40 funciones de test

cd services/billing-stock && go build ./... && go vet ./... && go test ./... -count=1
  → build OK · vet OK · 9 paquetes ok · 0 fallos · 25 funciones de test

cd services/api-gateway && go build ./... && go vet ./... && go test ./... -count=1
  → build OK · vet OK · 3 paquetes ok · 0 fallos · 21 funciones de test
```

86 funciones de test en total; los tres servicios compilan y pasan `go vet` sin una sola salida.

### 2.2 Verificación end-to-end contra el stack real

Stack reconstruido (`docker compose up -d --build clinical-agenda billing-stock api-gateway`),
7/7 healthy. Se condujo el login real de los **5 roles** (correo → MFA → `access_token`) y se
ejercitó el contrato por el gateway. **66/66 comprobaciones OK** en dos guiones.

**Guion 1 — contrato, paginación y matriz RBAC (51/51)**

| Comprobación | Resultado |
| :--- | :--- |
| Login con MFA de los 5 roles | PASS ×5 |
| Los 11 `GET` nuevos responden `200 {items,total}` | PASS ×11 |
| `limit=2` respeta la ventana; `limit=9999` respeta el techo de 200 | PASS ×2 |
| `admin` lista usuarios; `dentist`/`receptionist`/`auditor` reciben 403 | PASS ×4 |
| `dentist`/`receptionist`/`assistant` leen `consents`; `auditor` recibe 403 | PASS ×4 |
| `dentist`/`receptionist`/`assistant` leen `treatment-plans`; `auditor` recibe 403 | PASS ×4 |
| **`auditor` no lee fichas clínicas** (`/api/v1/patients` → 403) | PASS |
| `auditor` sí audita (`/audit/logs`), liquida (`/settlements`) e inventaría (`/inventory/items`) | PASS ×3 |
| `POST /api/v1/treatment-plans/{id}/approve` ya **no** es `ROUTE_NOT_FOUND` | PASS |
| `GET /openapi.json` declara las 11 rutas nuevas | PASS ×11 |
| `GET /openapi.json` **no** declara ninguna de las 4 rutas ficticias | PASS ×4 |

**Guion 2 — RN-02 con cajas reales (15/15)**

Se abre una caja con flotante 50 000, se lista como receptor y como auditor, se cierra con un
conteo declarado de **48 000** (es decir, con discrepancia) y se vuelve a listar:

| Comprobación | Resultado |
| :--- | :--- |
| Caja OPEN: ni teórico ni discrepancia, para receptor **ni** auditor | PASS ×4 |
| Caja CLOSED: el receptor **no** ve el teórico | PASS |
| Caja CLOSED: el receptor **no** ve la discrepancia (si la viera, la restaría del conteo declarado y recuperaría el teórico) | PASS |
| Caja CLOSED: el receptor sí ve su propio conteo declarado (48 000) | PASS |
| Caja CLOSED: el auditor ve el teórico (50 000) y la discrepancia (−2 000) | PASS ×2 |
| `GET .../audit-report`: receptor 403, auditor 200 | PASS ×2 |

### 2.3 Traza de auditoría `RNF-06`

`GET /api/v1/audit/logs?action=EXPORT` devuelve las trazas de los listados clínicos con
`actor_id`, `actor_role`, `action=EXPORT`, `resource_type`/`resource_id`, `timestamp_iso8601`
(RFC 3339) y `source_ip`:

```
2026-10-30T13:04:31Z  admin         EXPORT  USER/users                 ip=172.19.0.1
2026-10-30T13:04:31Z  assistant     EXPORT  TREATMENT_PLAN/treatment-plans  ip=172.19.0.1
2026-10-30T13:04:31Z  receptionist  EXPORT  INFORMED_CONSENT/consents  ip=172.19.0.1
2026-10-30T13:04:31Z  dentist       EXPORT  TREATMENT_PLAN/treatment-plans  ip=172.19.0.1
```

19 trazas `EXPORT` más 8 `CREATE` en la bitácora del servicio clínico.

---

## 3. Defectos encontrados y corregidos

| # | Defecto | Origen | Corrección |
| :--- | :--- | :--- | :--- |
| 1 | **Ruteo roto: `RF-08` inalcanzable.** `ResolveUpstream` enruta por `/api/v1/treatments`, pero el servicio registra `/api/v1/treatment-plans`: la aprobación de presupuestos respondía 404 `ROUTE_NOT_FOUND` | `pdd/wi-15/plan.md` §1 (verificado en código) | los 3 prefijos nuevos en `gateway.service.go:110-115` + test end-to-end del proxy |
| 2 | **MFA evadido.** `RequiresMFA` cubría `/api/v1/treatments` pero no `/api/v1/treatment-plans`: arreglar sólo el ruteo habría dejado la aprobación de presupuestos fuera de `RNF-05` | `pdd/wi-15/plan.md` §1 | `RequiresMFAForMethod(método, ruta)` en `rbac_policy.go:73`: MFA en la aprobación y en la firma; los `GET` de listado no la exigen |
| 3 | **OpenAPI ficticio.** `defaultOpenAPIJSON` publicaba 4 rutas inexistencias y `buildOpenAPIJSONFromYAML` ignoraba el YAML | `pdd/wi-15/plan.md` §1 | `openapi_spec.go` con el registro real (49 rutas / 62 operaciones) + validación del YAML contra ese registro |
| 4 | **RN-02: el listado de cajas publicaba el total teórico y la discrepancia a cualquier rol con la caja cerrada.** El contrato ya establecido en el dominio (`GetAuditReport`) restringe la conciliación a `auditor`/`admin`, y el propio comentario de `CashRegisterAuditReportView` lo dice. Además la discrepancia es `declarado − teórico`, así que publicarla al receptor permitía recuperar el teórico **por resta** | revisión del orquestador sobre el diff de `billing-stock` | `ToListItemView(canSeeReconciliation bool)` (`cash_register.go:245`) + `canReconcile` en el servicio (`billing.service.go:360`); test de regresión por rol con discrepancia distinta de cero y verificación end-to-end con caja real |
| 5 | **`ListConsentsByPatient` huérfano.** El método de repositorio estaba declarado en el puerto `ConsentRepository` **sin un solo llamador** desde antes de `wi-15`, y quedó con el mismo nombre que el método de servicio, que tiene otra firma y otro contrato (`{items,total}` con `limit`/`offset`) | `git grep` sobre `c865d15` | retirado del puerto y del repositorio en memoria: una sola vía de lectura, un solo contrato |
| 6 | **`theoretical_cents` en la caja `OPEN` se comprobaba por el valor, no por la ausencia de la clave** | lectura del test heredado | el test afirma la **ausencia de la cadena** `theoretical_cents` en el JSON, no sólo que el valor no sea el esperado |

---

## 4. Desviaciones del plan

| Plan original | Realidad | Dónde |
| :--- | :--- | :--- |
| §7 no lista `internal/audit/` en `billing-stock` | El servicio **no tenía** contexto de auditoría y `S6` exige traza `RNF-06` en los listados financieros: se creó copiando el patrón de `clinical-agenda` (`AuditEntry` inmutable, `Recorder.Record`, repositorio append-only) | `services/billing-stock/internal/audit/` |
| `buildOpenAPIJSONFromYAML` sirve el YAML tal cual si es válido | El artefacto `openapi/sigo-dental.v1.yaml` describe 4 rutas que la plataforma **no expone**. Servirlo tal cual republicaría el contrato ficticio. Ahora el YAML se valida contra el registro real y, si describe rutas inexistentes, cae al contrato embebido. `openapi/**` está fuera del alcance de `wi-15` (queda para `wi-17`) | `openapi_spec.go` |
| `RequiresMFA(ruta)` | Se conserva la firma de 1 argumento (semántica de lectura) y se añade `RequiresMFAForMethod(método, ruta)`, que es la que evalúa el borde: sin ella, `GET /api/v1/patients/{id}/treatment-plans` y `POST` del mismo path no se pueden distinguir | `rbac_policy.go:64-73` |
| Los `GET` de listado no exigen MFA | Se mantiene la regla **preexistente** de la regla en blanco de `/api/v1/billing`: `GET /api/v1/billing/invoices` sigue exigiendo MFA. No se relajó una regla de seguridad existente para acomodar el listado | `rbac_policy.go:82-87` |
| `status` de planes: `DRAFT \| PARTIALLY_APPROVED \| APPROVED \| COMPLETED` | El **dominio** usa `DRAFT \| APPROVED_PARTIAL \| APPROVED_TOTAL \| COMPLETED`: el plan tenía el vocabulario equivocado. El servicio acepta el vocabulario público del plan **y** el del dominio, y los mapea al dominio | `treatment.service.go:328` (`normalizePlanStatus`) |
| `GET /api/v1/patients/{id}/consents` usa el método de repositorio que ya existía | El método existente no paginaba y no puede cumplir el contrato `{items,total}` con `limit`/`offset`. El endpoint usa el listado paginado con filtro por paciente; el método viejo se retiró (defecto 5) | `consent.service.go:132` |

---

## 5. Observaciones abiertas (no bloquean este work item)

1. **La bitácora de `billing-stock` no tiene endpoint de lectura.** Las trazas de los listados
   financieros se escriben en su repositorio de auditoría en memoria, pero no hay handler HTTP
   para leerlas: el `GET /api/v1/audit/logs` que expone el gateway es el de `clinical-agenda`.
   `RNF-06` pide la **escritura** de la traza, que existe; su lectura para el rol `auditor`
   queda para `wi-17`.
2. **El artefacto `openapi/sigo-dental.v1.yaml` sigue conteniendo las 4 rutas ficticias.** El
   gateway ya no las publica, pero el YAML es la fuente que `wi-17` debe regenerar.
3. **`/api/v1/patients/{id}/soep-notes` no exige MFA.** Es una regla preexistente y ajena a
   `wi-15` (que sólo nombra `treatment-plans` y la firma de consentimiento), pero las notas SOEP
   son historia clínica: conviene revisarlo al aplicar `RNF-05` de forma integral.
4. **Los repositorios siguen siendo en memoria.** Cada reinicio del contenedor borra pacientes,
   turnos, facturas y la bitácora, y vuelve a sembrar los 5 roles. Es el límite de persistencia
   ya conocido (`pdd/wi-15/plan.md` §S0) y no se introduce aquí.
5. **El `seed` corre en cada arranque** y las credenciales de desarrollo son públicas en
   `deploy/README.md`. Es aceptable en desarrollo, pero exige una guardia antes de producción.

---

## 6. Comandos para reproducir la verificación

```bash
# 1. Comandos canónicos (los tres servicios)
cd services/clinical-agenda && go build ./... && go vet ./... && go test ./... -count=1
cd services/billing-stock  && go build ./... && go vet ./... && go test ./... -count=1
cd services/api-gateway     && go build ./... && go vet ./... && go test ./... -count=1

# 2. Stack real y verificación end-to-end
cd deploy && docker compose up -d --build clinical-agenda billing-stock api-gateway
docker compose ps --format "table {{.Name}}\t{{.Status}}"     # 7/7 healthy

bash /tmp/wi15/verify-wi15.sh    # contrato + paginación + RBAC + ruteo + OpenAPI  → 51/51
bash /tmp/wi15/verify-rn02.sh    # RN-02 con cajas reales (apertura, cierre, roles)   → 15/15
```

Las credenciales de los 5 roles y el generador de TOTP están en `deploy/README.md`.
