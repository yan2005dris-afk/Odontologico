# Plan de Work Item — `wi-18-dev-access-and-demo-data`

Feature: `F07` (`07-frontend-portal-ui`) + `F01` (`01-platform-gateway-auth`) — acceso de desarrollo.
Worktrees: `wt/clinical-agenda`, `wt/billing-stock`, `wt/frontend`.
Rol activo: Planner → Implementer. `MaxCorrections`: `200`.
Estado: `plan_pending_approval`.
Base: commit `a43c2ea` (`wi-15` mergeado).
Origen: **directiva humana** — «una migración para ingresar las credenciales, para probar diferentes
roles, y mejorar el login; de momento el MFA no debería estar».

---

## 1. Descripción y alcance

El seed de los 5 roles RBAC ya existe (`wi-15` `S0`), pero el ciclo de prueba es incómodo: cada
ingreso exige calcular un TOTP. Este work item hace que **probar un rol sea un clic**, y que las
pantallas tengan **datos que mirar**.

### Dentro del alcance (In Scope)

- **S0.** Tres interruptores de entorno, con **default seguro**:
  - `MFA_REQUIRED` (backend, default `true`): MFA obligatorio.
  - `SIGO_SEED_DEMO_DATA` (backend, default `false`): siembra datos de demostración.
  - `NEXT_PUBLIC_DEV_ROLE_LOGIN` (frontend, default `false`): selector de rol en la pantalla de login.
- **S1.** `clinical-agenda`: MFA opcional. Con `MFA_REQUIRED=false`, `POST /api/v1/auth/login`
  emite **directamente** la sesión (`access_token`, `session_id`, `user`, `idle_timeout_seconds`)
  con `mfa_verified=true`; `POST /api/v1/auth/mfa/verify` responde `MFA_DISABLED`.
- **S2.** Frontend: `auth-flow.ts` reconoce la respuesta con token directo y salta al paso
  `authenticated` sin pedir TOTP. El paso MFA se conserva intacto cuando el backend lo exige.
- **S3.** Frontend: selector de rol de desarrollo en la pantalla de login, **visible sólo** con
  `NEXT_PUBLIC_DEV_ROLE_LOGIN=true`, con las 5 cuentas sembradas y su contraseña.
- **S4.** `clinical-agenda`: seed de datos clínicos de demostración.
- **S5.** `billing-stock`: seed de datos financieros de demostración.
- **S6.** `deploy/README.md`: tabla de cuentas, modo sin MFA, interruptores y advertencias.

### Fuera del alcance (Out of Scope)

- **Reactivar el MFA no se elimina**: el código de MFA sigue presente y probado; sólo se puede
  desactivar por configuración. Es una decisión de entorno, no una eliminación de la garantía.
- Persistencia real en PostgreSQL (sigue en memoria).
- Cambios en la matriz RBAC del gateway: **no** se toca. El `auditor` sigue sin leer fichas clínicas.
- Las pantallas de lista (`wi-16`, `wi-17`): este work item sólo deja los datos disponibles.

---

## 2. Dependencias previas

- **`wi-15-read-contract-listings`** (merged): los 11 `GET` de listado y el seed de los 5 roles.
- **`wi-14-shadcn-admin-foundation`** (merged): primitivos `ui/`, `roles.ts`, shell.

---

## 3. Specs numeradas

- **S0 — Interruptores de entorno.** Los tres se leen en el arranque; ninguno se puede activar por
  accidente en un entorno que no lo pida, porque los defaults son los seguros.
- **S1 — MFA opcional en el servicio de identidad.** `AuthService` recibe `mfaRequired bool`.
  Con MFA activo, el flujo no cambia **en absoluto**: mismo desafío, misma verificación TOTP, mismo
  anti-replay de 30 s. Con MFA desactivado, `Login` crea la sesión directamente reutilizando el
  mismo camino de creación de sesión que `VerifyMFA` (misma firma de token, mismo registro de
  sesión idle en Redis, misma traza `RNF-06` con `mfa_verified`), de modo que **el resto del
  sistema no puede distinguir** una sesión sin MFA verificado de una verificada salvo por la
  bandera.
  - **Seguridad**: el interruptor vive en el **servicio de identidad**, no en el borde. El gateway
    sigue exigiendo `mfa_verified=true` en las rutas clínicas y financieras (`RNF-05`): con el
    MFA desactivado el token sale con `mfa_verified=true`, pero **nadie** puede fabricar un token
    sin pasar por `Login`, que exige la contraseña correcta.
  - `VerifyMFA` con MFA desactivado responde `409 MFA_DISABLED` en vez de validar un código.
- **S2 — Frontend sin paso MFA.** `submitCredentials` distingue tres casos: `mfa_required=true` →
  paso MFA (comportamiento actual); respuesta con `access_token` → sesión directa; cualquier otra
  cosa → el error que ya existe.
- **S3 — Selector de rol.** Botones con el nombre y el rol de cada cuenta sembrada; al pulsarlos
  rellenan correo y contraseña y envían el formulario. Badge visible «MODO DESARROLLO».
- **S4 — Datos clínicos de demostración.** Pacientes (adultos, un menor con tutor, uno con
  alertas críticas `RF-02`), agenda del día en 3 sillones, planes de tratamiento en los cuatro
  estados, consentimientos en los tres estados, y para un paciente completo: odontograma,
  periodontograma y notas SOEP.
- **S5 — Datos financieros de demostración.** Comprobantes pagados y pendientes, **una caja
  `OPEN` y una `CLOSED`** (para poder ejercitar `RN-02` de verdad), liquidaciones, reglas de
  comisión, insumos (algunos bajo el mínimo) y recetas.
- **S6 — Documentación.** `deploy/README.md` con: tabla de las 5 cuentas (correo, contraseña, rol,
  TOTP), cómo entrar en modo sin MFA, cómo sembrar datos demo y **cuándo hay que volver a
  encender el MFA**.

---

## 4. Contrato de entrada

- **Variables de entorno**: `MFA_REQUIRED` (bool, default `true`), `SIGO_SEED_DEMO_DATA` (bool,
  default `false`), `NEXT_PUBLIC_DEV_ROLE_LOGIN` (bool, default `false`).
- **Wire format**: `POST /api/v1/auth/login` conserva `mfa_required`, `challenge_token` y
  `expires_in_seconds`, y **añade** `access_token`, `session_id`, `user` e
  `idle_timeout_seconds` cuando el MFA está desactivado. Los clientes que leen sólo los campos
  antiguos siguen funcionando.

---

## 5. Contrato de salida

- `go build`/`go vet`/`go test` verdes en los dos servicios; `pnpm test`/`tsc`/`build`/`lint`
  verdes en el frontend.
- Con `MFA_REQUIRED=false`: login → sesión en **un** paso, con los 5 roles; el gateway sigue
  aplicando la matriz RBAC y el timeout idle de 15 min.
- Con `MFA_REQUIRED=true` (default): el flujo MFA actual no cambia; los tests existentes siguen
  verdes.
- Con `SIGO_SEED_DEMO_DATA=true`: los 11 listados devuelven filas, y existe una caja `OPEN` (sin
  total teórico para nadie) y una `CLOSED` (con total teórico sólo para `auditor`/`admin`).

---

## 6. Seguridad y resiliencia

- El interruptor de MFA **no se puede activar por descuido**: el default del código es MFA activo.
  `deploy/compose.yaml` lo desactiva **explícitamente** para el stack de desarrollo, con un
  comentario visible.
- El selector de rol **no existe** en el bundle salvo que se compile con
  `NEXT_PUBLIC_DEV_ROLE_LOGIN=true`: el default es que no se compile.
- Las contraseñas de desarrollo viajan en el bundle del cliente **sólo** en ese modo. El aviso
  «MODO DESARROLLO» va en la propia pantalla.
- El seed de demostración es **idempotente** y no pisa datos existentes.
- `RN-02` se mantiene: el seed crea una caja `OPEN` y una `CLOSED`, y la caja `OPEN` no expone
  total teórico para ningún rol.

---

## 7. Archivos afectados

**`wt/clinical-agenda`**
- `internal/identity/application/auth.service.go` (+ test) — `mfaRequired`, `Login` directo, `ErrMFADisabled`.
- `internal/identity/application/auth.port.go` — `LoginOutput` ampliado.
- `internal/identity/adapters/http/auth_handler.go` (+ test) — `MFA_DISABLED`.
- `cmd/api/main.go` — lee `MFA_REQUIRED`; llama al seed de demostración.

**`wt/billing-stock`**
- `cmd/api/main.go` — llama al seed de demostración.

**`wt/frontend`**
- `src/lib/auth-flow.ts` (+ test) — respuesta con token directo.
- `src/lib/gateway-server.ts` — `LoginWireResponse` ampliado.
- `src/lib/dev-accounts.ts` (nuevo) — las 5 cuentas de desarrollo.
- `src/components/auth/LoginForm.tsx` — selector de rol.
- `src/app/(auth)/login/page.tsx` — badge de modo desarrollo.

**PDD**: `pdd/wi-18-dev-access-and-demo-data/{apply,review}.md`, `deploy/README.md`.

---

## 8. Orden de ejecución (TDD)

| Task | Specs | Prueba RED |
| :--- | :--- | :--- |
| T0 | S0 | — (configuración) |
| T1 | S1 | `Login` con MFA desactivado devuelve `access_token` y `mfa_required=false`; con MFA activo, el comportamiento actual no cambia |
| T2 | S2 | `submitCredentials` con respuesta con token → `step: "authenticated"` |
| T3 | S3 | el selector sólo se renderiza con el flag activo |
| T4 | S4 | el seed deja filas en los 5 listados clínicos y es idempotente |
| T5 | S5 | el seed deja una caja `OPEN` sin teórico y una `CLOSED` con teórico sólo para auditor |
| T6 | S6 | — (documentación) |
| T7 | S0..S6 | verificación end-to-end con los 5 roles, sin MFA, y con datos |

**Criterio de salida**: T7 verde, `review` de 4 lentes en `review.md` con citas `file:line`, y
`apply.md` con los comandos ejecutados y sus salidas.
