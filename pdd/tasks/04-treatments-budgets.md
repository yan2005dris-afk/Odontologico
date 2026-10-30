# Feature 04 — Module 4.3: Multi-Phase Treatment Plans, Budgets & Transition to Schedulable Tasks

- **Feature ID:** `04-treatments-budgets`
- **SRS References:** `RF-07`, `RF-08`, `RN-01`, `RNF-05`, `RNF-06`
- **Status:** `awaiting_plan_approval`
- **Work Items:**
  - [`wi-06-clinical-treatments`](../wi-06-clinical-treatments/plan.md) (`wt/clinical-agenda` — `services/clinical-agenda/internal/treatments`)

---

## 1. Purpose & Scope Boundaries

### In Scope
- Clinical Procedure Catalog & Multi-level Treatment Plans / Budgets (`RF-07`): grouping clinical procedures by phases/sessions (e.g., Phase 1: Urgency/Hygiene, Phase 2: Restorative/Endodontics, Phase 3: Rehabilitation/Implants), computing professional fees (`honorarios`), laboratory costs (`costo_laboratorio`), authorized discounts (percentage or fixed within role limits), tax/coverage adjustments, and exact phase & grand totals.
- Partial or Total Budget Approval & Transition to Schedulable Tasks (`RF-08`): recording patient approval of the entire budget or a subset of specific phases/items, automatically transitioning every approved item into a `SCHEDULABLE` clinical task (`schedulable_tasks`) ready to be linked to an appointment in Module 4.4 (`scheduling`) and billed in Module 4.5 (`billing`).
- Protection of completed (`REALIZADO`) treatment items against deletion (`RN-01`).

### Out of Scope
- Calendar slot booking and chair overlap checks (covered in Feature `05` / `wi-07-clinical-scheduling`).
- Fiscal invoice emission and payment collection (covered in Feature `06` / `wi-08-billing-pos-cash`).

---

## 2. Numbered Specifications (`S1..S6`)

- **S1 — Clinical Procedure & Tariff Catalog (`RF-07`):** CRUD and lookup for standardized dental procedures (`code`, `name`, `specialty`, `base_fee`, `default_lab_cost`, `estimated_duration_minutes`, `requires_informed_consent`).
- **S2 — Multi-Phase Budget Calculation (`RF-07`):** `POST /api/v1/patients/{id}/treatment-plans` builds a treatment plan organized into ordered `phases[]` and `items[]` (referencing tooth FDI/Universal number and surface where applicable). For each item: $\text{net\_total} = (\text{professional\_fee} + \text{lab\_cost}) - \text{authorized\_discount}$. Discounts exceeding the clinician's threshold require `admin` authorization code.
- **S3 — Total & Partial Budget Approval (`RF-08`):** `POST /api/v1/treatment-plans/{planId}/approve` accepts `approval_mode` (`TOTAL` | `PARTIAL`) and `approved_item_ids[]` (plus optional consent/signature reference). The plan status transitions from `DRAFT`/`PRESENTED` to `APPROVED_TOTAL` or `APPROVED_PARTIAL`, while unapproved items remain `PENDING_APPROVAL` or `REJECTED`.
- **S4 — Automatic Generation of Schedulable Tasks (`RF-08`):** Upon total or partial approval, each newly approved treatment item atomically generates a `SchedulableTask` record (`task_id`, `patient_id`, `plan_id`, `phase_number`, `treatment_item_id`, `procedure_code`, `tooth_fdi`, `estimated_duration_minutes`, `status = READY_TO_SCHEDULE`) inside the same PostgreSQL transaction.
- **S5 — Procedure Completion Lifecycle & Immutability (`RN-01`, `RF-14`):** `POST /api/v1/treatment-plans/{planId}/items/{itemId}/complete` marks an item as `"REALIZADO"` with the executing dentist ID and emits a domain event (`clinical.procedure.completed`) carrying `procedure_code`, `dentist_id`, `patient_id`, `professional_fee`, and `lab_cost` for downstream fee settlement (`RF-13`) and stock deduction (`RF-14`). Once marked `"REALIZADO"`, any attempt to delete or revert the item fails with `409 Conflict` (`RN-01`).
- **S6 — Audit Trail & RBAC (`RNF-05`, `RNF-06`):** Every creation, discount authorization, approval, and completion of a treatment plan logs an immutable entry in `audit_logs`.

---

## 3. Worktree & Parallelization Strategy

| Work Item | Git Worktree Branch | Target Directory Boundary | Upstream Dependency |
| :--- | :--- | :--- | :--- |
| `wi-06-clinical-treatments` | `wt/clinical-agenda` | `services/clinical-agenda/internal/treatments/**`, `services/clinical-agenda/migrations/00005_treatments_budgets.sql` | `wi-03-clinical-patients`, `wi-04-clinical-odontogram-perio` |
