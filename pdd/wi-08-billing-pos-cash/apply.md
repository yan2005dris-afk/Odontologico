# PDD Apply Report — `wi-08-billing-pos-cash`

- **Work Item:** `wi-08-billing-pos-cash`
- **Parent Feature:** [`pdd/tasks/06-billing-cash-inventory.md`](../tasks/06-billing-cash-inventory.md)
- **Plan Reference:** [`pdd/wi-08-billing-pos-cash/plan.md`](./plan.md)
- **Assigned Worktree / Branch:** `wt/billing-stock`
- **Status:** `COMPLETED (GREEN)`

---

## 1. Spec Coverage (`S1..S4`)

| Spec | Requirement | Implementation & Verification | Status |
| :--- | :--- | :--- | :--- |
| **S1** | Mixed-payment POS & Electronic Fiscal Invoicing (`RF-12`) | `domain.NewInvoice` (`internal/billing/domain/invoice.go`) validates line items, basis-points taxes (`0..10000`), and mixed payments across `CASH`, `CREDIT_CARD`, `DEBIT_CARD`, `BANK_TRANSFER`, and `DENTAL_INSURANCE`, enforcing $\sum \text{payments} == \text{total\_cents}$ (`422 PAYMENT_SUM_MISMATCH` on mismatch). `application.BillingService.IssueInvoice` (`internal/billing/application/billing.service.go`) authorizes non-cash payments via `PaymentGatewayPort`, stamps the electronic invoice via `FiscalTaxAuthorityPort` (`fiscal_authorization_code`, `xml_hash`, `stamped_at`), and records the `CASH` movement in the active cash register. | Verified (`invoice_test.go`, `billing.service_test.go`, `billing_handler_test.go`) |
| **S2** | Strict Blind Cash Close — No Theoretical Cash Exposure Before Close (`RN-02`) | `domain.PublicCashRegisterView` (`internal/billing/domain/cash_register.go`) omits `theoretical_cash_cents` and `discrepancy_cents` at the struct level. `GET /api/v1/billing/cash-registers/{id}` returns only session opening metadata. Calling `GET /api/v1/billing/cash-registers/{id}/audit-report` while `status == "OPEN"` returns `403 Forbidden` (`BLIND_CASH_CLOSE_ENFORCED`). | Verified (`cash_register_test.go`, `billing.service_test.go`, `billing_handler_test.go`) |
| **S3** | Atomic Blind Cash Close Execution (`RN-02`) | `POST /api/v1/billing/cash-registers/{id}/blind-close` requires `declared_cash_cents >= 0`, seals the cash register in `CLOSED` state, computes $\text{theoretical\_cash\_cents} = \text{opening\_float\_cents} + \sum \text{in} - \sum \text{out}$ and $\text{discrepancy\_cents} = \text{declared\_cash\_cents} - \text{theoretical\_cash\_cents}$, and returns a SHA-256 `receipt_hash` without leaking theoretical or discrepancy values to the receptionist. Subsequent close or cash-movement attempts return `409 Conflict` (`CASH_REGISTER_ALREADY_CLOSED`). | Verified (`cash_register_test.go`, `billing.service_test.go`, `billing_handler_test.go`) |
| **S4** | Auditor / Admin Reconciliation Report (`SRS §3`, `RN-02`) | `GET /api/v1/billing/cash-registers/{id}/audit-report` requires `X-MFA-Verified: true` and role `auditor` or `admin` (`403 Forbidden` for `receptionist`), returning `opening_float_cents`, `theoretical_cash_cents`, `declared_cash_cents`, and `discrepancy_cents` once the register is closed. | Verified (`billing.service_test.go`, `billing_handler_test.go`) |

---

## 2. Files Created / Modified (Strictly within Plan §7)

- `services/billing-stock/go.mod`
- `services/billing-stock/cmd/api/main.go`
- `services/billing-stock/migrations/00001_billing_pos_cash.sql`
- `services/billing-stock/internal/billing/domain/invoice.go`
- `services/billing-stock/internal/billing/domain/cash_register.go`
- `services/billing-stock/internal/billing/domain/invoice_test.go`
- `services/billing-stock/internal/billing/domain/cash_register_test.go`
- `services/billing-stock/internal/billing/application/billing.port.go`
- `services/billing-stock/internal/billing/application/billing.service.go`
- `services/billing-stock/internal/billing/application/billing.service_test.go`
- `services/billing-stock/internal/billing/adapters/postgres/billing_repository.go`
- `services/billing-stock/internal/billing/adapters/fiscal/tax_authority_client.go`
- `services/billing-stock/internal/billing/adapters/http/billing_handler.go`
- `services/billing-stock/internal/billing/adapters/http/billing_handler_test.go`

---

## 3. Executed Commands & Observed Output

### Command
```bash
cd services/billing-stock && go test -v -cover ./... && go vet ./... && go build -o bin/billing-stock ./cmd/api
```

### Real Observed Output
```text
	github.com/sigo-dental/billing-stock/cmd/api		coverage: 0.0% of statements
	github.com/sigo-dental/billing-stock/internal/billing/adapters/fiscal		coverage: 0.0% of statements
=== RUN   TestBillingHandler_EndToEndSpecs_S1_S2_S3_S4
--- PASS: TestBillingHandler_EndToEndSpecs_S1_S2_S3_S4 (0.00s)
PASS
coverage: 82.5% of statements
ok  	github.com/sigo-dental/billing-stock/internal/billing/adapters/http	0.007s	coverage: 82.5% of statements
	github.com/sigo-dental/billing-stock/internal/billing/adapters/postgres		coverage: 0.0% of statements
=== RUN   TestBillingService_POSAndBlindCashClose_S1_S2_S3_S4
--- PASS: TestBillingService_POSAndBlindCashClose_S1_S2_S3_S4 (0.00s)
PASS
coverage: 85.4% of statements
ok  	github.com/sigo-dental/billing-stock/internal/billing/application	0.012s	coverage: 85.4% of statements
=== RUN   TestCashRegister_BlindCashCloseInvariants_RN02
--- PASS: TestCashRegister_BlindCashCloseInvariants_RN02 (0.00s)
=== RUN   TestNewInvoice_MixedPaymentValidation_RF12
=== RUN   TestNewInvoice_MixedPaymentValidation_RF12/missing_patient_id
=== RUN   TestNewInvoice_MixedPaymentValidation_RF12/empty_items
=== RUN   TestNewInvoice_MixedPaymentValidation_RF12/invalid_quantity
=== RUN   TestNewInvoice_MixedPaymentValidation_RF12/invalid_tax_rate
=== RUN   TestNewInvoice_MixedPaymentValidation_RF12/unsupported_payment_method
=== RUN   TestNewInvoice_MixedPaymentValidation_RF12/non-positive_payment_amount
--- PASS: TestNewInvoice_MixedPaymentValidation_RF12 (0.00s)
    --- PASS: TestNewInvoice_MixedPaymentValidation_RF12/missing_patient_id (0.00s)
    --- PASS: TestNewInvoice_MixedPaymentValidation_RF12/empty_items (0.00s)
    --- PASS: TestNewInvoice_MixedPaymentValidation_RF12/invalid_quantity (0.00s)
    --- PASS: TestNewInvoice_MixedPaymentValidation_RF12/invalid_tax_rate (0.00s)
    --- PASS: TestNewInvoice_MixedPaymentValidation_RF12/unsupported_payment_method (0.00s)
    --- PASS: TestNewInvoice_MixedPaymentValidation_RF12/non-positive_payment_amount (0.00s)
PASS
coverage: 86.5% of statements
ok  	github.com/sigo-dental/billing-stock/internal/billing/domain	0.005s	coverage: 86.5% of statements
```

---

## 4. Architectural Decisions & Out-of-Scope Boundaries Respected

- **Hexagonal Pattern B:** Domain entities (`Invoice`, `CashRegister`) have zero external dependencies; ports (`billing.port.go`) are colocated alongside `billing.service.go` in `internal/billing/application/`; HTTP, fiscal, and persistence adapters live under `internal/billing/adapters/`.
- **Type-Safe Blind Cash Close (`RN-02`):** `PublicCashRegisterView` and `BlindCloseReceiptView` structurally exclude `TheoreticalCashCents` and `DiscrepancyCents`, preventing any JSON marshaling leak to `receptionist`.
- **Out-of-Scope Respected:** No frontend files or `services/clinical-agenda/` files were touched.
