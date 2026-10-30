# PDD Apply Report — `wi-03-clinical-patients`

- **Work Item:** `wi-03-clinical-patients`
- **Feature Spec:** [`pdd/tasks/02-patients-anamnesis.md`](../tasks/02-patients-anamnesis.md)
- **Plan Reference:** [`pdd/wi-03-clinical-patients/plan.md`](./plan.md)
- **Worktree / Branch:** `wt/clinical-agenda`
- **Status:** `COMPLETED (GREEN)`

---

## 1. Spec Coverage Matrix (`S1..S5`)

| Spec | Requirement Summary | Verification & Test Coverage | Status |
| :--- | :--- | :--- | :--- |
| **S1** | Unified patient registration & minor (< 18 years) legal guardian rule (`422 LEGAL_GUARDIAN_REQUIRED_FOR_MINOR`) + unique document enforcement (`409 DUPLICATE_PATIENT_DOCUMENT`) (`RF-01`) | `TestNewPatient_AdultAndMinorGuardianRule`, `TestPatientService_CreateSearchAnamnesisAndAudit`, `TestPatientHandler_Endpoints` | PASS |
| **S2** | Fast indexed patient directory search (`GET /api/v1/patients?q=`) by document, `hc_number`, or name (`RF-01`, `RNF-01`) | `TestPatientService_CreateSearchAnamnesisAndAudit`, `TestPatientHandler_Endpoints` | PASS |
| **S3** | Structured medical anamnesis (`PUT /api/v1/patients/{id}/anamnesis`) restricted to `dentist` and `assistant` with `mfa_verified=true` (`RF-02`, `RNF-05`) | `TestPatientService_CreateSearchAnamnesisAndAudit`, `TestPatientHandler_Endpoints` | PASS |
| **S4** | Automatic derivation of high-visibility critical medical alert badges (`ALLERGY_PENICILLIN`, `ANTICOAGULANT_ACTIVE`, `UNCONTROLLED_SYSTEMIC`, `PREGNANCY_ACTIVE`) on patient header (`RF-02`) | `TestDeriveCriticalAlerts`, `TestPatientService_CreateSearchAnamnesisAndAudit`, `TestPatientHandler_Endpoints` | PASS |
| **S5** | Immutable audit log emission (`audit.Recorder`) on every create, read, and update of patient demographics or anamnesis (`RNF-06`) | `TestPatientService_CreateSearchAnamnesisAndAudit` | PASS |

---

## 2. Files Created / Modified

- `services/clinical-agenda/migrations/00002_patients_anamnesis.sql`
- `services/clinical-agenda/internal/patients/domain/patient.go`
- `services/clinical-agenda/internal/patients/domain/anamnesis.go`
- `services/clinical-agenda/internal/patients/domain/patient_test.go`
- `services/clinical-agenda/internal/patients/application/patient.port.go`
- `services/clinical-agenda/internal/patients/application/patient.service.go`
- `services/clinical-agenda/internal/patients/application/patient.service_test.go`
- `services/clinical-agenda/internal/patients/adapters/postgres/patient_repository.go`
- `services/clinical-agenda/internal/patients/adapters/http/patient_handler.go`
- `services/clinical-agenda/internal/patients/adapters/http/patient_handler_test.go`

---

## 3. Executed Commands & Real Observed Output

```bash
cd services/clinical-agenda && go test -count=1 -v -cover ./... && go vet ./... && go build -o bin/clinical-agenda ./cmd/api
```

```text
=== RUN   TestPatientHandler_Endpoints
--- PASS: TestPatientHandler_Endpoints (0.00s)
PASS
coverage: 84.9% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/patients/adapters/http	0.041s	coverage: 84.9% of statements
=== RUN   TestPatientService_CreateSearchAnamnesisAndAudit
--- PASS: TestPatientService_CreateSearchAnamnesisAndAudit (0.00s)
PASS
coverage: 85.4% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/patients/application	0.036s	coverage: 85.4% of statements
=== RUN   TestNewPatient_AdultAndMinorGuardianRule
--- PASS: TestNewPatient_AdultAndMinorGuardianRule (0.00s)
=== RUN   TestDeriveCriticalAlerts
--- PASS: TestDeriveCriticalAlerts (0.00s)
PASS
coverage: 96.1% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/patients/domain	0.026s	coverage: 96.1% of statements
```

---

## 4. Architectural Decisions & Out-of-Scope Boundaries

- **Single-Round-Trip Sticky Header:** `critical_alerts[]` are computed deterministically by `domain.DeriveCriticalAlerts` upon saving `Anamnesis` and stored directly on the patient projection so `GET /api/v1/patients/{id}` and `GET /api/v1/patients/{id}/header` return in $< 10\text{ ms}$ (`RNF-01`).
- **Strict Role Separation:** Receptionists can manage administrative demographic data (`RF-01`) and view critical warning badges, whereas mutating clinical anamnesis (`RF-02`) strictly requires `dentist` or `assistant` role with `X-MFA-Verified: true`.
