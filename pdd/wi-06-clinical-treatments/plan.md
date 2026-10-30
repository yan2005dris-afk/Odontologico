# Plan de Work Item — `wi-06-clinical-treatments`

- **Feature Padre:** [`pdd/tasks/04-treatments-budgets.md`](../tasks/04-treatments-budgets.md)
- **Worktree / Rama Asignada:** `wt/clinical-agenda`
- **Directorio Objetivo:** `services/clinical-agenda/internal/treatments/`, `services/clinical-agenda/migrations/00005_treatments_budgets.sql`
- **Referencias SRS:** `RF-07`, `RF-08`, `RN-01`, `RNF-05`, `RNF-06`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Implementación del contexto `treatments` en `services/clinical-agenda/internal/treatments/` bajo arquitectura Hexagonal Pattern B.
- Migración SQL `00005_treatments_budgets.sql` creando `procedure_catalog`, `treatment_plans`, `treatment_phases`, `treatment_items` y `schedulable_tasks`.
- Generación de Presupuestos Multinivel (`RF-07`):
  - Armado de planes de tratamiento divididos en fases/sesiones ordenadas (`phase_number`, `title`).
  - Cálculo exacto en centavos/enteros (`int64`) de honorarios profesionales (`professional_fee_cents`), costos de laboratorio (`lab_cost_cents`), descuentos autorizados (`discount_cents` o porcentaje con tope por rol) y subtotales por fase y total general.
- Aprobación Total o Parcial y Transición a Tareas Agendables (`RF-08`):
  - Registro de aprobación total (`TOTAL`) o parcial (`PARTIAL`, seleccionando un subconjunto de `treatment_item_ids[]`) por parte del paciente.
  - Conversión automática y transaccional de cada ítem aprobado en una tarea agendable (`schedulable_tasks` con estado `READY_TO_SCHEDULE`) consumible por el módulo de Agenda (`RF-09`).
- Ejecución de procedimiento y cumplimiento de `RN-01`: al marcar un ítem como `"REALIZADO"`, el ítem queda bloqueado contra eliminación o edición destructiva y publica los datos necesarios para liquidación de honorarios (`RF-13`) y descargo de insumos (`RF-14`).

### Fuera del alcance (Out of Scope)
- Asignación de sillón y horario en calendario (cubierto en `wi-07-clinical-scheduling`).
- Emisión de comprobantes fiscales de pago (cubierto en `wi-08-billing-pos-cash`).

---

## 2. Dependencias previas

- **Work Items previos:** `wi-02-clinical-identity-audit`, `wi-03-clinical-patients`, `wi-04-clinical-odontogram-perio`.
- **Migraciones previas:** `00001` a `00004` en `services/clinical-agenda/migrations/`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Cálculo de presupuesto multinivel por fases (`RF-07`) | `POST /api/v1/patients/{id}/treatment-plans` valida fases e ítems, calcula por cada ítem $\text{net\_cents} = (\text{professional\_fee\_cents} + \text{lab\_cost\_cents}) - \text{discount\_cents}$, verifica que $\text{discount\_cents} \le \text{max\_allowed\_discount}$, y calcula totales por fase y del presupuesto completo sin errores de punto flotante. | Unit + Integration + Adversarial |
| **S2** | Aprobación total o parcial y creación de tareas agendables (`RF-08`) | `POST /api/v1/treatment-plans/{planId}/approve` con `mode: "PARTIAL"` y `approved_item_ids` marca únicamente esos ítems como `APPROVED`, cambia el plan a `APPROVED_PARTIAL` y crea exactamente un registro `schedulable_tasks` (`status = "READY_TO_SCHEDULE"`) por cada ítem aprobado dentro de la misma transacción. Ítems no aprobados NO generan tareas agendables. | Unit + Integration + Adversarial |
| **S3** | Aprobación total del presupuesto (`RF-08`) | `POST /api/v1/treatment-plans/{planId}/approve` con `mode: "TOTAL"` aprueba todos los ítems pendientes del plan, transiciona el plan a `APPROVED_TOTAL` y genera las `schedulable_tasks` correspondientes sin duplicar tareas previamente generadas. | Unit + Integration |
| **S4** | Inmutabilidad de procedimientos marcados como `"REALIZADO"` (`RN-01`) | `POST /api/v1/treatment-plans/{planId}/items/{itemId}/complete` marca el ítem como `"REALIZADO"`. Cualquier intento posterior de `DELETE` o modificación sobre dicho ítem o de desaprobarlo retorna `409 Conflict` (`CLINICAL_RECORD_IMMUTABLE`). | Unit + Integration + Adversarial |
| **S5** | Consulta de tareas agendables y auditoría (`RF-08`, `RNF-06`) | `GET /api/v1/patients/{id}/schedulable-tasks` retorna las tareas en estado `READY_TO_SCHEDULE`, y cada mutación de presupuesto registra una entrada en `audit_logs`. | Integration |

---

## 4. Contrato de entrada

### `POST /api/v1/patients/{id}/treatment-plans`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist` | `admin`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `title` (`string`, requerido).
  - `currency` (`string`, ISO-4217, ej. `"USD"` o `"ARS"`, requerido).
  - `phases`: array no vacío de fases:
    - `phase_number` (`int`, $\ge 1$, requerido).
    - `name` (`string`, requerido).
    - `items`: array no vacío de procedimientos:
      - `procedure_code` (`string`, requerido).
      - `procedure_name` (`string`, requerido).
      - `tooth_fdi` (`string`, opcional).
      - `surface` (`string`, opcional).
      - `professional_fee_cents` (`int64`, $\ge 0$, requerido).
      - `lab_cost_cents` (`int64`, $\ge 0$, requerido).
      - `discount_cents` (`int64`, $\ge 0$, requerido).
      - `estimated_duration_minutes` (`int`, $15..240$, requerido).

### `POST /api/v1/treatment-plans/{planId}/approve`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist` | `receptionist` | `admin`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `mode` (`string`: `"TOTAL"` | `"PARTIAL"`, requerido).
  - `approved_item_ids` (`[]string`, UUIDs requeridos cuando `mode == "PARTIAL"`).
  - `patient_acceptance_ref` (`string`, referencia de firma o aceptación del paciente, requerido).

---

## 5. Contrato de salida

### Respuesta `POST /api/v1/treatment-plans/{planId}/approve` (`200 OK`)
```json
{
  "plan_id": "11223344-5566-4777-8899-aabbccddeeff",
  "patient_id": "2c5ea4c0-4067-11e9-8bad-9b1deb4d3b7d",
  "status": "APPROVED_PARTIAL",
  "approved_total_cents": 45000,
  "pending_total_cents": 80000,
  "generated_schedulable_tasks": [
    {
      "task_id": "99887766-5544-4333-8221-ffeeddccbbaa",
      "treatment_item_id": "55667788-1122-4333-8445-667788990011",
      "phase_number": 1,
      "procedure_code": "ENDO-01",
      "procedure_name": "Endodoncia Birradicular",
      "tooth_fdi": "14",
      "estimated_duration_minutes": 60,
      "status": "READY_TO_SCHEDULE"
    }
  ]
}
```

### Matriz de Estados HTTP
- `200 OK` / `201 Created`: Plan creado, aprobado o ítem completado exitosamente.
- `403 Forbidden`: Rol sin permiso o descuento por encima del umbral permitido sin autorización de `admin`.
- `404 Not Found`: Plan o ítem inexistente.
- `409 Conflict`: Intento de borrar o revertir un ítem en estado `"REALIZADO"` (`RN-01`).
- `422 Unprocessable Entity`: Descuento mayor a la suma de honorarios + laboratorio, montos negativos, o `approved_item_ids` vacío en modo `PARTIAL`.

---

## 6. Seguridad y resiliencia

- **Aritmética Monetaria Exacta:** Todos los importes (`professional_fee_cents`, `lab_cost_cents`, `discount_cents`) se procesan en enteros de 64 bits (`int64` centavos) y `BIGINT` en PostgreSQL para evitar errores de redondeo IEEE-754.
- **Atomicidad Transaccional (`RF-08`):** La actualización de estado de los ítems aprobados y la inserción en `schedulable_tasks` ocurren dentro de una única transacción ACID con rollback automático ante cualquier fallo.
- **Inmutabilidad (`RN-01`) y Auditoría (`RNF-06`):** Prohibición de `DELETE` sobre ítems `"REALIZADO"` tanto en dominio como en base de datos, y registro en `audit_logs` de cada cambio de estado.

---

## 7. Archivos afectados

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
