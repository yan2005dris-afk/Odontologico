# PDD Apply Report — `wi-06-clinical-treatments`

- **Work Item:** `wi-06-clinical-treatments`
- **Feature Spec:** [`pdd/tasks/04-treatments-budgets.md`](../tasks/04-treatments-budgets.md)
- **Plan Reference:** [`pdd/wi-06-clinical-treatments/plan.md`](./plan.md)
- **Worktree / Branch:** `wt/clinical-agenda`
- **Status:** `COMPLETED (GREEN)`

---

## 1. Spec Coverage Matrix (`S1..S5`)

| Spec | Requirement Summary | Verification & Test Coverage | Status |
| :--- | :--- | :--- | :--- |
| **S1** | Multi-phase treatment budget calculation in exact `int64` cents ($\text{net\_cents} = \text{professional\_fee\_cents} + \text{lab\_cost\_cents} - \text{discount\_cents}$) with role discount threshold (`RF-07`) | `TestTreatmentPlan_MultiPhaseCalculationApprovalAndRN01`, `TestTreatmentService_ApprovalAndSchedulableTasks`, `TestTreatmentHandler_Endpoints` | PASS |
| **S2** | Partial budget approval (`mode: "PARTIAL"`, `approved_item_ids[]`) atomically transitioning approved items to `schedulable_tasks` (`READY_TO_SCHEDULE`) while unapproved items generate no tasks (`RF-08`) | `TestTreatmentPlan_MultiPhaseCalculationApprovalAndRN01`, `TestTreatmentService_ApprovalAndSchedulableTasks`, `TestTreatmentHandler_Endpoints` | PASS |
| **S3** | Total budget approval (`mode: "TOTAL"`) transitioning all remaining items to `APPROVED` and generating `schedulable_tasks` without duplicating previously generated tasks (`RF-08`) | `TestTreatmentPlan_MultiPhaseCalculationApprovalAndRN01` | PASS |
| **S4** | Immutability of treatment items marked `"REALIZADO"` (`POST .../complete`) blocking subsequent `DELETE` or reversal with `409 CLINICAL_RECORD_IMMUTABLE` (`RN-01`) | `TestTreatmentPlan_MultiPhaseCalculationApprovalAndRN01`, `TestTreatmentService_ApprovalAndSchedulableTasks`, `TestTreatmentHandler_Endpoints` | PASS |
| **S5** | Querying patient schedulable tasks (`GET /api/v1/patients/{id}/schedulable-tasks`) and recording every budget mutation in `audit_logs` (`RF-08`, `RNF-06`) | `TestTreatmentService_ApprovalAndSchedulableTasks`, `TestTreatmentHandler_Endpoints` | PASS |

---

## 2. Files Created / Modified

- `services/clinical-agenda/migrations/00005_treatments_budgets.sql`
- `services/clinical-agenda/internal/treatments/domain/treatment_plan.go`
- `services/clinical-agenda/internal/treatments/domain/schedulable_task.go`
- `services/clinical-agenda/internal/treatments/domain/treatment_plan_test.go`
- `services/clinical-agenda/internal/treatments/application/treatment.port.go`
- `services/clinical-agenda/internal/treatments/application/treatment.service.go`
- `services/clinical-agenda/internal/treatments/application/treatment.service_test.go`
- `services/clinical-agenda/internal/treatments/adapters/postgres/treatment_repository.go`
- `services/clinical-agenda/internal/treatments/adapters/http/treatment_handler.go`
- `services/clinical-agenda/internal/treatments/adapters/http/treatment_handler_test.go`

---

## 3. Executed Commands & Real Observed Output

```bash
cd services/clinical-agenda && go test -count=1 -v -cover ./... && go vet ./... && go build -o bin/clinical-agenda ./cmd/api
```

```text
=== RUN   TestTreatmentHandler_Endpoints
--- PASS: TestTreatmentHandler_Endpoints (0.00s)
PASS
coverage: 92.9% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/treatments/adapters/http	0.031s	coverage: 92.9% of statements
=== RUN   TestTreatmentService_ApprovalAndSchedulableTasks
--- PASS: TestTreatmentService_ApprovalAndSchedulableTasks (0.00s)
PASS
coverage: 84.2% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/treatments/application	0.025s	coverage: 84.2% of statements
=== RUN   TestTreatmentPlan_MultiPhaseCalculationApprovalAndRN01
--- PASS: TestTreatmentPlan_MultiPhaseCalculationApprovalAndRN01 (0.00s)
PASS
coverage: 85.7% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/treatments/domain	0.027s	coverage: 85.7% of statements
```

---

## 4. Architectural Decisions & Out-of-Scope Boundaries

- **Exact Integer Monetary Arithmetic:** All fees, lab costs, discounts, phase subtotals, and plan totals use `int64` cents (`BIGINT` in SQL) to eliminate IEEE-754 floating-point drift.
- **Transactional Task Generation (`RF-08`):** `SavePlanApprovalTx` persists the updated plan state and newly generated `SchedulableTask` records atomically.
