# Feature 03 — Module 4.2: Clinical Record, 2D Odontogram, Periodontogram, Immutable SOEP & Digital Consents

- **Feature ID:** `03-clinical-odontogram-soep`
- **SRS References:** `RF-03`, `RF-04`, `RF-05`, `RF-06`, `RN-01`, `RNF-01`, `RNF-04`, `RNF-05`, `RNF-06`
- **Status:** `awaiting_plan_approval`
- **Work Items:**
  - [`wi-04-clinical-odontogram-perio`](../wi-04-clinical-odontogram-perio/plan.md) (`wt/clinical-agenda` — `RF-03`, `RF-04`, `RNF-01`)
  - [`wi-05-clinical-soep-consents`](../wi-05-clinical-soep-consents/plan.md) (`wt/clinical-agenda` — `RF-05`, `RF-06`, `RN-01`)

---

## 1. Purpose & Scope Boundaries

### In Scope
- Interactive 2D Odontogram (`RF-03`): anatomical representation of permanent dentition (32 teeth: FDI `11-18, 21-28, 31-38, 41-48` / Universal `1-32`) and deciduous/temporary dentition (20 teeth: FDI `51-55, 61-65, 71-75, 81-85` / Universal `A-T`), bidirectional FDI $\leftrightarrow$ Universal notation mapping, 5 dental surfaces (`mesial`, `distal`, `oclusal_incisal`, `vestibular`, `lingual_palatino`) plus whole-tooth findings, and standardized visual catalog (`CARIES` = red, `EXISTING_RESTORATION` = blue, `ENDODONTICS_DONE`, `ENDODONTICS_REQUIRED`, `IMPLANT`, `FIXED_PROSTHESIS`, `REMOVABLE_PROSTHESIS`, `EXTRACTION`, `SEALANT`).
- Digital Periodontogram (`RF-04`): 6 probing sites per tooth (`MB`, `B`, `DB`, `ML`, `L`, `DL`), probing depth (`PD` in mm), gingival margin (`GM` in mm), automatic Clinical Attachment Level calculation ($\text{NIC / CAL} = \text{PD} - \text{GM}$), furcation involvement (`0`, `I`, `II`, `III`), tooth mobility (`0`, `1`, `2`, `3`), bleeding on probing (`BOP` boolean per site), and plaque index.
- Immutable SOEP Evolution Notes (`RF-05`, `RN-01`): chronological clinical evolution notes structured into `Subjective`, `Objective`, `Assessment (Evaluación)`, and `Plan` with clinician digital signature hash, automatic lock after a configurable window (default `24h`), strict prohibition on deleting or updating locked notes or procedures marked `"Realizado"` (`RN-01`), and support for cryptographically linked signed Addendum notes (`adenda`).
- Digital Informed Consent (`RF-06`): legal consent template rendering per procedure, biometric signature stroke/image capture or 6-digit patient OTP verification, SHA-256 tamper-evident PDF generation stored in S3/Blob storage (`RNF-04`).

### Out of Scope
- Treatment budget pricing and appointment scheduling (covered in Features `04` and `05`).
- Frontend SVG canvas rendering of the odontogram and periodontogram (covered in Feature `07` / `wi-11-frontend-clinical-workspace`).

---

## 2. Numbered Specifications (`S1..S8`)

- **S1 — Permanent (32) & Deciduous (20) Odontogram with FDI/Universal Dual Notation (`RF-03`):** Domain validator supports all 52 canonical teeth, translates seamlessly between FDI (`11..48`, `51..85`) and Universal (`1..32`, `A..T`), and validates the 5 canonical surfaces (`M`, `D`, `O/I`, `V`, `L/P`) plus `WHOLE_TOOTH`.
- **S2 — Standardized Clinical Finding Catalog & Color Semantics (`RF-03`):** Findings enforce the standardized catalog (`CARIES` [`#DC2626` red], `RESTORATION_EXISTING` [`#2563EB` blue], `ENDODONTICS_DONE`, `ENDODONTICS_REQUIRED`, `IMPLANT`, `PROSTHESIS_FIXED`, `PROSTHESIS_REMOVABLE`, `EXTRACTION_DONE`, `EXTRACTION_INDICATED`, `SEALANT`) and status (`DIAGNOSED`, `PLANNED`, `REALIZADO`).
- **S3 — Immutability of `"Realizado"` Procedures (`RN-01`):** Any odontogram finding or clinical procedure recorded with status `"REALIZADO"` cannot be deleted or overwritten (`409 Conflict` / `CLINICAL_RECORD_IMMUTABLE`). Corrections must create a signed addendum record referencing the original finding ID.
- **S4 — Digital Periodontogram & Automatic NIC/CAL Calculation (`RF-04`):** `POST /api/v1/patients/{id}/periodontograms` validates 6 sites per tooth (`MB, B, DB, ML, L, DL`), computes $\text{NIC}_i = \text{probing\_depth}_i - \text{gingival\_margin}_i$ per site, calculates whole-mouth BOP percentage and mean NIC, and stores furcation (`0..3`) and mobility (`0..3`).
- **S5 — Sub-1.5s Unified Clinical Workspace Payload (`RNF-01`):** `GET /api/v1/patients/{id}/clinical-workspace` returns the active odontogram state, latest periodontogram summary, and sticky patient critical alerts in a single indexed query completing in $< 150\text{ ms}$ backend time (guaranteeing $< 1.5\text{ s}$ end-to-end render on $\ge 15\text{ Mbps}$).
- **S6 — Chronological SOEP Notes with 24h Lock & Digital Signature (`RF-05`, `RN-01`):** `POST /api/v1/patients/{id}/soep-notes` creates a signed SOEP entry (`subjective`, `objective`, `assessment`, `plan`, `signed_by`, `signature_hash`, `lock_at = created_at + lock_window_hours` where default is `24h`). Any `PUT` or `DELETE` after `now() >= lock_at` or once finalized is rejected with `409 Conflict` (`SOEP_NOTE_LOCKED`).
- **S7 — Signed Clinical Addendum (`RN-01`, `RF-05`):** `POST /api/v1/patients/{id}/soep-notes/{noteId}/addenda` appends an immutable addendum linked to `parent_note_id` (or `procedure_id`), requiring the responsible dentist's MFA-verified session and digital signature.
- **S8 — Digital Informed Consent with Biometric Signature or OTP (`RF-06`, `RNF-04`):** `POST /api/v1/patients/{id}/consents` instantiates a legal consent for a procedure; `POST /api/v1/patients/{id}/consents/{consentId}/sign` accepts either a biometric touch signature payload (SVG strokes + pressure/timestamp metadata) or a validated 6-digit patient OTP code, seals the document with a SHA-256 digest, stores the artifact in S3-compatible storage, and logs the action in the audit trail (`RNF-06`).

---

## 3. Worktree & Parallelization Strategy

| Work Item | Git Worktree Branch | Target Directory Boundary | Upstream Dependency |
| :--- | :--- | :--- | :--- |
| `wi-04-clinical-odontogram-perio` | `wt/clinical-agenda` | `services/clinical-agenda/internal/clinical/domain/odontogram*`, `periodontogram*`, `services/clinical-agenda/internal/clinical/application/odontogram*`, `periodontogram*`, `services/clinical-agenda/internal/clinical/adapters/ odontogram & perio`, `migrations/00003_odontogram_perio.sql` | `wi-03-clinical-patients` |
| `wi-05-clinical-soep-consents` | `wt/clinical-agenda` | `services/clinical-agenda/internal/clinical/domain/soep*`, `consent*`, `services/clinical-agenda/internal/clinical/application/soep*`, `consent*`, `services/clinical-agenda/internal/clinical/adapters/ soep & consent`, `migrations/00004_soep_consents.sql` | `wi-03-clinical-patients` |
