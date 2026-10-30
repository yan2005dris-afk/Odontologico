# Plan de Work Item — `wi-09-billing-settlements-inventory`

- **Feature Padre:** [`pdd/tasks/06-billing-cash-inventory.md`](../tasks/06-billing-cash-inventory.md)
- **Worktree / Rama Asignada:** `wt/billing-stock`
- **Directorio Objetivo:** `services/billing-stock/internal/settlements/`, `services/billing-stock/internal/inventory/`, `services/billing-stock/migrations/00002_settlements_inventory.sql`
- **Referencias SRS:** `RF-13`, `RF-14`, `RNF-05`, `RNF-06`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Implementación de los contextos `settlements` (`RF-13`) e `inventory` (`RF-14`) en `services/billing-stock/` bajo arquitectura Hexagonal Pattern B.
- Migración SQL `00002_settlements_inventory.sql` creando las tablas `commission_rules`, `dentist_settlements`, `dentist_settlement_items`, `inventory_supplies`, `procedure_supply_recipes` e `inventory_movements`.
- Liquidación de Honorarios a Profesionales (`RF-13`):
  - Configuración de reglas de comisión por odontólogo y especialidad/procedimiento soportando las 3 modalidades del SRS:
    1. `FIXED_PERCENTAGE`: porcentaje fijo sobre el honorario bruto ($\text{gross\_fee\_cents} \times \text{rate\_bps} / 10000$).
    2. `FIXED_AMOUNT`: monto fijo en centavos por procedimiento realizado.
    3. `NET_PERCENTAGE_AFTER_COSTS`: porcentaje aplicado tras deducir costos de laboratorio y de materiales ($\max(0, \text{gross\_fee\_cents} - \text{lab\_cost\_cents} - \text{material\_cost\_cents}) \times \text{rate\_bps} / 10000$).
  - Generación de liquidación por período (`[period_start, period_end]`) para conciliación por `auditor` / `admin`.
- Descargo Automático de Insumos Clínicos (`RF-14`):
  - Catálogo de insumos y materiales clínicos por consultorio/sede (resinas, anestesias, guantes, limas) con control de lote, stock actual y umbral de alerta de stock mínimo (`min_stock_quantity`).
  - Configuración de la receta estándar de consumo (*Bill of Materials*) por código de procedimiento clínico (`procedure_code`).
  - Deducción automática e idempotente de stock al completarse un procedimiento clínico (`REALIZADO`), registrando el movimiento en el kárdex (`inventory_movements`), calculando el costo de materiales (`material_cost_cents`) consumido para `RF-13` y activando alerta cuando `current_stock <= min_stock_quantity`.

### Fuera del alcance (Out of Scope)
- Punto de venta y cierre de caja ciego (cubiertos en `wi-08-billing-pos-cash`).
- Pantallas de liquidaciones e inventario en Next.js (cubiertas en `wi-12-frontend-billing-inventory`).

---

## 2. Dependencias previas

- **Work Items previos:** `wi-08-billing-pos-cash` (bootstrap de `services/billing-stock/` y migración `00001`).
- **Migraciones previas:** `00001_billing_pos_cash.sql`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Cálculo de comisiones en las 3 modalidades del SRS (`RF-13`) | `CalculateCommission` evalúa exactamente: (a) `FIXED_PERCENTAGE` ($100000 \times 40\% = 40000$), (b) `FIXED_AMOUNT` ($25000$), y (c) `NET_PERCENTAGE_AFTER_COSTS` (si bruto $= 100000$, lab $= 20000$, materiales $= 10000$, tasa $= 50\% \Rightarrow (100000 - 30000) \times 50\% = 35000$). Si los costos superan el honorario bruto, la comisión neta es `0` (nunca negativa). | Unit + Adversarial |
| **S2** | Generación de liquidación periódica por odontólogo (`RF-13`) | `POST /api/v1/settlements/generate` agrupa procedimientos completados del odontólogo en el rango de fechas, calcula el total bruto, deducciones de laboratorio/materiales y honorario neto a pagar, evitando liquidar dos veces el mismo `completed_procedure_id` (`409 Conflict` / `PROCEDURE_ALREADY_SETTLED`). | Unit + Integration + Adversarial |
| **S3** | Receta estándar de consumo y descargo automático de stock (`RF-14`) | `POST /api/v1/inventory/deductions/procedure-completed` busca la receta estándar para `procedure_code`, descuenta atómicamente cada insumo (ej. 1 cartucho de anestesia, 2 pares de guantes, 0.5g de resina) del `clinic_id`, inserta las filas en `inventory_movements` y retorna `total_material_cost_cents` y `low_stock_alerts[]`. | Unit + Integration |
| **S4** | Idempotencia de descargo y protección contra stock insuficiente (`RF-14`) | Enviar dos veces el mismo `completed_procedure_id` no duplica el descargo de stock (retorna el descargo previo por clave de idempotencia). Si un insumo no tiene stock suficiente y la política estricta está activa, retorna `409 Conflict` (`INSUFFICIENT_SUPPLY_STOCK`) con rollback total de la transacción. | Unit + Integration + Adversarial |

---

## 4. Contrato de entrada

### `POST /api/v1/settlements/rules`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`admin` | `auditor`), `X-MFA-Verified: true`.
- Body JSON:
  - `dentist_id` (`string`, UUID, requerido).
  - `procedure_code` (`string`, `"*"` para regla general o código específico, requerido).
  - `calculation_mode` (`string`: `"FIXED_PERCENTAGE"` | `"FIXED_AMOUNT"` | `"NET_PERCENTAGE_AFTER_COSTS"`, requerido).
  - `rate_bps` (`int`, `0..10000` donde `4000` = 40.00%, requerido en modos porcentuales).
  - `fixed_amount_cents` (`int64`, $\ge 0$, requerido en modo `FIXED_AMOUNT`).

### `POST /api/v1/inventory/deductions/procedure-completed`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist` | `assistant` | `admin`), `X-MFA-Verified: true`.
- Body JSON:
  - `completed_procedure_id` (`string`, UUID único del procedimiento realizado, requerido como clave de idempotencia).
  - `clinic_id` (`string`, UUID del consultorio/sede, requerido).
  - `dentist_id` (`string`, UUID, requerido).
  - `procedure_code` (`string`, requerido).

---

## 5. Contrato de salida

### Respuesta `POST /api/v1/inventory/deductions/procedure-completed` (`200 OK`)
```json
{
  "completed_procedure_id": "55667788-1122-4333-8445-667788990011",
  "procedure_code": "REST-RESINA-1S",
  "total_material_cost_cents": 3200,
  "deducted_items": [
    { "supply_sku": "ANES-LIDO-2", "quantity_deducted": 1, "remaining_stock": 48, "low_stock_alert": false },
    { "supply_sku": "RESINA-A2-3M", "quantity_deducted": 1, "remaining_stock": 3, "low_stock_alert": true }
  ]
}
```

### Respuesta `POST /api/v1/settlements/generate` (`201 Created`)
```json
{
  "settlement_id": "77889900-aabb-4ccd-8eef-112233445566",
  "dentist_id": "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
  "period_start": "2026-10-01",
  "period_end": "2026-10-31",
  "gross_fees_cents": 500000,
  "total_lab_deductions_cents": 80000,
  "total_material_deductions_cents": 20000,
  "net_payable_commission_cents": 200000,
  "status": "DRAFT"
}
```

### Matriz de Estados HTTP
- `200 OK` / `201 Created`: Regla creada, descargo ejecutado o liquidación generada.
- `403 Forbidden`: Rol sin permiso o `MFA_REQUIRED`.
- `404 Not Found`: Receta o insumo inexistente.
- `409 Conflict`: `INSUFFICIENT_SUPPLY_STOCK` o `PROCEDURE_ALREADY_SETTLED`.
- `422 Unprocessable Entity`: `rate_bps` fuera de `0..10000` o cantidades $\le 0$.

---

## 6. Seguridad y resiliencia

- **Transaccionalidad en Kárdex de Inventario (`RF-14`):** El descuento de cada insumo de la receta utiliza bloqueo de fila (`SELECT ... FOR UPDATE` ordenando por `supply_id` ascendente para evitar deadlocks) y registra un movimiento inmutable en `inventory_movements`.
- **Exactitud Financiera (`RF-13`):** Cálculo íntegro en centavos (`int64`) y puntos básicos (`1 bp = 0.01%`), evitando pérdida de precisión en liquidaciones médicas.
- **Control RBAC (`SRS §3`, `RNF-05`):** Sólo `admin` y `auditor` pueden configurar reglas de comisión y aprobar liquidaciones de honorarios; el descargo automático ocurre al completar la atención clínica.

---

## 7. Archivos afectados

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
