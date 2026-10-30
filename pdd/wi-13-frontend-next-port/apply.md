# Apply — `wi-13-frontend-next-port`

Work item: `pdd/wi-13-frontend-next-port/plan.md` (aprobado).
Rol: Implementer. `MaxCorrections`: 200.

Este documento registra los comandos ejecutados y su salida, en orden de
ejecución. La sección `## Review` del plan se completa en `review.md`.

---

## 1. Infraestructura — MinIO → RustFS y Dockerfiles (previo a wi-13)

### 1.1 Compose con RustFS

- `deploy/compose.yaml`: servicio `rustfs` (imagen `rustfs/rustfs:1.0.4`) con
  `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY` canónicos, `RUSTFS_ADDRESS=:9000`,
  `RUSTFS_CONSOLE_ADDRESS=:9001`, `RUSTFS_CONSOLE_ENABLE=true`,
  `RUSTFS_VOLUMES=/data`.
- Volumen renombrado `minio_clinical_blobs` → `rustfs_clinical_blobs`
  (`sigo-dental_rustfs_clinical_blobs`). El formato on-disk de RustFS **no** es
  compatible con MinIO: la migración de blobs existentes es una copia a nivel
  objeto (`mc mirror` / `rclone`), no un rename de volumen.
- `clinical-agenda` recibe `S3_ENDPOINT: http://rustfs:9000` y depende de
  `rustfs` con `condition: service_healthy`.
- `api-gateway` monta `../openapi:/app/openapi:ro` porque
  `OPENAPI_SPEC_PATH=/app/openapi/sigo-dental.v1.yaml`: sin el spec,
  `NewProxyRouter` + `log.Fatalf` matan el contenedor.
- Healthcheck de RustFS: `curl -f http://127.0.0.1:9000/health` (la imagen de
  runtime instala `curl` y `/health` está activo por defecto).

Comandos ejecutados:

```bash
cd deploy && docker compose up -d postgres redis rustfs
# → 3/3 healthy; /health 200; /health/ready 200; GET / (sin auth) 403
# → volumen sigo-dental_rustfs_clinical_blobs creado
```

### 1.2 Dockerfiles faltantes

- `services/api-gateway/Dockerfile`, `services/clinical-agenda/Dockerfile`,
  `services/billing-stock/Dockerfile`: multi-stage `golang:1.24-alpine` →
  `alpine:3.21`, `CGO_ENABLED=0`, `-trimpath`, `-ldflags="-s -w"`, usuario no
  root `sigo`. No hace falta `go.sum`: los módulos son stdlib-only.
- `frontend/Dockerfile`: multi-stage `node:22-alpine`, `npm install` (el repo
  no tiene lockfile), `build.args` para los `NEXT_PUBLIC_*` (se inlinean en
  build time), runner `standalone` con `node server.js` en `:3000`.
- `frontend/next.config.ts`: `output: "standalone"`.
- `.dockerignore` en los cuatro proyectos.

Verificación:

```bash
# 3 imágenes Go construidas y verificadas
sigo-dental-api-gateway   36MB
sigo-dental-clinical-agenda 25.7MB
sigo-dental-billing-stock  25.2MB
```

---

## 2. T1 — Gateway: `POST /api/v1/auth/mfa/verify` público (S10)

Archivos: `services/api-gateway/internal/ingress/adapters/http/proxy_router.go`
(`isPublicAuthPath()`), `.../proxy_router_mfa_test.go` (nuevo),
`.../proxy_router_test.go` (topología).

```bash
cd services/api-gateway && go test ./...
# → Go test: 63 passed in 5 packages
```

Evidencia de seguridad: `AuthService.VerifyMFA` valida el `challengeToken`
como JWT (firma HMAC + `typ == "mfa_challenge"` + búsqueda del usuario), por
lo que abrir el path no lo convierte en un oráculo de fuerza bruta.

---

## 3. T2 — Cookie de sesión httpOnly (S4)

Archivo: `frontend/src/lib/session-cookie.ts` (+ `session-cookie.test.ts`).

- Cookie `sigo_session`, `HttpOnly`, `SameSite=Lax`, `Path=/`,
  `Max-Age` derivado del `idleTimeoutMs` que devuelve el gateway, `Secure`
  sólo cuando el portal se sirve por TLS.
- Payload: `{ accessToken, role, userName? }`; un payload manipulado se trata
  como sesión inexistente.

```bash
cd frontend && node --import ./vitest.config.ts --test src/lib/session-cookie.test.ts
# → tests 9, pass 9, fail 0
```

---

## 4. T3 — Capa de autenticación y de datos (S3/S4/S5)

Archivos nuevos: `frontend/src/lib/auth-flow.ts` (+ test),
`frontend/src/lib/gateway-server.ts` (+ test), `frontend/src/lib/session.ts`,
`frontend/src/lib/portal-views.ts`.

- `auth-flow.ts`: máquina de estados pura (credenciales → MFA → sesión) sobre
  el puerto `AuthGateway`. El TOTP nunca se persiste ni se registra; un código
  MFA inválido no revela si la cuenta existe.
- `gateway-server.ts`: cliente server-side del gateway (`GATEWAY_INTERNAL_URL`,
  default `http://api-gateway:8080`) + mapeo puro del wire `snake_case` a los
  DTO `camelCase` del portal. Errores deterministas: `SESSION_IDLE_TIMEOUT`,
  `FORBIDDEN_ROLE`, `CHAIR_OVERLAP_CONFLICT`, `LEGAL_GUARDIAN_REQUIRED_FOR_MINOR`.
- `session.ts`: lee la cookie httpOnly y redirige a `/login` si no hay sesión.

```bash
cd frontend && node --import ./vitest.config.ts --test \
  src/lib/session-cookie.test.ts src/lib/gateway-server.test.ts src/lib/auth-flow.test.ts
# → tests 29, pass 29, fail 0
```

## 5. T4 — Layouts y shell (S1, S2)

- `frontend/src/app/layout.tsx`: root layout válido (`<html lang="es">`,
  `<body>` con la superficie `bg-slate-50`), `metadata` y `viewport` exportados.
- `frontend/src/app/(portal)/layout.tsx`: resuelve la sesión con
  `requirePortalSession()` (redirige a `/login` si no hay cookie) y monta el
  `AppShell` con el rol de la sesión.
- `frontend/src/components/shell/AppShellView.tsx`: vista React del shell que
  reutiliza el modelo puro `buildAppShellModel` (filtro RBAC, clases de
  superficie, indicador de sesión).
- `frontend/src/components/shell/SidebarNav.tsx`: navegación cliente con
  `usePathname()`; el módulo activo se resuelve en el navegador, no en el
  servidor.

## 6. T5–T8 — Páginas del portal (S6, S7, S8, S9)

Implementado por un worker acotado (`gentle-ai-worker`) sobre las superficies
autorizadas, con revisión del padre sobre el diff completo.

| Archivo | Rol |
| :--- | :--- |
| `frontend/src/app/(portal)/patients/page.tsx` | Listado con headers clínicos por paciente (fallback al item del listado si falla el header) y estados ready/empty/error |
| `frontend/src/app/(portal)/patients/create/route.ts` | `POST` de alta: `?created=1` o `?error=<CODE>` (nunca un éxito fabricado) |
| `frontend/src/app/(portal)/patients/[id]/page.tsx` | Header sticky + odontograma + periodontograma + SOEP con `Promise.allSettled`; pestañas PRESUPUESTOS/CONSENTIMIENTOS con estado vacío explícito |
| `frontend/src/app/(portal)/agenda/page.tsx` | Agenda con nombres reales de paciente, filtro de fecha/vista y formulario de transición real |
| `frontend/src/app/(portal)/agenda/transition/route.ts` | `POST` de transición: `?updated=1` o `?conflict=CHAIR_OVERLAP_CONFLICT&appointment=&target=` conservando la fecha/vista vistas |
| `frontend/src/app/(portal)/billing,settlements,inventory/page.tsx` | Estado vacío explícito que nombra el endpoint de lectura pendiente (RN-02: sin total teórico de caja) |
| `frontend/src/components/patients/StickyPatientHeader.tsx` | Nuevo helper `renderCriticalAlertBadgesHtml` (una sola fuente del markup de badges) |
| `frontend/src/components/patients/PatientRegistrationForm.tsx` | `name` en cada input, tipo de documento, sexo, contacto de emergencia, bloque de tutor legal y `formAction`/`formMethod` |
| `frontend/src/components/agenda/MultiChairCalendar.tsx` | Hook opcional `transitionControlHtml`; sin el hook el markup es idéntico al anterior |

Ajustes del padre sobre el trabajo del worker:

1. `AppShellView` usaba `props.activePath`, que no existe en su tipo
   (`Omit<AppShellProps, "activePath">`): corregido.
2. La redirección de `/agenda/transition` perdía la fecha y la vista vistas:
   el formulario ahora las transporta y el handler las devuelve.
3. `401 SESSION_IDLE_TIMEOUT` se renderizaba como estado de error: ahora
   redirige a `/login?reason=idle_timeout` (S4), también en los route handlers.
4. Los route handlers exportaban helpers (`mapTransitionErrorCode`,
   `splitPatientFullName`): Next.js sólo admite los handlers HTTP en
   `route.ts`, así que los helpers se movieron a `frontend/src/lib/portal-forms.ts`.
5. `frontend/tsconfig.tsbuildinfo` (basura de `tsc --noEmit`) borrado y
   `*.tsbuildinfo` agregado al `.gitignore`.

## 7. Verificación funcional del frontend

```bash
cd frontend && npm test
# → tests 108, suites 24, pass 108, fail 0   (línea base: 43 tests)

cd frontend && ./node_modules/.bin/tsc --noEmit
# → sin errores

cd frontend && npm run build
# → ✓ Compiled successfully
# → Route (app): /agenda, /agenda/transition, /billing, /inventory, /login,
#   /patients, /patients/[id], /patients/create, /settlements  (todas ƒ)
```

Servicios Go (sin regresiones):

```bash
cd services/api-gateway && go test ./...   # → 63 passed
cd services/clinical-agenda && go test ./... # → 30 passed
cd services/billing-stock && go test ./...  # → 16 passed
```

## 8. Defecto de integración encontrado en el stack (wi-01 ↔ wi-02)

Al probar el flujo real (login → `GET /api/v1/patients`) el gateway respondió
`401 SESSION_IDLE_TIMEOUT` con un token válido. Causa: el gateway refresca la
clave de sesión compartida `session:idle:<session_id>` en Redis, pero **nadie
la creaba**: el contexto de identidad guardaba las sesiones en su repositorio
en memoria y no tenía adaptador de Redis. Toda llamada autenticada del portal
contra el gateway hubiera fallado con 401.

Corrección (aditiva, con puerto propio):

- `services/clinical-agenda/internal/identity/application/auth.port.go`: nuevo
  puerto `IdleSessionRegistrar`.
- `services/clinical-agenda/internal/identity/application/auth.service.go`:
  `VerifyMFA` publica el marcador de sesión con el TTL de inactividad de
  15 min cuando el MFA es válido (nil-safe: sin Redis el servicio sigue
  funcionando).
- `services/clinical-agenda/internal/identity/adapters/redis/session_registrar.go`:
  adaptador RESP (`SET session:idle:<id> 1 EX 900`).
- `services/clinical-agenda/cmd/api/main.go`: cableado con `REDIS_ADDR`.

```bash
cd services/clinical-agenda && go test ./...   # → 33 passed (3 nuevos)
```

## 9. Ruta home del portal

El healthcheck del contenedor `frontend` fallaba: la app **no tenía ruta `/`**
(404). Se agregó `frontend/src/app/(portal)/page.tsx`, que redirige a
`/patients` (la sesión ya fue resuelta por el layout del portal).

## 10. Verificación end-to-end sobre el stack (`docker compose up -d --build`)

```bash
cd deploy && docker compose up -d --build
# → 7/7 servicios healthy
```

| Prueba | Resultado |
| :--- | :--- |
| `GET /` sin sesión | `307` → `/login` |
| `GET /login` | `200` (documento Next real) |
| `POST /api/v1/auth/login` (gateway) | `200 {"mfa_required":true,"challenge_token":…}` |
| `POST /api/v1/auth/mfa/verify` (gateway, sin Bearer) | `200` con `access_token` — **confirma S10** |
| clave Redis `session:idle:<sid>` | creada por el servicio de identidad |
| `GET /api/v1/patients` (gateway, con Bearer) | `200 {"items":[…],"total":1}` |
| `POST /api/v1/patients` (gateway) | `201` con `hc_number` `HC-2026-000142` |
| `GET /api/v1/patients/{id}/header` | `200` con `full_name` |
| `GET /api/v1/scheduling/appointments` | `200 {"items":null}` → estado vacío del portal |
| `GET /patients` con cookie de sesión | `200` renderiza el paciente real (`Roberto Carlos Pérez`, `HC-2026-000142`) |
| `GET /patients/{id}` con cookie de sesión | `200` con header clínico y pestaña `Odontograma 2D` |
| `GET /agenda` con cookie de sesión | `200` con estado vacío explícito |
| `GET /billing` con cookie de sesión | `200` con estado vacío explícito (RN-02) |

> Nota: el TOTP tiene protección anti-replay (`MarkOTPUsed`): cada código de 6
> dígitos sirve una sola vez, por lo que la prueba automatizada usa la ventana
> de 30 s siguiente.

