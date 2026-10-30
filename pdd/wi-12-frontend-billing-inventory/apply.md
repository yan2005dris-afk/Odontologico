# Registro de Implementación (`apply`) — `wi-12-frontend-billing-inventory`

- **Work Item:** [`pdd/wi-12-frontend-billing-inventory/plan.md`](./plan.md)
- **Feature Padre:** [`pdd/tasks/07-frontend-portal-ui.md`](../tasks/07-frontend-portal-ui.md)
- **Worktree / Rama:** `wt/frontend`
- **Estado:** `COMPLETED` (TDD RED $\rightarrow$ GREEN $\rightarrow$ REFACTOR)

---

## 1. Cobertura de Especificaciones (`S1..S4`)

| Spec | Requisito SRS / Regla | Implementación y Evidencia | Estado |
| :--- | :--- | :--- | :--- |
| **S1** | Checkout POS con pagos mixtos (`CASH`, `CREDIT_CARD`, `DEBIT_CARD`, `BANK_TRANSFER`, `DENTAL_INSURANCE`) y validación exacta de suma en centavos (`RF-12`) | Implementado en `frontend/src/components/billing/PosCheckoutSheet.tsx` (`evaluatePosCheckoutBalance`, `renderPosCheckoutSheetHtml`), manteniendo deshabilitado el botón `Emitir Comprobante Fiscal` mientras $\sum \text{payments} \neq \text{invoiceTotal}$ y aplicando `tabular-nums`. | **PASS** |
| **S2** | Pantalla de Cierre de Caja Ciego sin exposición de saldo teórico del sistema (`RN-02`) | Implementado en `frontend/src/components/billing/BlindCashCloseView.tsx` (`calculatePhysicalDenominationTotalCents`, `validateBlindCashCloseSubmission`, `renderBlindCashCloseViewHtml`). Verifica adversarialmente que para `receptionist` en caja `OPEN` jamás se renderice el saldo teórico en el DOM; el reporte de auditoría de descuadre sólo se muestra para `auditor` o `admin` con caja `CLOSED`. | **PASS** |
| **S3** | Configurador y simulador de Liquidación de Honorarios a Profesionales (`RF-13`) | Implementado en `frontend/src/components/billing/DentistSettlementPanel.tsx` (`calculateDentistFeeSettlement`, `renderDentistSettlementPanelHtml`) soportando `FIXED_PERCENTAGE`, `FIXED_AMOUNT` y `NET_PERCENTAGE_AFTER_COSTS` (deduciendo costos de laboratorio y materiales `RF-14`). | **PASS** |
| **S4** | Catálogo de Insumos Clínicos por consultorio, alertas de stock crítico y recetas de consumo automático por procedimiento (`RF-14`) | Implementado en `frontend/src/components/inventory/InventoryWorkspace.tsx` (`evaluateInventoryStockAlerts`, `simulateProcedureRecipeDeduction`, `renderInventoryWorkspaceHtml`). | **PASS** |

---

## 2. Archivos Creados / Modificados

- `frontend/src/components/billing/PosCheckoutSheet.tsx`
- `frontend/src/components/billing/PosCheckoutSheet.test.tsx`
- `frontend/src/components/billing/BlindCashCloseView.tsx`
- `frontend/src/components/billing/BlindCashCloseView.test.tsx`
- `frontend/src/components/billing/DentistSettlementPanel.tsx`
- `frontend/src/components/billing/DentistSettlementPanel.test.tsx`
- `frontend/src/components/inventory/InventoryWorkspace.tsx`
- `frontend/src/components/inventory/InventoryWorkspace.test.tsx`
- `frontend/src/app/(portal)/billing/page.tsx`
- `frontend/src/app/(portal)/settlements/page.tsx`
- `frontend/src/app/(portal)/inventory/page.tsx`
- `frontend/tests/domain-invariants.test.mjs`

---

## 3. Comandos Ejecutados y Salida Real Observada

### 3.1 Suite Unitaria y de Integración (`wi-12`)

Comando:
```bash
cd frontend && npm run test:wi12
```

Salida real:
```text
npm notice run sigo-dental-frontend@0.1.0 test:wi12
npm notice run node --import ./vitest.config.ts --test src/components/billing/PosCheckoutSheet.test.tsx src/components/billing/BlindCashCloseView.test.tsx src/components/billing/DentistSettlementPanel.test.tsx src/components/inventory/InventoryWorkspace.test.tsx
▶ wi-12 S2 — Blind Cash Close UI Strictly Hiding System Theoretical Balance (RN-02)
  ✔ calculates declared physical cash strictly from bill/coin denominations in integer cents (S2) (0.782754ms)
  ✔ NEVER renders or exposes any theoretical system balance in the DOM for receptionist when register is OPEN (RN-02 adversarial test) (0.697714ms)
  ✔ requires explicit cashier confirmation before submitting blind close and allows audit discrepancy view ONLY for auditor or admin (S2, RN-02) (0.325112ms)
✔ wi-12 S2 — Blind Cash Close UI Strictly Hiding System Theoretical Balance (RN-02) (3.849457ms)
▶ wi-12 S3 — Dentist Fee & Commission Settlement Simulator (RF-13, RF-14, AGENTS.md §3)
  ✔ calculates FIXED_PERCENTAGE, FIXED_AMOUNT, and NET_PERCENTAGE_AFTER_COSTS (deducting lab & RF-14 materials) accurately (S3) (0.853206ms)
  ✔ renders settlement breakdown with tabular-nums (S3) (0.443955ms)
✔ wi-12 S3 — Dentist Fee & Commission Settlement Simulator (RF-13, RF-14, AGENTS.md §3) (2.403786ms)
▶ wi-12 S1 — Mixed-Payment POS Checkout & Exact Sum Validation (RF-12, AGENTS.md §3)
  ✔ supports mixed payment methods (CASH, CREDIT_CARD, DEBIT_CARD, BANK_TRANSFER, DENTAL_INSURANCE) and disables issue button until sum === invoiceTotal (S1) (0.947123ms)
  ✔ renders PosCheckoutSheet with tabular-nums and disables 'Emitir Comprobante Fiscal' when balance != 0 (S1) (1.230466ms)
✔ wi-12 S1 — Mixed-Payment POS Checkout & Exact Sum Validation (RF-12, AGENTS.md §3) (3.418265ms)
▶ wi-12 S4 — Clinical Supplies Inventory, Low-Stock Alerts & Procedure Recipes (RF-14)
  ✔ flags low_stock_alert whenever currentStock <= minStockThreshold (S4, RF-14) (0.728061ms)
  ✔ calculates automatic stock deduction and total material cost for a procedure recipe (RF-14) (0.273114ms)
  ✔ renders low-stock warning badge and recipe editor with tabular-nums (S4) (0.621901ms)
✔ wi-12 S4 — Clinical Supplies Inventory, Low-Stock Alerts & Procedure Recipes (RF-14) (3.831042ms)
ℹ tests 10
ℹ suites 4
ℹ pass 10
ℹ fail 0
ℹ duration_ms 122.998758
```

### 3.2 Suite Completa Frontend (`wi-10 + wi-11 + wi-12` — 43 tests en 13 suites)

Comando:
```bash
cd frontend && npm test
```

Salida real:
```text
ℹ tests 43
ℹ suites 13
ℹ pass 43
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 336.253006
```

### 3.3 Suite Consolidada Zero-Dependency (`node --test frontend/tests/domain-invariants.test.mjs`)

Comando:
```bash
node --test frontend/tests/domain-invariants.test.mjs
```

Salida real:
```text
▶ SIGO-Dental Frontend Domain & UI Invariants (wi-10, wi-11, wi-12)
  ✔ wi-10 S1 / RNF-05: enforces 15-minute (900,000 ms) idle session timeout, TOTP MFA, and RBAC sidebar filtering (1.31196ms)
  ✔ wi-10 S2 & S3 / RF-01 & RF-02: enforces Legal Guardian on minors (<18) and renders StickyPatientHeader critical alerts (0.460807ms)
  ✔ wi-10 S4 & S5 / RF-09, RF-10 & RN-03: prevents dental chair overlap even across different dentists and enforces 7-state graph (0.326545ms)
  ✔ wi-11 S1 & S2 / RF-03, RN-01 & RNF-01: renders 52 teeth in <100ms, toggles FDI<->Universal, and locks REALIZADO findings & 24h SOEP notes (1.510674ms)
  ✔ wi-11 S3, S4 & S5 / RF-04, RF-06, RF-07 & RF-08: calculates 6-site NIC = PD - GM, validates biometric/OTP consent, and auto-collapses completed phases (0.511433ms)
  ✔ wi-12 S1, S2, S3 & S4 / RF-12, RN-02, RF-13 & RF-14: enforces exact mixed-payment POS sum, Blind Cash Close hiding theoretical balance, dentist settlements, and inventory alerts (0.997047ms)
✔ SIGO-Dental Frontend Domain & UI Invariants (wi-10, wi-11, wi-12) (6.375533ms)
ℹ tests 6
ℹ suites 1
ℹ pass 6
ℹ fail 0
ℹ duration_ms 136.334609
```

---

## 4. Decisiones Arquitectónicas y Límites de Alcance

- **Garantía de Cierre Ciego (`RN-02`):** `BlindCashCloseView` opera exclusivamente sobre el conteo físico de denominaciones ingresado por el cajero; el contrato de tipos `BlindCashRegisterSessionDTO` omite `theoreticalCashCents` y el renderizador ignora cualquier propiedad espuria en tiempo de ejecución para el rol `receptionist`.
- **Aritmética Entera en Centavos y `tabular-nums` (`AGENTS.md §3`):** Todos los cálculos de pagos mixtos (`RF-12`), arqueo de billetes (`RN-02`), liquidaciones de honorarios (`RF-13`) y costos de recetas de insumos (`RF-14`) se ejecutan en centavos enteros y se formatean con `formatCentsCurrency` en celdas `tabular-nums`.
- **Límites Respetados:** Sólo se crearon y modificaron los archivos dentro de `frontend/` y `pdd/wi-12-frontend-billing-inventory/apply.md`.
