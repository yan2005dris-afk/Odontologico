# Plan de Work Item — `wi-12-frontend-billing-inventory`

- **Feature Padre:** [`pdd/tasks/07-frontend-portal-ui.md`](../tasks/07-frontend-portal-ui.md)
- **Worktree / Rama Asignada:** `wt/frontend`
- **Directorio Objetivo:** `frontend/src/components/billing/`, `frontend/src/components/inventory/`, `frontend/src/app/(portal)/billing/`, `frontend/src/app/(portal)/settlements/`, `frontend/src/app/(portal)/inventory/`
- **Referencias SRS:** `RF-12`, `RF-13`, `RF-14`, `RN-02`, `RNF-05`, `RNF-07`, Interface Architecture (`AGENTS.md §3`)

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Punto de Venta (POS) y Facturación Electrónica (`RF-12`):
  - *Slide-over Sheet* y vista de cobro para emisión de comprobantes fiscales electrónicos.
  - Constructor de pagos mixtos (`Efectivo`, `Tarjeta Crédito`, `Tarjeta Débito`, `Transferencia`, `Seguro/Convenio`) con validación en tiempo real de saldo pendiente por cubrir ($\sum \text{pagos} == \text{total}$) y tipografía `tabular-nums`.
- Interfaz de Cierre de Caja Ciego (`RN-02`):
  - Flujo de arqueo diario diseñado específicamente bajo la regla de negocio `RN-02`: la pantalla de recepción muestra únicamente el formulario de ingreso de conteo físico en efectivo (por monto total o desglose de denominaciones de billetes/monedas) y **jamás renderiza ni solicita el saldo teórico del sistema** antes de confirmar el cierre.
  - Vista de auditoría de cortes de caja (exclusiva para roles `auditor` y `admin`) que exhibe el comparativo entre saldo teórico, efectivo físico declarado y diferencia/descuadre.
- Liquidación de Honorarios a Profesionales (`RF-13`):
  - Panel de configuración de reglas de comisión (porcentaje fijo, monto fijo por procedimiento o porcentaje neto tras deducir laboratorio y materiales) y tabla de liquidaciones por período.
- Inventario de Insumos Clínicos y Recetas de Consumo (`RF-14`):
  - Vista de stock por consultorio con alertas visuales de stock crítico (`low_stock_alert`), editor de recetas estándar de consumo por procedimiento clínico y bitácora de descargos automáticos.

### Fuera del alcance (Out of Scope)
- App Shell, Pacientes y Agenda (cubiertos en `wi-10-frontend-shell-patients-agenda`).
- Odontograma, Periodontograma, SOEP y Presupuestos (cubiertos en `wi-11-frontend-clinical-workspace`).

---

## 2. Dependencias previas

- **Work Items previos:** `wi-10-frontend-shell-patients-agenda` (componentes base de shell, `SlideOverSheet` y cliente API). Contratos backend de `wi-08-billing-pos-cash` y `wi-09-billing-settlements-inventory`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Checkout POS con pagos mixtos y validación exacta de suma (`RF-12`) | `PosCheckoutSheet` permite agregar múltiples líneas de pago (`CASH`, `CREDIT_CARD`, `DEBIT_CARD`, `BANK_TRANSFER`, `DENTAL_INSURANCE`), muestra el remanente en centavos con `tabular-nums`, y mantiene deshabilitado el botón `Emitir Comprobante Fiscal` mientras $\sum \text{payments} \neq \text{invoiceTotal}$. | Unit + Integration + Adversarial |
| **S2** | Pantalla de Cierre de Caja Ciego sin exposición de saldo teórico (`RN-02`) | `BlindCashCloseView` renderizado para el rol `receptionist` con caja en estado `OPEN` no contiene en el DOM ningún monto teórico ni esperado del sistema; calcula el total declarado únicamente a partir del conteo físico ingresado por el cajero y exige confirmación antes de enviar `POST .../blind-close`. | Unit + Integration + Adversarial |
| **S3** | Configurador y simulador de Liquidación de Honorarios (`RF-13`) | `DentistSettlementPanel` permite seleccionar entre `FIXED_PERCENTAGE`, `FIXED_AMOUNT` y `NET_PERCENTAGE_AFTER_COSTS`, mostrando el desglose de honorario bruto, deducción de laboratorio, deducción de materiales (`RF-14`) y neto a liquidar con `tabular-nums`. | Unit + Integration |
| **S4** | Catálogo de Insumos Clínicos, alertas de stock bajo y recetas por procedimiento (`RF-14`) | `InventoryWorkspace` destaca con badge de advertencia todo insumo donde `currentStock <= minStockThreshold` y permite editar la receta estándar de insumos por código de procedimiento. | Unit + Integration |

---

## 4. Contrato de entrada

### Props de Componentes Financieros y de Inventario
- `PosCheckoutSheetProps`:
  - `patientId: string`, `cashRegisterId: string`, `items: InvoiceLineItemInput[]`, `onSubmitInvoice: (payload: CreateInvoiceRequest) => Promise<InvoiceResponse>`.
- `BlindCashCloseViewProps`:
  - `cashRegister: { id: string; openedBy: string; openedAt: string; status: 'OPEN' | 'CLOSED' }` (sin `theoreticalCashCents` por contrato `RN-02`), `userRole: UserRole`, `onSubmitBlindClose: (declaredCashCents: number, denominations: Record<string, number>) => Promise<void>`.

---

## 5. Contrato de salida

### Peticiones Emitidas al API Gateway
- `POST /api/v1/billing/invoices` (emisión de comprobante electrónico con pagos mixtos).
- `POST /api/v1/billing/cash-registers/{id}/blind-close` (envío de arqueo físico ciego).
- `GET /api/v1/billing/cash-registers/{id}/audit-report` (sólo si `userRole === 'auditor' || userRole === 'admin'`).
- `POST /api/v1/settlements/rules` y `POST /api/v1/settlements/generate`.
- `PUT /api/v1/inventory/recipes/{procedureCode}` y `GET /api/v1/inventory/items`.

---

## 6. Seguridad y resiliencia

- **Garantía de Cierre Ciego en Cliente (`RN-02`):** El componente `BlindCashCloseView` verifica que la sesión de caja no incluya saldos teóricos y deshabilita la edición una vez enviado el conteo físico, previniendo tanteos o envíos duplicados (idempotencia de botón en estado `submitting`).
- **Precisión Numérica y Tipográfica (`AGENTS.md §3`):** Todas las tablas financieras, arqueos de billetes, deducciones de materiales y comisiones aplican `tabular-nums` y operan internamente en centavos enteros (`number` entero) antes de formatear para visualización.

---

## 7. Archivos afectados

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
