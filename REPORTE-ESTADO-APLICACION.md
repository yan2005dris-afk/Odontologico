# Reporte de Estado — SIGO-Dental

**Proyecto:** Sistema Integral de Gestión Odontológica y Expediente Clínico Digital
**Fecha del reporte:** 30 de octubre de 2026
**Commit auditado:** `4091810` (`main`, 2026-10-30 03:18:41 -0500)
**Alcance:** verificación del estado real del código contra el SRS v1.0 y el board PDD

---

## 1. Resumen ejecutivo

**El sistema compila, pasa sus pruebas y cubre el núcleo clínico. No está listo para producción, y la brecha no es de código: es de alcance.**

Lo que hay hoy es un backend funcional y probado (3 servicios Go, 120 archivos, 54 funciones de prueba) con un frontend Next.js que renderiza las pantallas clínicas pero **no las financieras**. La deuda real está en tres frentes concretos:

- **El portal no opera finanzas.** `/billing`, `/settlements` e `/inventory` muestran literalmente "Endpoint de lectura pendiente". No hay pantalla de cierre de caja ni de auditoría, aunque los endpoints existen.
- **Un defecto de ruteo en el gateway deja `RF-08` inalcanzable.** La aprobación de presupuestos devuelve 404 de punta a punta.
- **El OpenAPI que publica el gateway es ficción.** Anuncia rutas que no existen y omite las que sí.

Ninguno de los tres es difícil. Los tres están ya diagnosticados y planificados en `wi-15`, `wi-16` y `wi-17`, que esperan aprobación humana.

**Veredicto:** ~65% del alcance del SRS implementado, 100% de lo implementado pasa pruebas. Listo para demo clínica, no para cobrar.

---

## 2. Estado de verificación (ejecutado, no inferido)

Todo lo siguiente se corrió en esta sesión sobre el commit `4091810`:

| Verificación | Comando | Resultado |
| :--- | :--- | :--- |
| Gateway — vet + tests | `go vet ./...` / `go test ./...` | ✅ limpio, 3 paquetes `ok` |
| Clinical-Agenda — vet + tests | `go vet ./...` / `go test ./...` | ✅ limpio, 12 paquetes `ok` |
| Billing-Stock — vet + tests | `go vet ./...` / `go test ./...` | ✅ limpio, 6 paquetes `ok` |
| Frontend — suite de pruebas | `pnpm test` | ✅ **160/160 tests, 23 archivos, 4.74 s** |
| Frontend — build de producción | `pnpm build` | ✅ 12 rutas generadas, 103 kB First Load JS |
| Frontend — lint | incluido en build | ⚠️ 1 warning (`_event` sin usar, `session-idle-guard.ts:135`) |

**Cero fallos. Un warning cosmético.** El estado de compilación no es el problema del proyecto.

### Volumen de código

| Componente | Archivos | Líneas |
| :--- | ---: | ---: |
| Go — producción | 120 totales | 9 446 |
| Go — pruebas | — | 5 419 |
| Frontend — TS/TSX | — | 13 176 |
| **Total** | — | **~28 000** |

Proporción test/código en Go: **57%**. Es una base sana, no un prototipo.

---

## 3. Cobertura funcional contra el SRS

### 3.1 Lo que está implementado y probado

| Req. | Módulo | Estado | Evidencia |
| :--- | :--- | :--- | :--- |
| `RF-01` | Registro unificado de pacientes | ✅ Backend + UI | `patients/adapters/http/patient_handler.go` |
| `RF-02` | Anamnesis y alertas médicas críticas | ✅ Backend + UI | `StickyPatientHeader.tsx` con `role="alert"` |
| `RF-03` | Odontograma 2D (FDI/Universal) | ✅ Backend + UI | `OdontogramCanvas.tsx`, `odontogram.service.go` |
| `RF-04` | Periodontograma digital | ✅ Backend + UI | `PeriodontogramGrid.tsx`, 6 puntos/diente |
| `RF-05` | Notas SOEP inmutables + addenda | ✅ Backend + UI | `soep_note.go`, bloqueo 24 h |
| `RF-06` | Consentimiento informado digital | ✅ Backend + UI | `consent.service.go`, firma + OTP |
| `RF-07` | Presupuestos multinivel | ✅ Backend | `treatment_handler.go:22` |
| `RF-08` | Aprobación → tareas agendables | ⚠️ **Backend sí, ruteo roto** | ver §4.1 |
| `RF-09` | Agenda multirrecinto | ✅ Backend + UI | `MultiChairCalendar.tsx` |
| `RF-10` | Máquina de estados de cita | ✅ Backend + UI | `scheduling/domain` |
| `RF-11` | Recordatorios 48 h / 24 h | ✅ Backend | `scheduling/adapters/messaging` |
| `RF-12` | POS y facturación electrónica | ⚠️ Backend sí, **sin pantalla** | ver §4.2 |
| `RF-13` | Liquidaciones a profesionales | ⚠️ Backend sí, **sin pantalla** | ver §4.2 |
| `RF-14` | Inventario y descuento automático | ⚠️ Backend sí, **sin pantalla** | ver §4.2 |

### 3.2 Reglas de negocio no negociables

| Regla | Estado | Nota |
| :--- | :--- | :--- |
| `RN-01` Inmutabilidad del expediente | ✅ Implementada y probada | Bloqueo a 24 h + addendum firmado |
| `RN-02` Cierre de caja ciego | ✅ Backend correcto | **Sin pantalla** — el invariante no se puede ejercer desde el portal |
| `RN-03` Prevención de solapamiento de sillón | ✅ Implementada y probada | Validación en dominio de `scheduling` |
| `RNF-05` Sesión 15 min + MFA | ✅ Implementada | ⚠️ MFA **no cubre** `/api/v1/treatment-plans` (§4.1) |
| `RNF-06` Bitácora de auditoría inmutable | ✅ Backend | **Sin pantalla** — no hay ruta `/audit` |
| `RNF-01` Carga < 1.5 s | ❓ **No medida** | Nadie ha instrumentado la métrica |

---

## 4. Hallazgos críticos

### 4.1 El gateway no rutea `/api/v1/treatment-plans` — `RF-08` inalcanzable

**Severidad: alta. Confirmado leyendo el código.**

`ResolveUpstream` (`services/api-gateway/internal/ingress/application/gateway.service.go:112`) enruta por el prefijo `/api/v1/treatments`. Pero el servicio clínico registra sus rutas bajo `/api/v1/treatment-plans` (`treatment_handler.go:23-25`).

Consecuencia: `POST /api/v1/treatment-plans/{id}/approve`, `.../items/{itemId}/complete` y `DELETE .../items/{itemId}` caen en `UpstreamUnknown` → **404 `ROUTE_NOT_FOUND`**. La aprobación de presupuestos no existe de punta a punta.

Efecto colateral: `RequiresMFA` (`rbac_policy.go:68`) también cubre `/api/v1/treatments`, no `/api/v1/treatment-plans`. Si alguien arregla solo el ruteo, la aprobación de presupuestos queda **fuera de la exigencia de MFA de `RNF-05`**. Son dos líneas, y hay que tocar las dos.

### 4.2 Tres módulos financieros sin interfaz

`billing/page.tsx:36`, `settlements/page.tsx:35` e `inventory/page.tsx:35` renderizan el texto "Endpoint de lectura pendiente". El backend de esos tres módulos está completo y probado; lo que falta son los endpoints de **listado** (`GET`) y las pantallas.

Peor caso: el cierre de caja ciego (`RN-02`) **tiene** endpoint (`GET /api/v1/billing/cash-registers/{id}` y `.../audit-report`) pero **no tiene ruta** `/cash-registers`. La regla de negocio más delicada del sistema es inejercitable desde la interfaz.

### 4.3 El OpenAPI publicado es ficticio

`proxy_router.go:255-280` publica un documento que anuncia rutas inexistentes:

- `/api/v1/clinical/odontograms` — no existe
- `/api/v1/treatments/plans` — no existe
- `/api/v1/settlements/commissions` — no existe
- `/api/v1/inventory/supplies` — no existe

Y omite las que sí existen. Además `buildOpenAPIJSONFromYAML` (`proxy_router.go:282-288`) **ignora el YAML** y devuelve siempre el default hardcodeado. Existe `openapi/sigo-dental.v1.yaml` en el repo; el gateway nunca lo lee.

Cualquier integrador que consuma `/openapi.json` va a construir contra un contrato falso.

### 4.4 El board PDD está desactualizado

`pdd/board.md` marca `wi-01` a `wi-12` como `implementing` / `plan_approved`. El código de todos ellos está implementado, probado y mergeado a `main` (commits `3d0731c`, `835fedf`, `375b2c2`, `ec729e5`). Solo `wi-13` y `wi-14` figuran correctamente como `implemented`.

El board subestima el avance real en 12 work items. Quien lo lea para planificar va a creer que hay menos hecho del que hay.

---

## 5. Deuda técnica abierta (heredada de `wi-14`)

Cuatro observaciones registradas en `pdd/wi-14-shadcn-admin-foundation/review.md` §5, ninguna bloqueante:

| # | Observación | Riesgo |
| :--- | :--- | :--- |
| 1 | El fix del `<Command>` vive en archivo vendorizado; un `shadcn add --overwrite` lo revierte | medio |
| 1b | Igual para el atributo `data-active` del sidebar vendorizado | medio |
| 2 | `IMPLEMENTED_ROUTES` es lista manual: una ruta nueva no anotada no aparece en navegación | bajo |
| 3 | `formatCents` depende del ICU de Node (`toLocaleString("es-EC")`) | bajo |
| 4 | `DataTable` usa `useSearchParams()`, exige página dinámica o `Suspense` | bajo |

Los dos primeros tienen test de regresión que falla si el defecto vuelve. Eso es la mitigación correcta.

---

## 6. Lo que falta para cerrar

Tres work items planificados, todos en `plan_pending_approval`. **Ninguno puede arrancar sin aprobación humana** (`AGENTS.md §6`).

| WI | Alcance | Bloquea |
| :--- | :--- | :--- |
| **`wi-15`** | 11 endpoints `GET` de listado + corrección del ruteo, MFA y OpenAPI del gateway | `wi-16`, `wi-17` |
| **`wi-16`** | Migración big-bang a React real: borra 14 renderers de HTML string, reescribe 12 suites | `wi-17` |
| **`wi-17`** | Pantallas financieras, `/users`, `/audit`, `/cash-registers`, dashboard, verificación de invariantes | Cierre del proyecto |

`wi-16` es el más caro: 14 archivos con `render*Html(...)` y 5 puntos de `dangerouslySetInnerHTML` en el portal. La decisión de big-bang (sin fase intermedia) ya está registrada como decisión humana.

---

## 7. Riesgos residuales

1. **`RNF-01` nunca se midió.** El SRS exige carga de expediente + render de odontograma en < 1.5 s sobre ≥ 15 Mbps. No hay instrumentación, ni prueba de carga, ni dato. Es un requisito contractual sin evidencia.
2. **Ningún entorno real ha corrido esto.** Hay `deploy/compose.yaml` y `.env.example`, pero no hay evidencia de que el stack completo (PostgreSQL + Redis + S3 + los 3 servicios + gateway) haya levantado junto. Los tests son unitarios y de handler; no hay prueba de integración end-to-end.
3. **Los adaptadores de persistencia no tienen tests.** `postgres/` y `s3/` figuran como `[no test files]` en los tres servicios. Toda la capa de repositorio está sin cubrir.
4. **Recordatorios `RF-11` sin verificación de entrega.** El código existe; no hay evidencia de que un WhatsApp/SMS salga realmente.

---

## 8. Métricas de esta sesión

| Métrica | Valor |
| :--- | :--- |
| Tiempo de trabajo | **~2 horas** |
| Tokens consumidos | **~1.2 M** (estimado) |
| Comandos ejecutados | ~15 (tests, vet, build, grep, git) |
| Archivos leídos | ~20 (board, SRS, 3 reviews, 3 planes, código de gateway y handlers) |
| Hallazgos nuevos confirmados | 4 (§4.1 a §4.4) |

*Las cifras de tiempo y tokens son estimaciones del operador, no mediciones instrumentadas.*

---

## 9. Recomendación

**Aprobar `wi-15` esta semana.** Es el desbloqueo de mayor apalancamiento: arregla un requisito roto (`RF-08`), cierra una brecha de seguridad (`RNF-05` sobre presupuestos), elimina un contrato de API falso, y habilita los otros dos work items. Es el trabajo más pequeño de los tres y el que más destraba.

`wi-16` y `wi-17` pueden correr después, en ese orden. `wi-16` es mecánico y de alto volumen; `wi-17` es el que realmente pone el sistema en condiciones de operar una clínica.

Antes de cualquier piloto real: instrumentar `RNF-01` y levantar el stack completo al menos una vez. Hoy nadie ha visto este sistema correr de verdad, de punta a punta, contra una base de datos.
