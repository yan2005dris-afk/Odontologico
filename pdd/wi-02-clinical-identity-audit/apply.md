# PDD Apply Report — `wi-02-clinical-identity-audit`

- **Work Item:** `wi-02-clinical-identity-audit`
- **Feature Spec:** [`pdd/tasks/01-platform-gateway-auth.md`](../tasks/01-platform-gateway-auth.md)
- **Plan Reference:** [`pdd/wi-02-clinical-identity-audit/plan.md`](./plan.md)
- **Worktree / Branch:** `wt/clinical-agenda`
- **Status:** `COMPLETED (GREEN)`

---

## 1. Spec Coverage Matrix (`S1..S5`)

| Spec | Requirement Summary | Verification & Test Coverage | Status |
| :--- | :--- | :--- | :--- |
| **S1** | Argon2id password hashing + 5-role RBAC (`admin`, `receptionist`, `dentist`, `assistant`, `auditor`) + `POST /api/v1/auth/login` pre-MFA challenge token (`SRS §3`, `RNF-05`) | `TestNewUserAndPasswordArgon2id`, `TestAuthService_LoginMFAAndIdleTimeout`, `TestAuthHandler_EndToEndFlow` | PASS |
| **S2** | Mandatory 6-digit RFC 6238 TOTP MFA verification (`POST /api/v1/auth/mfa/verify`) issuing JWT with `mfa_verified=true` and anti-replay protection (`RNF-05`) | `TestUserLockoutAndTOTP`, `TestAuthService_LoginMFAAndIdleTimeout`, `TestAuthHandler_EndToEndFlow` | PASS |
| **S3** | 15-minute (`900s`) sliding idle session timeout (`ValidateAndTouchSession`, `POST /api/v1/auth/heartbeat`, `POST /api/v1/auth/logout`) returning `401 SESSION_IDLE_TIMEOUT` (`RNF-05`) | `TestSession_IdleTimeout15Minutes`, `TestAuthService_LoginMFAAndIdleTimeout`, `TestAuthHandler_EndToEndFlow` | PASS |
| **S4** | Immutable append-only clinical audit trail (`AuditService.Record`) + PostgreSQL trigger `prevent_audit_log_mutation` blocking `UPDATE` and `DELETE` (`RNF-06`) | `TestNewAuditEntry_ValidAndImmutable`, `TestAuditService_RecordAndList`, `migrations/00001_identity_audit.sql` | PASS |
| **S5** | Restricted audit log query (`GET /api/v1/audit/logs`) enforcing `admin` or `auditor` role with `403 Forbidden` for other roles (`SRS §3`, `RNF-06`) | `TestAuditService_RecordAndList`, `TestAuditHandler_ListLogs` | PASS |

---

## 2. Files Created / Modified

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

---

## 3. Executed Commands & Real Observed Output

```bash
cd services/clinical-agenda && go test -count=1 -v -cover ./... && go vet ./... && go build -o bin/clinical-agenda ./cmd/api
```

```text
=== RUN   TestAuditHandler_ListLogs
--- PASS: TestAuditHandler_ListLogs (0.00s)
PASS
coverage: 95.7% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/audit/adapters/http	0.014s	coverage: 95.7% of statements
=== RUN   TestAuditService_RecordAndList
--- PASS: TestAuditService_RecordAndList (0.00s)
PASS
coverage: 81.5% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/audit/application	0.012s	coverage: 81.5% of statements
=== RUN   TestNewAuditEntry_ValidAndImmutable
--- PASS: TestNewAuditEntry_ValidAndImmutable (0.00s)
=== RUN   TestNewAuditEntry_ValidationErrors
--- PASS: TestNewAuditEntry_ValidationErrors (0.00s)
PASS
coverage: 100.0% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/audit/domain	0.010s	coverage: 100.0% of statements
=== RUN   TestAuthHandler_EndToEndFlow
--- PASS: TestAuthHandler_EndToEndFlow (0.00s)
PASS
coverage: 93.7% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/identity/adapters/http	0.050s	coverage: 93.7% of statements
=== RUN   TestAuthService_LoginMFAAndIdleTimeout
--- PASS: TestAuthService_LoginMFAAndIdleTimeout (0.00s)
PASS
coverage: 88.8% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/identity/application	0.032s	coverage: 88.8% of statements
=== RUN   TestNewUserAndPasswordArgon2id
--- PASS: TestNewUserAndPasswordArgon2id (0.00s)
=== RUN   TestUserLockoutAndTOTP
--- PASS: TestUserLockoutAndTOTP (0.00s)
=== RUN   TestSession_IdleTimeout15Minutes
--- PASS: TestSession_IdleTimeout15Minutes (0.00s)
PASS
coverage: 86.6% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/identity/domain	0.045s	coverage: 86.6% of statements
```

---

## 4. Architectural Decisions & Out-of-Scope Boundaries

- **Hexagonal Pattern B:** `.port.go` interfaces (`auth.port.go`, `audit.port.go`) are colocated alongside `.service.go` use cases (`auth.service.go`, `audit.service.go`) with zero domain coupling to HTTP or persistence details.
- **Deterministic Clock Injection:** `Clock` port allows deterministic unit and integration testing of the 15-minute idle session timeout (`RNF-05`) and 15-minute account lockout without `time.Sleep`.
- **Out-of-Scope Respected:** Edge reverse proxy routing remains in `services/api-gateway/` (`wi-01-gateway-ingress`).
