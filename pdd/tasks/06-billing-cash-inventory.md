# Feature 06 — Module 4.5: POS, Electronic Invoicing, Blind Cash Close, Dentist Settlements & Inventory Deduction

- **Feature ID:** `06-billing-cash-inventory`
- **SRS References:** `RF-12`, `RF-13`, `RF-14`, `RN-02`, `RNF-04`, `RNF-05`, `RNF-06`, `SRS §7.1`
- **Status:** `awaiting_plan_approval`
- **Work Items:**
  - [`wi-08-billing-pos-cash`](../wi-08-billing-pos-cash/plan.md) (`wt/billing-stock` — `services/billing-stock/internal/billing`)
  - [`wi-09-billing-settlements-inventory`](../wi-09-billing-settlements-inventory/plan.md) (`wt/billing-stock` — `services/billing-stock/internal/settlements`, `services/billing-stock/internal/inventory`)

---

## 1. Purpose & Scope Boundaries

### In Scope
- Point of Sale (POS) & Electronic Invoicing (`RF-12`, `SRS §7.1`): issuance of tax-compliant electronic invoices (*comprobantes fiscales electrónicos*) via external Fiscal/Tax API port and Payment Gateway port, supporting split/mixed payments (`CASH`, `CREDIT_CARD`, `DEBIT_CARD`, `BANK_TRANSFER`, `DENTAL_INSURANCE`) whose sum must equal the invoice total.
- Non-Negotiable Blind Cash Close (`RN-02`): strict separation between open cash register state and theoretical cash balance. Receptionists/Cashiers are NEVER allowed to query or receive `expected_cash_amount` before submitting their physical cash count (`declared_cash_amount` / denomination breakdown) during daily cash close. Once submitted, the close is sealed and only `admin` and `auditor` (or the sealed close receipt according to role policy) can inspect discrepancy details.
- Dentist Fee & Commission Settlement (`RF-13`): configurable commission rules per dentist and procedure/specialty supporting all 3 SRS modes: (1) `FIXED_PERCENTAGE` on gross fee, (2) `FIXED_AMOUNT` per completed procedure, and (3) `NET_PERCENTAGE_AFTER_COSTS` (percentage applied after deducting laboratory and material costs).
- Clinical Supply Inventory & Automatic Stock Deduction (`RF-14`): catalog of clinical supplies (resins, anesthetics, gloves, endodontic files) by clinic/consultorio with batch/expiry and minimum stock alerts, plus configurable Bill of Materials (*receta estándar de consumo*) per clinical procedure that automatically deducts stock in an atomic ledger transaction when a procedure is completed (`REALIZADO`).

### Out of Scope
- Clinical chart editing (owned by `services/clinical-agenda/`).
- Frontend UI components for POS, blind cash close, settlements, and inventory (covered in Feature `07` / `wi-12-frontend-billing-inventory`).

---

## 2. Numbered Specifications (`S1..S8`)

- **S1 — Mixed-Payment POS & Electronic Fiscal Invoicing (`RF-12`, `SRS §7.1`):** `POST /api/v1/billing/invoices` validates line items, tax breakdown, and `payments[]` across `CASH`, `CREDIT_CARD`, `DEBIT_CARD`, `TRANSFER`, and `INSURANCE` ($\sum \text{payments} = \text{total\_amount}$), invokes the `FiscalStampingPort` to obtain fiscal authorization (`cae_cufe`, `xml_hash`, `stamped_at`), and persists the invoice and cash register movement atomically.
- **S2 — Cash Register Session & Strict Blind Cash Close (`RN-02`):** `GET /api/v1/billing/cash-registers/{id}/status` for role `receptionist` returns session metadata (`opened_at`, `opened_by`, `status = OPEN`) and **strictly omits** `theoretical_cash_total` and `cash_movements_sum`. Attempting to call `/api/v1/billing/cash-registers/{id}/audit-summary` as `receptionist` before closing returns `403 Forbidden` (`BLIND_CLOSE_ENFORCED`).
- **S3 — Blind Cash Count Submission & Discrepancy Calculation (`RN-02`):** `POST /api/v1/billing/cash-registers/{id}/blind-close` requires `declared_cash_amount` (and optional bill/coin denomination breakdown). Only after persisting the immutable `declared_cash_amount` does the service compute $\text{discrepancy} = \text{declared\_cash\_amount} - \text{theoretical\_cash\_amount}$ and transition the cash register to `CLOSED`.
- **S4 — Configurable Dentist Commission Rules (`RF-13`):** `POST /api/v1/settlements/rules` configures a dentist's commission contract with mode:
  - `FIXED_PERCENTAGE`: $\text{commission} = \text{gross\_fee} \times r$
  - `FIXED_AMOUNT`: $\text{commission} = \text{fixed\_amount}$
  - `NET_PERCENTAGE_AFTER_COSTS`: $\text{commission} = \max(0, \text{gross\_fee} - \text{lab\_cost} - \text{material\_cost}) \times r$
- **S5 — Period Settlement Generation & Reconciliation (`RF-13`):** `POST /api/v1/settlements/generate` aggregates all `"REALIZADO"` procedures for a dentist within `[period_start, period_end]`, applies the matching commission rules, deducts applicable lab/material costs, and produces an auditable settlement statement (`DRAFT` $\rightarrow$ `APPROVED` $\rightarrow$ `PAID`).
- **S6 — Clinical Supply Catalog & Procedure Consumption Recipes (`RF-14`):** `POST /api/v1/inventory/items` manages clinical supplies (`sku`, `name`, `unit`, `current_stock`, `min_stock_threshold`, `unit_cost`) per consultorio/warehouse, and `PUT /api/v1/inventory/recipes/{procedureCode}` defines the standard Bill of Materials (`items: [{supply_id, quantity_per_procedure}]`).
- **S7 — Automatic Stock Deduction on Procedure Completion (`RF-14`):** When a clinical procedure is marked `"REALIZADO"` (via `POST /api/v1/inventory/deductions/procedure-completed` or event consumer), the service looks up the standard recipe for `procedure_code`, atomically deducts each supply's quantity from the consultorio's stock, records an immutable `inventory_movements` ledger entry (`PROCEDURE_CONSUMPTION`), computes `material_cost` for `RF-13`, and flags `LOW_STOCK_ALERT` if `current_stock <= min_stock_threshold`.
- **S8 — RBAC, MFA & Financial Audit Trail (`RNF-05`, `RNF-06`):** All endpoints require `mfa_verified=true`; `receptionist` operates POS and blind cash count submission; `auditor` and `admin` inspect cash close discrepancies, settlements, and stock adjustments.

---

## 3. Worktree & Parallelization Strategy

| Work Item | Git Worktree Branch | Target Directory Boundary | Upstream Dependency |
| :--- | :--- | :--- | :--- |
| `wi-08-billing-pos-cash` | `wt/billing-stock` | `services/billing-stock/cmd/**`, `services/billing-stock/internal/billing/**`, `services/billing-stock/migrations/00001_billing_pos_cash.sql` | None (root bootstrap for `billing-stock`) |
| `wi-09-billing-settlements-inventory` | `wt/billing-stock` | `services/billing-stock/internal/settlements/**`, `services/billing-stock/internal/inventory/**`, `services/billing-stock/migrations/00002_settlements_inventory.sql` | `wi-08-billing-pos-cash` |
