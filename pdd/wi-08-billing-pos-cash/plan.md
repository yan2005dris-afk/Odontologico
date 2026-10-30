# Plan de Work Item — `wi-08-billing-pos-cash`

- **Feature Padre:** [`pdd/tasks/06-billing-cash-inventory.md`](../tasks/06-billing-cash-inventory.md)
- **Worktree / Rama Asignada:** `wt/billing-stock`
- **Directorio Objetivo:** `services/billing-stock/cmd/`, `services/billing-stock/internal/billing/`, `services/billing-stock/migrations/00001_billing_pos_cash.sql`
- **Referencias SRS:** `RF-12`, `RN-02`, `RNF-04`, `RNF-05`, `RNF-06`, `SRS §7.1`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Bootstrap del microservicio `services/billing-stock/` (Go 1.24+) siguiendo arquitectura Hexagonal Pattern B (`domain`, `application`, `adapters`).
- Migración SQL inicial `00001_billing_pos_cash.sql` creando las tablas `cash_registers`, `cash_movements`, `invoices`, `invoice_items` y `invoice_payments`.
- Punto de Venta (POS) y Facturación Electrónica (`RF-12`, `SRS §7.1`):
  - Emisión de comprobantes fiscales electrónicos mediante el puerto `FiscalTaxAuthorityPort` (timbrado / CAE / CUFE) y el puerto `PaymentGatewayPort`.
  - Soporte nativo para pagos mixtos (`CASH`, `CREDIT_CARD`, `DEBIT_CARD`, `BANK_TRANSFER`, `DENTAL_INSURANCE`), verificando que la suma exacta de los medios de pago iguale el total facturado ($\sum \text{payments\_cents} == \text{total\_cents}$).
  - Registro automático de los pagos en efectivo (`CASH`) en la sesión de caja activa del cajero/recepcionista.
- Regla de Negocio Innegociable `RN-02` (Cierre de Caja Ciego / *Blind Cash Close*):
  - Mientras una caja está abierta (`OPEN`), ningún endpoint accesible al perfil `receptionist` devuelve jamás el saldo teórico de efectivo del sistema (`theoretical_cash_cents`).
  - El endpoint `POST /api/v1/billing/cash-registers/{id}/blind-close` exige que el recepcionista ingrese primero su arqueo físico en efectivo (`declared_cash_cents` y desglose opcional de billetes/monedas).
  - Recién después de persistir de manera inmutable el conteo físico declarado, el sistema calcula la diferencia ($\text{discrepancy\_cents} = \text{declared\_cash\_cents} - \text{theoretical\_cash\_cents}$) y cierra la caja (`CLOSED`).

### Fuera del alcance (Out of Scope)
- Liquidación de honorarios a profesionales y descargo de insumos clínicos (cubiertos en `wi-09-billing-settlements-inventory`).
- Interfaz de usuario de caja en Next.js (cubierta en `wi-12-frontend-billing-inventory`).

---

## 2. Dependencias previas

- **Work Items previos:** Ninguno (work item raíz para `services/billing-stock/` en el worktree `wt/billing-stock`).
- **Base de datos:** `services/billing-stock/migrations/00001_billing_pos_cash.sql`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Emisión de factura electrónica con pagos mixtos (`RF-12`) | `POST /api/v1/billing/invoices` valida que $\sum \text{payments}[i].\text{amount\_cents} == \text{total\_cents}$ (admitiendo combinaciones como `CASH` + `CREDIT_CARD` + `DENTAL_INSURANCE`), timbra el comprobante vía `FiscalTaxAuthorityPort`, registra el movimiento de efectivo en la caja abierta y retorna el comprobante autorizado con `fiscal_authorization_code`. Si la suma de pagos difiere del total, retorna `422 Unprocessable Entity` (`PAYMENT_SUM_MISMATCH`). | Unit + Integration + Adversarial |
| **S2** | Prohibición de exposición del saldo teórico antes del cierre (`RN-02`) | `GET /api/v1/billing/cash-registers/{id}` invocado mientras `status == "OPEN"` (o por el rol `receptionist`) retorna metadatos de apertura (`id`, `opened_by`, `opened_at`, `opening_float_cents`, `status`) y **jamás incluye** `theoretical_cash_cents` ni la suma de ingresos en efectivo. Intentar consultar `/audit` sobre una caja abierta como `receptionist` retorna `403 Forbidden` (`BLIND_CASH_CLOSE_ENFORCED`). | Unit + Integration + Adversarial |
| **S3** | Ejecución atómica del Cierre de Caja Ciego (`RN-02`) | `POST /api/v1/billing/cash-registers/{id}/blind-close` recibe `declared_cash_cents` ($\ge 0$), sella la caja en estado `CLOSED` impidiendo reintentos (`409 Conflict` / `CASH_REGISTER_ALREADY_CLOSED`), y calcula internamente $\text{theoretical\_cash\_cents} = \text{opening\_float\_cents} + \sum \text{cash\_in\_cents} - \sum \text{cash\_out\_cents}$ y $\text{discrepancy\_cents} = \text{declared\_cash\_cents} - \text{theoretical\_cash\_cents}$ para el reporte de `auditor`/`admin`. | Unit + Integration + Adversarial |
| **S4** | Conciliación para Auditor/Contador (`SRS §3`, `RN-02`) | `GET /api/v1/billing/cash-registers/{id}/audit-report` accesible exclusivamente para `auditor` y `admin` con `mfa_verified=true` expone `opening_float_cents`, `theoretical_cash_cents`, `declared_cash_cents` y `discrepancy_cents`. | Integration + Adversarial |

---

## 4. Contrato de entrada

### `POST /api/v1/billing/invoices`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`receptionist` | `admin`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `patient_id` (`string`, UUID, requerido).
  - `cash_register_id` (`string`, UUID de la caja abierta, requerido).
  - `customer_tax_id` (`string`, CUIT/RUC/NIT/DNI fiscal, requerido).
  - `customer_name` (`string`, requerido).
  - `items`: array no vacío de `{ "procedure_code": string, "description": string, "quantity": int, "unit_price_cents": int64, "tax_rate_bps": int }`.
  - `payments`: array no vacío de `{ "method": "CASH" | "CREDIT_CARD" | "DEBIT_CARD" | "BANK_TRANSFER" | "DENTAL_INSURANCE", "amount_cents": int64, "external_reference": string }`.

### `POST /api/v1/billing/cash-registers/{id}/blind-close`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`receptionist` | `admin`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `declared_cash_cents` (`int64`, $\ge 0$, requerido).
  - `denominations` (`map[string]int`, opcional, ej. `{"1000": 25, "500": 10}`).
  - `cashier_notes` (`string`, opcional).

---

## 5. Contrato de salida

### Respuesta `GET /api/v1/billing/cash-registers/{id}` para Recepcionista (`200 OK` — Cumple `RN-02`)
```json
{
  "id": "ca5b0001-1111-4222-8333-444455556666",
  "clinic_id": "sede-central-uuid",
  "opened_by": "recepcionista-01-uuid",
  "opened_at": "2026-10-30T07:00:00Z",
  "opening_float_cents": 50000,
  "status": "OPEN"
}
```
*(Nota: `theoretical_cash_cents` está ausente por diseño del DTO de recepción).*

### Respuesta `POST /api/v1/billing/cash-registers/{id}/blind-close` para Recepcionista (`200 OK`)
```json
{
  "cash_register_id": "ca5b0001-1111-4222-8333-444455556666",
  "status": "CLOSED",
  "declared_cash_cents": 185000,
  "closed_by": "recepcionista-01-uuid",
  "closed_at": "2026-10-30T21:00:00Z",
  "receipt_hash": "sha256:4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a"
}
```

### Matriz de Estados HTTP
- `200 OK` / `201 Created`: Apertura de caja, emisión de factura electrónica o cierre ciego exitoso.
- `403 Forbidden`: `BLIND_CASH_CLOSE_ENFORCED` (intento de ver saldo teórico antes de cerrar), rol inválido o `MFA_REQUIRED`.
- `404 Not Found`: Caja o factura inexistente.
- `409 Conflict`: `CASH_REGISTER_ALREADY_CLOSED` o intento de facturar en efectivo sobre una caja cerrada.
- `422 Unprocessable Entity`: `PAYMENT_SUM_MISMATCH` o `declared_cash_cents < 0`.

---

## 6. Seguridad y resiliencia

- **Cumplimiento Criptográfico y de Dominio de `RN-02`:** El struct de respuesta `PublicCashRegisterView` para el rol `receptionist` no posee el campo `TheoreticalCashCents` (ni siquiera oculto), haciendo imposible su filtración accidental por serialización JSON. Una vez enviado el `blind-close`, la fila en `cash_registers` se bloquea con `SELECT ... FOR UPDATE` y pasa a `CLOSED`, impidiendo que el cajero modifique su conteo tras enviar la solicitud.
- **Transaccionalidad en Pagos Mixtos (`RF-12`):** Si el timbrado fiscal o el cargo en pasarela falla, se revierte toda la transacción sin alterar el saldo de caja.
- **MFA y RBAC (`RNF-05`):** Toda operación financiera exige `X-MFA-Verified: true`.

---

## 7. Archivos afectados

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
