# Feature 02 — Module 4.1: Unified Patient Record & Anamnesis Alerts

- **Feature ID:** `02-patients-anamnesis`
- **SRS References:** `RF-01`, `RF-02`, `RNF-01`, `RNF-05`, `RNF-06`
- **Status:** `awaiting_plan_approval`
- **Work Items:**
  - [`wi-03-clinical-patients`](../wi-03-clinical-patients/plan.md) (`wt/clinical-agenda` — `services/clinical-agenda/internal/patients`)

---

## 1. Purpose & Scope Boundaries

### In Scope
- Unified Patient Record (`RF-01`): creation, retrieval, search, and update of patient demographic data, national ID/document uniqueness, emergency contact, mandatory legal guardian (`tutor_legal`) validation when the patient is a minor ($< 18$ years old at registration/consultation), and dental insurance/agreement (`convenio`) metadata.
- Medical History / Anamnesis & High-Visibility Critical Alerts (`RF-02`): structured capture of systemic diseases, drug allergies, surgical history, and coagulopathies/anticoagulant therapy. Automatic derivation of high-visibility critical alert badges (`CRITICAL_ALLERGY`, `ANTICOAGULANT`, `COAGULOPATHY`, `SYSTEMIC_HIGH_RISK`, `IMMUNOCOMPROMISED`) returned in the patient summary header payload within $< 1.5\text{ s}$ (`RNF-01`).
- Automatic audit trail emission (`RNF-06`) on every create, read, update, or export of patient or anamnesis records.

### Out of Scope
- Odontogram 2D, periodontogram, SOEP notes, and consents (covered in Feature `03`).
- Frontend UI components for patient directory and sticky header badges (covered in Feature `07` / `wi-10-frontend-shell-patients-agenda`).

---

## 2. Numbered Specifications (`S1..S6`)

- **S1 — Unified Patient Registration & Minor Guardian Rule (`RF-01`):** `POST /api/v1/patients` registers a unified patient record with full name, document type/number (unique), date of birth, biological sex/gender, contact info, emergency contact (name, phone, relationship), and optional dental insurance (`provider`, `policy_number`, `coverage_plan`). If age $< 18$, `legal_guardian` (`full_name`, `document_number`, `phone`, `relationship`) is strictly required or the request fails with `422 Unprocessable Entity` (`LEGAL_GUARDIAN_REQUIRED_FOR_MINOR`).
- **S2 — Patient Consultation, Update & Fast Directory Search (`RF-01`, `RNF-01`):** `GET /api/v1/patients/{id}`, `PUT /api/v1/patients/{id}`, and `GET /api/v1/patients?q=<search>&limit=&offset=` allow sub-100ms indexed lookup by document number, full name (trigram/prefix), or medical record number (`hc_number`).
- **S3 — Mandatory Anamnesis Recording (`RF-02`):** `PUT /api/v1/patients/{id}/anamnesis` records a versioned medical history snapshot containing `systemic_diseases[]`, `drug_allergies[]` (allergen, severity, reaction), `surgical_history[]`, `coagulopathies[]`, `current_medications[]`, `pregnancy_status`, and `smoking_alcohol_notes`, signed by the recording clinician (`dentist` or `assistant`).
- **S4 — High-Visibility Critical Alert Derivation (`RF-02`):** Saving an anamnesis automatically evaluates clinical rules and populates `critical_alerts[]` on the patient header (e.g., Penicillin/NSAID/Latex anaphylactic allergy, Warfarin/Rivaroxaban/Aspirin anticoagulant use, Hemophilia/Von Willebrand coagulopathy, uncontrolled hypertension/diabetes, bisphosphonates, endocarditis prophylaxis). `GET /api/v1/patients/{id}/summary` always returns `critical_alerts[]` with `code`, `label`, `severity` (`CRITICAL` | `HIGH`), and `clinical_warning` so the sticky header never requires a secondary round-trip.
- **S5 — Clinical Audit Trail Integration (`RNF-06`):** Every `CREATE`, `READ`, and `UPDATE` on `patients` and `patient_anamnesis` invokes the `audit.Recorder` port with the actor ID, role, source IP, and resource ID within the same unit of work.
- **S6 — RBAC & MFA Protection (`SRS §3`, `RNF-05`):** `receptionist`, `dentist`, `assistant`, and `admin` may read/write demographic records (`RF-01`); only `dentist` and `assistant` (with `mfa_verified=true`) may mutate `anamnesis` (`RF-02`); `auditor` has read-only access.

---

## 3. Worktree & Parallelization Strategy

| Work Item | Git Worktree Branch | Target Directory Boundary | Upstream Dependency |
| :--- | :--- | :--- | :--- |
| `wi-03-clinical-patients` | `wt/clinical-agenda` | `services/clinical-agenda/internal/patients/**`, `services/clinical-agenda/migrations/00002_patients_anamnesis.sql` | `wi-02-clinical-identity-audit` |
