# Feature 01 — Platform, API Gateway, Identity/RBAC/MFA & Audit Trail

- **Feature ID:** `01-platform-gateway-auth`
- **SRS References:** `RNF-01`, `RNF-02`, `RNF-03`, `RNF-04`, `RNF-05`, `RNF-06`, Topology `§7`, Role Matrix `§3`
- **Status:** `awaiting_plan_approval`
- **Work Items:**
  - [`wi-01-gateway-ingress`](../wi-01-gateway-ingress/plan.md) (`wt/gateway` — `services/api-gateway/`, `deploy/`)
  - [`wi-02-clinical-identity-audit`](../wi-02-clinical-identity-audit/plan.md) (`wt/clinical-agenda` — `services/clinical-agenda/internal/identity`, `services/clinical-agenda/internal/audit`)

---

## 1. Purpose & Scope Boundaries

### In Scope
- Reproducible container topology (`deploy/compose.yaml`) for `postgres` (AES-256 volume encryption configuration), `redis`, `minio` (S3-compatible blob store), `api-gateway`, `clinical-agenda`, `billing-stock`, and `frontend`.
- Edge API Gateway (`services/api-gateway/`) providing TLS 1.3 configuration (`RNF-04`), reverse-proxy routing (`/api/v1/clinical/*`, `/api/v1/patients/*`, `/api/v1/treatments/*`, `/api/v1/appointments/*`, `/api/v1/auth/*` $\rightarrow$ `clinical-agenda`; `/api/v1/billing/*`, `/api/v1/settlements/*`, `/api/v1/inventory/*` $\rightarrow$ `billing-stock`), JWT RS256/HS256 verification, MFA claim enforcement (`mfa_verified=true` required for clinical and financial routes per `RNF-05`), Redis-backed 15-minute idle session timeout (`RNF-05`), rate limiting (`RNF-03`), and OpenTelemetry request latency metrics (`RNF-01`).
- Identity & RBAC bounded context (`services/clinical-agenda/internal/identity`) managing the 5 canonical roles (`admin`, `receptionist`, `dentist`, `assistant`, `auditor` per SRS `§3`), Argon2id password hashing, TOTP/OTP MFA enrollment and challenge verification (`RNF-05`), and session issuance/revocation.
- Immutable Clinical Audit Trail bounded context (`services/clinical-agenda/internal/audit`) recording every CRUD and export operation on clinical records with `actor_id`, `role`, `action`, `resource_type`, `resource_id`, `timestamp_iso8601`, `source_ip`, and `payload_hash` (`RNF-06`), protected by database triggers against `UPDATE` and `DELETE`.

### Out of Scope
- Domain business logic for patients, odontogram, treatments, scheduling, or billing (covered in Features `02` through `06`).
- Frontend login/MFA screen rendering (covered in Feature `07`).

---

## 2. Numbered Specifications (`S1..S8`)

- **S1 — Reproducible Multi-Service Compose Topology (`SRS §7`, `RNF-02`, `RNF-04`):** `deploy/compose.yaml` defines healthchecked services for PostgreSQL 16, Redis 7, MinIO S3, `api-gateway`, `clinical-agenda`, `billing-stock`, and `frontend`, plus TLS 1.3 ingress config and environment templates.
- **S2 — Gateway Routing & Reverse Proxy (`SRS §7`, `RNF-01`, `RNF-03`):** `services/api-gateway/` routes `/api/v1/auth/*`, `/api/v1/patients/*`, `/api/v1/clinical/*`, `/api/v1/treatments/*`, `/api/v1/scheduling/*`, and `/api/v1/audit/*` to `clinical-agenda`, and `/api/v1/billing/*`, `/api/v1/settlements/*`, and `/api/v1/inventory/*` to `billing-stock`, propagating `X-Request-ID`, `X-Forwarded-For`, and verified actor headers (`X-Actor-ID`, `X-Actor-Role`, `X-MFA-Verified`).
- **S3 — 15-Minute Idle Session Timeout in Redis (`RNF-05`):** Every authenticated request checks and refreshes a sliding TTL key `session:idle:<session_id>` in Redis set to `900` seconds (15 minutes). Requests arriving after key expiration return `401 Unauthorized` with code `SESSION_IDLE_TIMEOUT` and force token revocation.
- **S4 — Mandatory MFA Enforcement for Clinical & Financial Scopes (`RNF-05`):** Any request targeting `/api/v1/clinical/*`, `/api/v1/patients/*/anamnesis`, `/api/v1/treatments/*`, `/api/v1/billing/*`, or `/api/v1/settlements/*` without `mfa_verified: true` in the validated JWT claims is rejected by the gateway and service middleware with `403 Forbidden` (`MFA_REQUIRED`).
- **S5 — 5-Role RBAC Matrix Enforcement (`SRS §3`, `RNF-05`):** Fine-grained permission middleware enforces the exact SRS §3 role matrix across `admin` (*Administrador TI*), `receptionist` (*Recepcionista / Cajero*), `dentist` (*Odontólogo General / Especialista*), `assistant` (*Asistente Dental*), and `auditor` (*Auditor / Contador*).
- **S6 — Authentication & TOTP MFA Flow (`RNF-05`):** `POST /api/v1/auth/login` verifies credentials (Argon2id) and issues a pre-MFA token; `POST /api/v1/auth/mfa/verify` validates a 6-digit TOTP/OTP code and upgrades the session to a full `mfa_verified=true` JWT registered in Redis with a 15-minute sliding TTL.
- **S7 — Immutable Append-Only Audit Log (`RNF-06`):** `audit_logs` table and `AuditService.Record(ctx, entry)` persist every clinical CRUD/export event with `id`, `actor_id`, `actor_role`, `action` (`CREATE`, `READ`, `UPDATE`, `EXPORT`), `resource_type`, `resource_id`, `source_ip`, `occurred_at` (ISO-8601 UTC), and `metadata_json`. PostgreSQL trigger `prevent_audit_log_mutation` raises an exception on any `UPDATE` or `DELETE`.
- **S8 — OpenAPI v3.0 & Telemetry (`SRS §8.2`, `RNF-01`):** Gateway exposes `/healthz`, `/readyz`, `/metrics` (histogram of request latency verifying $< 1.5\text{ s}$ SLA), and serves consolidated OpenAPI v3.0 specs at `/openapi.json`.

---

## 3. Worktree & Parallelization Strategy

| Work Item | Git Worktree Branch | Target Directory Boundary | Upstream Dependency |
| :--- | :--- | :--- | :--- |
| `wi-01-gateway-ingress` | `wt/gateway` | `services/api-gateway/**`, `deploy/**`, `openapi/**` | None (root bootstrap) |
| `wi-02-clinical-identity-audit` | `wt/clinical-agenda` | `services/clinical-agenda/cmd/**`, `services/clinical-agenda/internal/identity/**`, `services/clinical-agenda/internal/audit/**`, `services/clinical-agenda/migrations/00001_identity_audit.sql` | None (root bootstrap for `clinical-agenda`) |
