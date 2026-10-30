# Plan de Work Item — `wi-02-clinical-identity-audit`

- **Feature Padre:** [`pdd/tasks/01-platform-gateway-auth.md`](../tasks/01-platform-gateway-auth.md)
- **Worktree / Rama Asignada:** `wt/clinical-agenda`
- **Directorio Objetivo:** `services/clinical-agenda/cmd/`, `services/clinical-agenda/internal/identity/`, `services/clinical-agenda/internal/audit/`, `services/clinical-agenda/migrations/`
- **Referencias SRS:** `SRS §3`, `RNF-04`, `RNF-05`, `RNF-06`, `RN-01`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Bootstrap del servicio `services/clinical-agenda/` (Go 1.24+) bajo arquitectura Hexagonal Pattern B.
- Migración SQL inicial `00001_identity_audit.sql` creando las tablas `users`, `user_sessions`, y `audit_logs` (con trigger PostgreSQL `prevent_audit_log_mutation` que bloquea `UPDATE` y `DELETE` para garantizar inmutabilidad `RNF-06`).
- Contexto `identity`:
  - Gestión de usuarios y asignación de los 5 roles del SRS §3 (`admin`, `receptionist`, `dentist`, `assistant`, `auditor`).
  - Hashing de contraseñas con Argon2id (`RNF-04`, `RNF-05`).
  - Flujo de login en 2 pasos: `POST /api/v1/auth/login` (valida credenciales y emite token pre-MFA) y `POST /api/v1/auth/mfa/verify` (valida código TOTP RFC 6238 de 6 dígitos, registra sesión con TTL de inactividad de 15 minutos y emite JWT final con `mfa_verified=true`).
  - Renovación de actividad y cierre explícito o por inactividad de 15 minutos (`POST /api/v1/auth/logout`, `POST /api/v1/auth/heartbeat`).
- Contexto `audit`:
  - Puerto y servicio `AuditService` (`audit.port.go` y `audit.service.go`) consumible por todos los módulos de `clinical-agenda` para registrar de forma inmutable cada operación `CREATE`, `READ`, `UPDATE`, `EXPORT` con `actor_id`, `actor_role`, `action`, `resource_type`, `resource_id`, `timestamp_iso8601`, `source_ip` y `metadata_json` (`RNF-06`).
  - Endpoint `GET /api/v1/audit/logs` restringido exclusivamente a roles `admin` y `auditor`.

### Fuera del alcance (Out of Scope)
- Ruteo reverse-proxy perimetral (cubierto en `wi-01-gateway-ingress`).
- Módulos de pacientes, odontograma, tratamientos y agenda (cubiertos en `wi-03` a `wi-07`).

---

## 2. Dependencias previas

- **Work Items previos:** Ninguno (work item raíz para `services/clinical-agenda/` en el worktree `wt/clinical-agenda`).
- **Base de datos:** Esquema inicial en `services/clinical-agenda/migrations/00001_identity_audit.sql`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Registro y autenticación con Argon2id + RBAC (`SRS §3`, `RNF-05`) | `POST /api/v1/auth/login` autentica usuarios activos en cualquiera de los 5 roles válidos (`admin`, `receptionist`, `dentist`, `assistant`, `auditor`) y retorna `mfa_required: true` con un `challenge_token` de corta duración (5 min). | Unit + Integration |
| **S2** | Verificación TOTP MFA obligatoria (`RNF-05`) | `POST /api/v1/auth/mfa/verify` valida el código TOTP de 6 dígitos contra el secreto MFA del usuario y emite el `access_token` JWT con `mfa_verified: true` y `sid` registrado con ventana de inactividad de `900s` (15 min). Códigos inválidos o reutilizados retornan `401 Unauthorized` (`INVALID_MFA_CODE`). | Unit + Adversarial |
| **S3** | Expiración de sesión tras 15 minutos de inactividad (`RNF-05`) | Si `last_activity_at` de la sesión supera `15 minutos` (`900s`), `ValidateAndTouchSession` marca la sesión como expirada y rechaza la operación con `401 Unauthorized` (`SESSION_IDLE_TIMEOUT`). | Unit + Adversarial |
| **S4** | Registro inmutable en bitácora de auditoría (`RNF-06`) | `AuditService.Record` persiste entradas con `actor_id`, `actor_role`, `action` (`CREATE`\|`READ`\|`UPDATE`\|`EXPORT`), `resource_type`, `resource_id`, `timestamp_iso8601` (UTC RFC3339) y `source_ip`. Cualquier intento de `UPDATE` o `DELETE` sobre `audit_logs` falla por restricción de dominio y trigger SQL. | Unit + Integration + Adversarial |
| **S5** | Consulta de auditoría restringida por rol (`SRS §3`, `RNF-06`) | `GET /api/v1/audit/logs?resource_type=&resource_id=&actor_id=&from=&to=` retorna registros paginados únicamente para `admin` y `auditor`; cualquier otro rol recibe `403 Forbidden`. | Integration + Adversarial |

---

## 4. Contrato de entrada

### `POST /api/v1/auth/login`
- Body JSON:
  - `email` (`string`, formato email válido, requerido).
  - `password` (`string`, longitud 8..128, requerido).
- Headers: `X-Source-IP` (IP de origen).

### `POST /api/v1/auth/mfa/verify`
- Body JSON:
  - `challenge_token` (`string`, JWT temporal emitido en login, requerido).
  - `totp_code` (`string`, exactamente 6 dígitos numéricos `^[0-9]{6}$`, requerido).

### `GET /api/v1/audit/logs`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`admin` | `auditor`), `X-MFA-Verified: true`.
- Query Params opcionales: `resource_type`, `resource_id`, `actor_id`, `action`, `from` (ISO-8601), `to` (ISO-8601), `limit` (1..200, default 50), `offset` ($\ge 0$).

---

## 5. Contrato de salida

### Respuesta `POST /api/v1/auth/mfa/verify` (`200 OK`)
```json
{
  "access_token": "eyJhbGciOi...",
  "session_id": "8f14e45f-ceea-467f-a8f1-4e45fceea67f",
  "user": {
    "id": "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
    "email": "dra.gomez@sigo-dental.com",
    "full_name": "Dra. Valeria Gómez",
    "role": "dentist",
    "mfa_verified": true
  },
  "idle_timeout_seconds": 900
}
```

### Respuesta `GET /api/v1/audit/logs` (`200 OK`)
```json
{
  "items": [
    {
      "id": "a1b2c3d4-e5f6-4a5b-8c9d-0e1f2a3b4c5d",
      "actor_id": "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
      "actor_role": "dentist",
      "action": "UPDATE",
      "resource_type": "ODONTOGRAM",
      "resource_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      "timestamp_iso8601": "2026-10-30T14:22:10Z",
      "source_ip": "192.168.10.24"
    }
  ],
  "total": 1
}
```

### Matriz de Estados HTTP
- `200 OK`: Login pre-MFA, verificación MFA exitosa, consulta de logs de auditoría.
- `204 No Content`: Logout o heartbeat exitoso.
- `400 Bad Request`: Payload JSON malformado o parámetros inválidos.
- `401 Unauthorized`: Credenciales inválidas, `INVALID_MFA_CODE`, o `SESSION_IDLE_TIMEOUT`.
- `403 Forbidden`: Rol sin permiso o `MFA_REQUIRED`.
- `423 Locked`: Cuenta bloqueada temporalmente tras 5 intentos fallidos consecutivos.

---

## 6. Seguridad y resiliencia

- **Protección de Credenciales y Secretos (`RNF-04`, `RNF-05`):** Contraseñas almacenadas exclusivamente con Argon2id (`time=3, memory=64MB, threads=4`); secretos TOTP cifrados con AES-256-GCM en reposo.
- **Prevención de SQL Injection:** Uso estricto de consultas parametrizadas (`$1, $2...`) en el adaptador PostgreSQL.
- **Inmutabilidad de Auditoría (`RNF-06`):** El repositorio de auditoría sólo expone `Append` y `List`; la migración SQL instala el trigger `prevent_audit_log_mutation` que aborta cualquier transacción que intente `UPDATE` o `DELETE` sobre `audit_logs`.
- **Bloqueo contra Fuerza Bruta:** Límite de 5 intentos fallidos de login/MFA antes de bloquear temporalmente el desafío por 15 minutos.

---

## 7. Archivos afectados

- `services/clinical-agenda/go.mod`
- `services/clinical-agenda/cmd/api/main.go`
- `services/clinical-agenda/migrations/00001_identity_audit.sql`
- `services/clinical-agenda/internal/identity/domain/user.go`
- `services/clinical-agenda/internal/identity/domain/session.go`
- `services/clinical-agenda/internal/identity/domain/user_test.go`
- `services/clinical-agenda/internal/identity/application/auth.port.go`
- `services/clinical-agenda/internal/identity/application/auth.service.go`
- `services/clinical-agenda/internal/identity/application/auth.service_test.go`
- `services/clinical-agenda/internal/identity/adapters/postgres/user_repository.go`
- `services/clinical-agenda/internal/identity/adapters/http/auth_handler.go`
- `services/clinical-agenda/internal/identity/adapters/http/auth_handler_test.go`
- `services/clinical-agenda/internal/audit/domain/audit_entry.go`
- `services/clinical-agenda/internal/audit/domain/audit_entry_test.go`
- `services/clinical-agenda/internal/audit/application/audit.port.go`
- `services/clinical-agenda/internal/audit/application/audit.service.go`
- `services/clinical-agenda/internal/audit/application/audit.service_test.go`
- `services/clinical-agenda/internal/audit/adapters/postgres/audit_repository.go`
- `services/clinical-agenda/internal/audit/adapters/http/audit_handler.go`
- `services/clinical-agenda/internal/audit/adapters/http/audit_handler_test.go`
