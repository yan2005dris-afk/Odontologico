# PDD Apply Report — `wi-05-clinical-soep-consents`

- **Work Item:** `wi-05-clinical-soep-consents`
- **Feature Spec:** [`pdd/tasks/03-clinical-odontogram-soep.md`](../tasks/03-clinical-odontogram-soep.md)
- **Plan Reference:** [`pdd/wi-05-clinical-soep-consents/plan.md`](./plan.md)
- **Worktree / Branch:** `wt/clinical-agenda`
- **Status:** `COMPLETED (GREEN)`

---

## 1. Spec Coverage Matrix (`S1..S5`)

| Spec | Requirement Summary | Verification & Test Coverage | Status |
| :--- | :--- | :--- | :--- |
| **S1** | Structured SOEP evolution note creation (`POST /api/v1/patients/{id}/soep-notes`) with SHA-256 clinician signature hash and configurable 24h lock window (`RF-05`) | `TestSOEPNote_24hLockImmutabilityAndAddendum`, `TestSOEPService_24hLockAndAddendum`, `TestSOEPConsentHandler_Endpoints` | PASS |
| **S2** | Automatic lock after 24h (`409 SOEP_NOTE_LOCKED` on `PUT` when `now >= locks_at`) and strict prohibition of `DELETE` (`409 CLINICAL_RECORD_IMMUTABLE`) (`RF-05`, `RN-01`) | `TestSOEPNote_24hLockImmutabilityAndAddendum`, `TestSOEPService_24hLockAndAddendum`, `TestSOEPConsentHandler_Endpoints` | PASS |
| **S3** | Cryptographically signed clinical addendum (`POST /api/v1/patients/{id}/soep-notes/{noteId}/addenda`) preserving original note text (`RN-01`) | `TestSOEPNote_24hLockImmutabilityAndAddendum`, `TestSOEPService_24hLockAndAddendum`, `TestSOEPConsentHandler_Endpoints` | PASS |
| **S4** | Digital informed consent signing (`POST /api/v1/patients/{id}/consents/{consentId}/sign`) via `BIOMETRIC` SVG strokes or 6-digit `OTP`, SHA-256 sealing, and encrypted S3/Blob storage (`RF-06`, `RNF-04`) | `TestPatientConsent_BiometricAndOTPSigning`, `TestConsentService_SignAndStoreEncryptedPDF`, `TestSOEPConsentHandler_Endpoints` | PASS |
| **S5** | Immutable audit log emission (`RNF-06`) on every SOEP note creation, update, addendum, and consent signature | `TestSOEPService_24hLockAndAddendum`, `TestConsentService_SignAndStoreEncryptedPDF` | PASS |

---

## 2. Files Created / Modified

- `services/clinical-agenda/migrations/00004_soep_consents.sql`
- `services/clinical-agenda/internal/clinical/domain/soep_note.go`
- `services/clinical-agenda/internal/clinical/domain/consent.go`
- `services/clinical-agenda/internal/clinical/domain/soep_note_test.go`
- `services/clinical-agenda/internal/clinical/domain/consent_test.go`
- `services/clinical-agenda/internal/clinical/application/soep.port.go`
- `services/clinical-agenda/internal/clinical/application/soep.service.go`
- `services/clinical-agenda/internal/clinical/application/soep.service_test.go`
- `services/clinical-agenda/internal/clinical/application/consent.port.go`
- `services/clinical-agenda/internal/clinical/application/consent.service.go`
- `services/clinical-agenda/internal/clinical/application/consent.service_test.go`
- `services/clinical-agenda/internal/clinical/adapters/postgres/soep_repository.go`
- `services/clinical-agenda/internal/clinical/adapters/postgres/consent_repository.go`
- `services/clinical-agenda/internal/clinical/adapters/s3/consent_blob_store.go`
- `services/clinical-agenda/internal/clinical/adapters/http/soep_consent_handler.go`
- `services/clinical-agenda/internal/clinical/adapters/http/soep_consent_handler_test.go`

---

## 3. Executed Commands & Real Observed Output

```bash
cd services/clinical-agenda && go test -count=1 -v -cover ./... && go vet ./... && go build -o bin/clinical-agenda ./cmd/api
```

```text
=== RUN   TestOdontogramAndPeriodontogramHandler
--- PASS: TestOdontogramAndPeriodontogramHandler (0.00s)
=== RUN   TestSOEPConsentHandler_Endpoints
--- PASS: TestSOEPConsentHandler_Endpoints (0.00s)
PASS
coverage: 85.6% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/http	0.025s	coverage: 85.6% of statements
=== RUN   TestConsentService_SignAndStoreEncryptedPDF
--- PASS: TestConsentService_SignAndStoreEncryptedPDF (0.00s)
=== RUN   TestOdontogramService_LifecycleAndRN01
--- PASS: TestOdontogramService_LifecycleAndRN01 (0.00s)
=== RUN   TestPeriodontogramService_RecordAndList
--- PASS: TestPeriodontogramService_RecordAndList (0.00s)
=== RUN   TestSOEPService_24hLockAndAddendum
--- PASS: TestSOEPService_24hLockAndAddendum (0.00s)
PASS
coverage: 81.3% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/clinical/application	0.010s	coverage: 81.3% of statements
=== RUN   TestPatientConsent_BiometricAndOTPSigning
--- PASS: TestPatientConsent_BiometricAndOTPSigning (0.00s)
=== RUN   TestResolveToothCode_All52TeethAndInvalidCodes
--- PASS: TestResolveToothCode_All52TeethAndInvalidCodes (0.00s)
=== RUN   TestOdontogramFinding_ColorsAndRN01Immutability
--- PASS: TestOdontogramFinding_ColorsAndRN01Immutability (0.00s)
=== RUN   TestNewPeriodontogramChart_NICAndBOPCalculation
--- PASS: TestNewPeriodontogramChart_NICAndBOPCalculation (0.00s)
=== RUN   TestSOEPNote_24hLockImmutabilityAndAddendum
--- PASS: TestSOEPNote_24hLockImmutabilityAndAddendum (0.00s)
PASS
coverage: 81.9% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/clinical/domain	0.010s	coverage: 81.9% of statements
```

---

## 4. Architectural Decisions & Out-of-Scope Boundaries

- **Injectable Clock for 24h Lock (`RN-01`):** `SOEPService` accepts a `Clock` port and configurable lock duration (default `24h`), verified deterministically at $+2\text{h}$ (unlocked) and $+25\text{h}$ (locked) without sleeping.
- **Tamper-Evident Consent Sealing (`RF-06`, `RNF-04`):** Signed consent PDFs are hashed with SHA-256 and persisted through `BlobStoragePort` (`adapters/s3/consent_blob_store.go`) with AES-256 metadata and full audit trail recording.
