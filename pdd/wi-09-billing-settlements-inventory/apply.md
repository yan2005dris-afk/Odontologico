# PDD Apply Report — `wi-09-billing-settlements-inventory`

- **Work Item:** `wi-09-billing-settlements-inventory`
- **Parent Feature:** [`pdd/tasks/06-billing-cash-inventory.md`](../tasks/06-billing-cash-inventory.md)
- **Plan Reference:** [`pdd/wi-09-billing-settlements-inventory/plan.md`](./plan.md)
- **Assigned Worktree / Branch:** `wt/billing-stock`
- **Status:** `COMPLETED (GREEN)`

---

## 1. Spec Coverage (`S1..S4`)

| Spec | Requirement | Implementation & Verification | Status |
| :--- | :--- | :--- | :--- |
| **S1** | Dentist Commission Calculation in All 3 SRS Modes (`RF-13`) | `domain.CommissionRule.CalculateCommission` (`internal/settlements/domain/commission_rule.go`) computes exact integer-cent commissions across: (1) `FIXED_PERCENTAGE` ($\text{gross} \times \text{rate\_bps} / 10000$), (2) `FIXED_AMOUNT` ($\text{fixed\_amount\_cents}$), and (3) `NET_PERCENTAGE_AFTER_COSTS` ($\max(0, \text{gross} - \text{lab} - \text{material}) \times \text{rate\_bps} / 10000$), clamping net commission to `0` when deductions exceed gross fee. | Verified (`commission_rule_test.go`, `settlement.service_test.go`) |
| **S2** | Period Dentist Settlement Generation & Duplicate Prevention (`RF-13`) | `POST /api/v1/settlements/generate` aggregates completed procedures for a dentist in `[period_start, period_end]`, resolves procedure-specific or wildcard (`"*"`) commission rules, computes `gross_fees_cents`, `total_lab_deductions_cents`, `total_material_deductions_cents`, and `net_payable_commission_cents`, and rejects any already-settled `completed_procedure_id` with `409 Conflict` (`PROCEDURE_ALREADY_SETTLED`). | Verified (`commission_rule_test.go`, `settlement.service_test.go`, `settlement_handler_test.go`) |
| **S3** | Standard Procedure Recipe & Automatic Stock Deduction (`RF-14`) | `POST /api/v1/inventory/items` and `PUT /api/v1/inventory/recipes/{procedureCode}` manage clinical supplies and Bill of Materials per procedure. `POST /api/v1/inventory/deductions/procedure-completed` atomically deducts each supply in the recipe for `clinic_id`, computes `total_material_cost_cents` for `RF-13`, and flags `low_stock_alert = true` whenever `remaining_stock <= min_stock_quantity`. | Verified (`supply_test.go`, `inventory.service_test.go`, `inventory_handler_test.go`) |
| **S4** | Deduction Idempotency & Insufficient Stock Rollback (`RF-14`) | `ExecuteAtomicProcedureDeduction` (`internal/inventory/adapters/postgres/inventory_repository.go`) keys deductions by `completed_procedure_id` for idempotency (re-sending the same `completed_procedure_id` returns the prior deduction without double-decrementing stock) and stages all supply deductions so any insufficient stock returns `409 Conflict` (`INSUFFICIENT_SUPPLY_STOCK`) with full transactional rollback. | Verified (`supply_test.go`, `inventory.service_test.go`, `inventory_handler_test.go`) |

---

## 2. Files Created / Modified (Strictly within Plan §7)

- `services/billing-stock/migrations/00002_settlements_inventory.sql`
- `services/billing-stock/internal/settlements/domain/commission_rule.go`
- `services/billing-stock/internal/settlements/domain/settlement.go`
- `services/billing-stock/internal/settlements/domain/commission_rule_test.go`
- `services/billing-stock/internal/settlements/application/settlement.port.go`
- `services/billing-stock/internal/settlements/application/settlement.service.go`
- `services/billing-stock/internal/settlements/application/settlement.service_test.go`
- `services/billing-stock/internal/settlements/adapters/postgres/settlement_repository.go`
- `services/billing-stock/internal/settlements/adapters/http/settlement_handler.go`
- `services/billing-stock/internal/settlements/adapters/http/settlement_handler_test.go`
- `services/billing-stock/internal/inventory/domain/supply.go`
- `services/billing-stock/internal/inventory/domain/recipe.go`
- `services/billing-stock/internal/inventory/domain/supply_test.go`
- `services/billing-stock/internal/inventory/application/inventory.port.go`
- `services/billing-stock/internal/inventory/application/inventory.service.go`
- `services/billing-stock/internal/inventory/application/inventory.service_test.go`
- `services/billing-stock/internal/inventory/adapters/postgres/inventory_repository.go`
- `services/billing-stock/internal/inventory/adapters/http/inventory_handler.go`
- `services/billing-stock/internal/inventory/adapters/http/inventory_handler_test.go`
- `services/billing-stock/cmd/api/main.go` (wired `settlements` and `inventory` routes into the service entrypoint)

---

## 3. Executed Commands & Observed Output

### Command
```bash
cd services/billing-stock && go test -v -cover ./... && go vet ./... && go build -o bin/billing-stock ./cmd/api
```

### Real Observed Output
```text
=== RUN   TestInventoryHandler_EndToEnd_S3_S4
--- PASS: TestInventoryHandler_EndToEnd_S3_S4 (0.00s)
PASS
coverage: 85.1% of statements
ok  	github.com/sigo-dental/billing-stock/internal/inventory/adapters/http	0.006s	coverage: 85.1% of statements
	github.com/sigo-dental/billing-stock/internal/inventory/adapters/postgres		coverage: 0.0% of statements
=== RUN   TestInventoryService_AtomicDeductionAndIdempotency_S3_S4
--- PASS: TestInventoryService_AtomicDeductionAndIdempotency_S3_S4 (0.00s)
PASS
coverage: 86.8% of statements
ok  	github.com/sigo-dental/billing-stock/internal/inventory/application	0.005s	coverage: 86.8% of statements
=== RUN   TestSupplyAndRecipe_DeductionAndLowStockAlerts_RF14
--- PASS: TestSupplyAndRecipe_DeductionAndLowStockAlerts_RF14 (0.00s)
PASS
coverage: 84.6% of statements
ok  	github.com/sigo-dental/billing-stock/internal/inventory/domain	0.007s	coverage: 84.6% of statements
=== RUN   TestSettlementHandler_EndToEnd_S1_S2
--- PASS: TestSettlementHandler_EndToEnd_S1_S2 (0.00s)
PASS
coverage: 85.7% of statements
ok  	github.com/sigo-dental/billing-stock/internal/settlements/adapters/http	0.006s	coverage: 85.7% of statements
	github.com/sigo-dental/billing-stock/internal/settlements/adapters/postgres		coverage: 0.0% of statements
=== RUN   TestSettlementService_GenerateAndPreventDuplicate_S1_S2
--- PASS: TestSettlementService_GenerateAndPreventDuplicate_S1_S2 (0.00s)
PASS
coverage: 86.0% of statements
ok  	github.com/sigo-dental/billing-stock/internal/settlements/application	0.005s	coverage: 86.0% of statements
=== RUN   TestCommissionRule_ThreeCalculationModes_RF13_S1
--- PASS: TestCommissionRule_ThreeCalculationModes_RF13_S1 (0.00s)
PASS
coverage: 88.6% of statements
ok  	github.com/sigo-dental/billing-stock/internal/settlements/domain	0.005s	coverage: 88.6% of statements
```

---

## 4. Architectural Decisions & Out-of-Scope Boundaries Respected

- **Hexagonal Pattern B:** Both `internal/settlements` and `internal/inventory` isolate domain rules from application ports (`.port.go` alongside `.service.go`) and adapters (`postgres`, `http`).
- **Exact Integer Arithmetic (`RF-13`, `RF-14`):** All monetary calculations use `int64` cents and integer basis points (`0..10000`), avoiding floating-point drift.
- **Out-of-Scope Respected:** Only files declared in `wi-08` and `wi-09` plans within `services/billing-stock/` and `pdd/` were touched.
