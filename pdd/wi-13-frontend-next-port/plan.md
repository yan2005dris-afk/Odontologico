# Plan de Work Item — `wi-13-frontend-next-port`

Feature: `F07` (Portal Web / PWA Next.js) — continuación de `wi-10`, `wi-11`, `wi-12`.
Worktree: `wt/frontend` + `wt/gateway` (T1 toca el gateway).
Rol activo: Planner → Implementer. `MaxCorrections`: `200`.
Estado: `plan_pending_approval`.

---

## 1. Descripción y alcance

Las rutas de `frontend/src/app/**` fueron entregadas como *renderers de string* (`renderLoginPageHtml`, etc.) más 12 suites de componentes que afirman HTML. Nunca pasaron por `next build`: el root layout devuelve un documento completo (`<!DOCTYPE html><html><body>`), las páginas exportan funciones que no son válidas como Page, y ninguna ruta consume datos. El objetivo es que el portal sea una app Next real, servida por el contenedor, que consuma el API Gateway.

### Dentro del alcance (In Scope)

- **S1.** El root layout (`src/app/layout.tsx`) y el layout del portal (`src/app/(portal)/layout.tsx`) pasan a ser componentes Next válidos: el root devuelve `<html lang="es"><body>` y el del portal monta `AppShell` con el rol de la sesión.
- **S2.** Cada ruta del App Router es un componente válido y carga datos reales del gateway. Nada de props obligatorias que Next no pueda proveer.
- **S3.** El login es un formulario real: `POST /api/v1/auth/login` → `POST /api/v1/auth/mfa/verify` → sesión persistida en cookie `httpOnly` `sigo_session` (`SameSite=Lax`, `Secure` fuera de desarrollo, TTL 15 min alineado a `RNF-05`).
- **S4.** Los datos viajan **same-origin**: el navegador habla solo con Next; los componentes de servidor y las *server actions* consultan el gateway por la red interna (`GATEWAY_INTERNAL_URL`, por defecto `http://api-gateway:8080`). Sin CORS y sin exponer el gateway al navegador.
- **S5.** Páginas con endpoint de lectura disponible consumen datos reales: `/patients` (`GET /api/v1/patients`, `GET /api/v1/patients/{id}/header`), `/patients/[id]` (`GET .../odontogram`, `.../periodontograms`, `.../soep-notes`), `/agenda` (`GET /api/v1/scheduling/appointments`).
- **S6.** Páginas sin endpoint de listado (`/billing`, `/settlements`, `/inventory`) muestran un estado vacío explícito que nombra la carencia, sin inventar datos ni fabricar cifras.
- **S7.** El gateway deja de exigir Bearer en `POST /api/v1/auth/mfa/verify`, que solo puede portar `challengeToken` (ver §2 y §6).
- **S8.** El stack levanta con `docker compose up -d --build` desde `deploy/`: `frontend` construye con `output: "standalone"` y sirve en `:3000`.
- **S9.** Los 12 suites de componentes existentes siguen verdes sin reescribirse; se agregan pruebas de ruta con `fetch` inyectado.

### Fuera del alcance (Out of Scope)

- Convertir los renderers de string a JSX interactivo (hidratación de odontograma, hojas, POS). Requiere work item propio por cada módulo y reescribir sus tests.
- Endpoints de listado para facturación, liquidaciones e inventario en `billing-stock` (hoy solo exponen `POST`).
- Persistencia real en PostgreSQL: los adaptadores `*pg` siguen siendo en memoria.
- TLS 1.3 en el borde: el gateway escucha HTTP plano en `:8080` dentro de la red interna; el mapeo `8443` de compose queda sin uso hasta que se monten certificados.
- Migración de blobs MinIO → RustFS (copia a nivel objeto).

---

## 2. Dependencias previas

- `wi-10`, `wi-11`, `wi-12` (vistas, cliente HTTP tipado y suites de componentes).
- `wi-01` (gateway): el *allowlist* público de `proxy_router.go:137-141` solo contempla `/api/v1/auth/login`.
- `wi-02` (identidad): `POST /api/v1/auth/login`, `POST /api/v1/auth/mfa/verify`, `POST /api/v1/auth/heartbeat`.
- Contenedores `rustfs`, `postgres`, `redis` ya verdes (`docker compose ps`).

---

## 3. Specs numeradas

- **S1 — Root layout válido.** `src/app/layout.tsx` exporta `metadata` y un componente que devuelve `<html lang="es"><body className="bg-slate-50 text-slate-900 antialiased">`. Sin `<!DOCTYPE>` propio: Next emite el documento.
- **S2 — Layout del portal.** `src/app/(portal)/layout.tsx` recibe `children`, resuelve rol desde la cookie de sesión y renderiza el `AppShell` como fragmento HTML dentro de un contenedor; sin sesión redirige a `/login`.
- **S3 — Login real.** `POST` a `/api/v1/auth/login` con email+password; si `mfaRequired`, la misma pantalla pide el TOTP de 6 dígitos y llama a `/api/v1/auth/mfa/verify`; en éxito escribe la cookie y redirige a `/patients`. Error de credenciales → `401 INVALID_CREDENTIALS` visible; TOTP inválido → mensaje sin filtrar si el usuario existe.
- **S4 — Sesión.** Cookie `sigo_session`: `httpOnly`, `SameSite=Lax`, `path=/`, `maxAge` = `idleTimeoutMs` devuelto por el gateway. Respuesta `401 SESSION_IDLE_TIMEOUT` desde cualquier página → redirect a `/login?reason=idle_timeout`.
- **S5 — Datos same-origin.** Las llamadas de servidor usan `GATEWAY_INTERNAL_URL` (default `http://api-gateway:8080`); `NEXT_PUBLIC_API_GATEWAY_URL` queda solo como compatibilidad del cliente browser-side legado. El navegador no llama al gateway directamente.
- **S6 — `/patients`.** Lista los pacientes devueltos por `GET /api/v1/patients`; por cada uno muestra el header clínico con alertas críticas (`RF-02`) desde `GET /api/v1/patients/{id}/header`. Lista vacía → estado vacío explícito. El alta de paciente invoca `POST /api/v1/patients` mediante server action.
- **S7 — `/patients/[id]`.** Carga odontograma, periodontograma y notas SOEP en paralelo; cada pestaña sin datos muestra su estado vacío. Pestañas `PRESUPUESTOS` y `CONSENTIMIENTOS` renderizan estado vacío declarando que falta `GET` de plan.
- **S8 — `/agenda`.** Lista `GET /api/v1/scheduling/appointments`; la transición de estado usa `POST /api/v1/scheduling/appointments/{id}/transition`; conflicto `409 CHAIR_OVERLAP_CONFLICT` (`RN-03`) se muestra sin perder el formulario.
- **S9 — `/billing`, `/settlements`, `/inventory`.** Estado vacío explícito: "Sin endpoint de listado en `billing-stock`". Ningún dato simulado, ninguna cifra inventada, ningún total teórico antes del cierre ciego (`RN-02`).
- **S10 — Gateway MFA público.** `POST /api/v1/auth/mfa/verify` se reenvía sin exigir Bearer, igual que `login`. Cualquier otro path sigue exigiendo token. Test de regresión: sin token, `mfa/verify` llega al upstream; sin token, `GET /api/v1/patients` sigue devolviendo `401`.
- **S11 — Build del contenedor.** `next build` sale 0 con `output: "standalone"`, y `node server.js` sirve `:3000`. `docker compose up -d --build` deja los 7 servicios `Up (healthy)`.

---

## 4. Contrato de entrada

- **Variables**: `GATEWAY_INTERNAL_URL` (nuevo, server-side), `NEXT_PUBLIC_API_GATEWAY_URL` (existente, solo compatibilidad), `SESSION_COOKIE_NAME` fijo `sigo_session`.
- **Endpoints consumidos**: `POST /api/v1/auth/login`, `POST /api/v1/auth/mfa/verify`, `POST /api/v1/auth/heartbeat`, `GET /api/v1/patients`, `GET /api/v1/patients/{id}/header`, `GET /api/v1/patients/{id}/odontogram`, `GET /api/v1/patients/{id}/periodontograms`, `GET /api/v1/patients/{id}/soep-notes`, `GET /api/v1/scheduling/appointments`, `POST /api/v1/scheduling/appointments/{id}/transition`, `POST /api/v1/patients`.
- **Tipos**: los DTO de `src/lib/session-idle-guard.ts` y `src/lib/api-client.ts` son la fuente; no se duplican shapes nuevos sin reusar esos tipos.

---

## 5. Contrato de salida

- Cada ruta renderiza HTML server-side con datos del gateway; el fragmento visual sale de los renderers existentes, sin reescribirlos.
- Errores deterministas mapeados a UI: `401 SESSION_IDLE_TIMEOUT` → `/login?reason=idle_timeout`; `401 INVALID_CREDENTIALS` → mensaje en login; `409 CHAIR_OVERLAP_CONFLICT` → banner en agenda; `422 LEGAL_GUARDIAN_REQUIRED_FOR_MINOR` → bloqueo del alta.
- `docker compose up -d --build` desde `deploy/` levanta 7 servicios sanos.

---

## 6. Seguridad y resiliencia

- **Sesión**: cookie `httpOnly` (no legible por JS), `SameSite=Lax`, `Secure` cuando `NODE_ENV=production` y el origen es https; rotación por `expiresAtIso` del gateway.
- **MFA**: el TOTP nunca se loguea ni se persiste; se reenvía y se descarta. `S10` es un cambio en el *allowlist* público del gateway: se revisa que el upstream (`AuthHandler.handleVerifyMFA`) valide el `challengeToken` y su expiración, para que abrir el endpoint no lo convierta en un oráculo de fuerza bruta.
- **Rate limit**: el gateway ya aplica token bucket (`NewTokenBucketRateLimiter(300, 600, ...)`); `mfa/verify` hereda ese límite.
- **Sin fuga de datos**: los estados vacíos no muestran montos teóricos (`RN-02`) ni datos de otros pacientes; las páginas de servidor no exponen el token al bundle.
- **Fallos del gateway**: timeout de 5 s (`UpstreamTimeout`) se muestra como error recuperable con reintento; nunca se renderiza un éxito fabricado.

---

## 7. Archivos afectados

**Gateway**
- `services/api-gateway/internal/ingress/adapters/http/proxy_router.go` (allowlist `mfa/verify`)
- `services/api-gateway/internal/ingress/adapters/http/proxy_router_test.go` (regresión `S10`)

**Frontend**
- `frontend/src/app/layout.tsx`, `frontend/src/app/(portal)/layout.tsx` (S1, S2)
- `frontend/src/app/(auth)/login/page.tsx` (S3) + `frontend/src/app/(auth)/login/actions.ts`
- `frontend/src/app/(portal)/patients/page.tsx` (S6), `patients/[id]/page.tsx` (S7), `agenda/page.tsx` (S8)
- `frontend/src/app/(portal)/billing/page.tsx`, `settlements/page.tsx`, `inventory/page.tsx` (S9)
- `frontend/src/lib/api-client.ts` (métodos de lectura + `GATEWAY_INTERNAL_URL`), `frontend/src/lib/session-cookie.ts` (nuevo)
- Tests nuevos: `frontend/src/lib/__tests__/`+ suites de ruta con `fetch` inyectado

**Infra**
- `deploy/compose.yaml` (`GATEWAY_INTERNAL_URL`, `SESSION_COOKIE_SECURE`), `deploy/.env.example` (bloque RustFS + nuevas variables) — **requiere edición manual**: el runtime deniega escritura sobre `*.env.*`

---

## 8. Orden de ejecución (TDD)

| Task | Specs | Archivos | Prueba RED |
| :--- | :--- | :--- | :--- |
| T1 | S10 | gateway router + test | sin token, `mfa/verify` llega al upstream |
| T2 | S4 | `session-cookie.ts` | cookie httpOnly con TTL del gateway |
| T3 | S3 | login page + action | login→MFA→cookie; credencial inválida |
| T4 | S1, S2 | layouts | layout devuelve `<html>` y AppShell con rol |
| T5 | S6 | patients page | lista vacía vs lista con alertas |
| T6 | S7 | patients/[id] | tres fetch en paralelo, estados vacíos |
| T7 | S8 | agenda page | `409` no pierde el formulario |
| T8 | S9 | billing/settlements/inventory | estado vacío sin cifras |
| T9 | S11 | compose + build | `docker compose up -d --build` 7/7 healthy |
