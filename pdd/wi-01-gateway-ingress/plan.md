# Plan de Work Item — `wi-01-gateway-ingress`

- **Feature Padre:** [`pdd/tasks/01-platform-gateway-auth.md`](../tasks/01-platform-gateway-auth.md)
- **Worktree / Rama Asignada:** `wt/gateway`
- **Directorio Objetivo:** `services/api-gateway/`, `deploy/`, `openapi/`
- **Referencias SRS:** `SRS §3`, `SRS §7`, `RNF-01`, `RNF-02`, `RNF-03`, `RNF-04`, `RNF-05`, `SRS §8.2`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Inicialización del módulo Go 1.24+ en `services/api-gateway/` con arquitectura Hexagonal Pattern B (`internal/ingress/domain`, `internal/ingress/application`, `internal/ingress/adapters`).
- Orquestación reproducible en `deploy/compose.yaml` para `postgres:16-alpine`, `redis:7-alpine`, `minio/minio`, `api-gateway`, `clinical-agenda`, `billing-stock` y `frontend`.
- Middleware de autenticación JWT (RS256/HS256), validación de estado MFA (`mfa_verified == true` obligatorio para rutas clínicas y financieras según `RNF-05`) y control de sesión inactiva de 15 minutos (`900s` sliding TTL en Redis según `RNF-05`).
- Middleware de autorización RBAC en el borde para los 5 perfiles (`admin`, `receptionist`, `dentist`, `assistant`, `auditor`).
- Reverse Proxy HTTP/WSS hacia `clinical-agenda` (`/api/v1/auth/*`, `/api/v1/patients/*`, `/api/v1/clinical/*`, `/api/v1/treatments/*`, `/api/v1/scheduling/*`, `/api/v1/audit/*`) y `billing-stock` (`/api/v1/billing/*`, `/api/v1/settlements/*`, `/api/v1/inventory/*`), inyectando cabeceras confiables (`X-Request-ID`, `X-Actor-ID`, `X-Actor-Role`, `X-MFA-Verified`, `X-Source-IP`).
- Rate-limiting por IP/usuario para sostener $\ge 150$ usuarios concurrentes sin degradación (`RNF-03`) y telemetría de latencia para vigilar el SLA de $< 1.5\text{ s}$ (`RNF-01`).
- Publicación de especificación base OpenAPI v3.0 en `openapi/sigo-dental.v1.yaml` servida en `GET /openapi.json` y `GET /healthz`.

### Fuera del alcance (Out of Scope)
- Persistencia de usuarios, hashing Argon2id de contraseñas o emisión de tokens TOTP (delegado a `wi-02-clinical-identity-audit` en `services/clinical-agenda/`).
- Lógica de negocio de pacientes, odontograma, agenda o facturación.

---

## 2. Dependencias previas

- **Work Items previos:** Ninguno (work item raíz para el worktree `wt/gateway`).
- **Infraestructura requerida en pruebas:** Mock/In-memory `SessionStorePort` para pruebas unitarias y Redis/HTTP test servers (`httptest.Server`) para pruebas de integración del proxy.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Topología Docker Compose (`SRS §7`, `RNF-02`, `RNF-04`) | `deploy/compose.yaml` define `postgres`, `redis`, `minio`, `api-gateway`, `clinical-agenda`, `billing-stock` y `frontend` con `healthcheck`, red interna aislada y configuración TLS 1.3. | Acceptance / Integration |
| **S2** | Ruteo Reverse Proxy por contexto (`SRS §7`) | Peticiones a `/api/v1/patients/*`, `/api/v1/clinical/*`, `/api/v1/treatments/*`, `/api/v1/scheduling/*` se enrutan al upstream `clinical-agenda`; peticiones a `/api/v1/billing/*`, `/api/v1/settlements/*`, `/api/v1/inventory/*` se enrutan a `billing-stock` preservando método, query params y cabeceras `X-Actor-*`. | Unit + Integration |
| **S3** | Expiración por inactividad de 15 minutos (`RNF-05`) | Para cada request autenticado con `session_id`, el gateway verifica `session:idle:<session_id>` en `SessionStorePort`. Si existe y su inactividad es $\le 900\text{ s}$, renueva el TTL a `900s`; si no existe o expiró, responde `401 Unauthorized` (`SESSION_IDLE_TIMEOUT`). | Unit + Adversarial |
| **S4** | Exigencia obligatoria de MFA en rutas clínicas y financieras (`RNF-05`) | Cualquier request hacia `/api/v1/clinical/*`, `/api/v1/patients/*/anamnesis`, `/api/v1/treatments/*`, `/api/v1/billing/*` o `/api/v1/settlements/*` cuyo JWT tenga `mfa_verified: false` es bloqueado con `403 Forbidden` (`MFA_REQUIRED`). | Unit + Adversarial |
| **S5** | Matriz RBAC de 5 roles en el borde (`SRS §3`, `RNF-05`) | Un `receptionist` intentando modificar `/api/v1/clinical/odontograms` o un `assistant` intentando invocar `/api/v1/billing/invoices` recibe `403 Forbidden` (`INSUFFICIENT_ROLE_PERMISSION`). | Unit + Adversarial |
| **S6** | Observabilidad, SLA $< 1.5\text{ s}$ y OpenAPI v3.0 (`RNF-01`, `SRS §8.2`) | `GET /healthz` retorna `200 OK`; `GET /metrics` expone latencia por ruta (`gateway_request_duration_ms`); `GET /openapi.json` entrega el esquema OpenAPI 3.0 válido. | Integration |

---

## 4. Contrato de entrada

### Cabeceras y Parámetros de Entrada (Edge HTTP)
- `Authorization`: `Bearer <jwt_token>` (obligatorio en todas las rutas excepto `POST /api/v1/auth/login`, `GET /healthz`, `GET /openapi.json`).
- Claims requeridos dentro del JWT:
  - `sub` (`string`, UUID del usuario, requerido).
  - `sid` (`string`, UUID de la sesión activa en Redis, requerido).
  - `role` (`string`, enum: `admin` | `receptionist` | `dentist` | `assistant` | `auditor`, requerido).
  - `mfa_verified` (` boolean`, requerido).
  - `exp` (`int64`, Unix timestamp futuro, requerido).
- `X-Forwarded-For` / `RemoteAddr`: dirección IP de origen para propagar a `X-Source-IP` (`RNF-06`).

---

## 5. Contrato de salida

### Cabeceras Propagadas a Servicios Downstream
- `X-Request-ID`: UUID v4 generado o propagado.
- `X-Actor-ID`: `claims.sub`.
- `X-Actor-Role`: `claims.role`.
- `X-Session-ID`: `claims.sid`.
- `X-MFA-Verified`: `"true"` | `"false"`.
- `X-Source-IP`: IP cliente normalizada.

### Formato Estándar de Error JSON y Matriz HTTP
```json
{
  "error": {
    "code": "SESSION_IDLE_TIMEOUT",
    "message": "Session expired after 15 minutes of inactivity",
    "request_id": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```
- `200 OK` / `201 Created` / `204 No Content`: Respuesta exitosa desde el upstream o healthcheck.
- `401 Unauthorized`: Token ausente, firma JWT inválida, token expirado o inactividad $> 15\text{ min}$ (`SESSION_IDLE_TIMEOUT`).
- `403 Forbidden`: `MFA_REQUIRED` o `INSUFFICIENT_ROLE_PERMISSION`.
- `429 Too Many Requests`: Límite de tasa excedido (`RATE_LIMIT_EXCEEDED`).
- `502 Bad Gateway` / `504 Gateway Timeout`: Servicio downstream inalcanzable o timeout ($> 5\text{ s}$).

---

## 6. Seguridad y resiliencia

- **Cifrado y Transporte (`RNF-04`):** Configuración TLS 1.3 estricta (`MinVersion: tls.VersionTLS13`), cabeceras HSTS (`Strict-Transport-Security`), `X-Content-Type-Options: nosniff` y CORS restringido al origen del portal clínico.
- **Autenticación, MFA e Inactividad (`RNF-05`):** Verificación criptográfica constante del JWT; rechazo de algoritmo `none`; control estricto de `900s` de inactividad en Redis; exigencia de `mfa_verified=true` antes de cruzar a `clinical-agenda` o `billing-stock`.
- **Anti-Spoofing de Cabeceras (`RNF-06`):** El gateway elimina cualquier cabecera `X-Actor-ID`, `X-Actor-Role` o `X-MFA-Verified` enviada por el cliente externo antes de inyectar los valores verificados desde el JWT.
- **Timeouts y Circuit Breaking (`RNF-01`, `RNF-02`):** `ReadHeaderTimeout: 3s`, `WriteTimeout: 10s`, `IdleTimeout: 60s`, y timeout de proxy upstream configurado para proteger el presupuesto de $< 1.5\text{ s}$ en lecturas clínicas.

---

## 7. Archivos afectados

- `deploy/compose.yaml`
- `deploy/.env.example`
- `openapi/sigo-dental.v1.yaml`
- `services/api-gateway/go.mod`
- `services/api-gateway/cmd/gateway/main.go`
- `services/api-gateway/internal/ingress/domain/claims.go`
- `services/api-gateway/internal/ingress/domain/rbac_policy.go`
- `services/api-gateway/internal/ingress/domain/rbac_policy_test.go`
- `services/api-gateway/internal/ingress/application/gateway.port.go`
- `services/api-gateway/internal/ingress/application/gateway.service.go`
- `services/api-gateway/internal/ingress/application/gateway.service_test.go`
- `services/api-gateway/internal/ingress/adapters/redis/session_store.go`
- `services/api-gateway/internal/ingress/adapters/http/middleware.go`
- `services/api-gateway/internal/ingress/adapters/http/proxy_router.go`
- `services/api-gateway/internal/ingress/adapters/http/proxy_router_test.go`
