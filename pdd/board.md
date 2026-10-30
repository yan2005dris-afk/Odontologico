# PDD Board — SIGO-Dental

Estado general del árbol de trabajo PDD (`pdd/`). Todos los planes siguen la arquitectura **Hexagonal Pattern B** en servicios Go (`services/api-gateway/`, `services/clinical-agenda/`, `services/billing-stock/`) y **Next.js App Router** en `frontend/`, particionados en **4 git worktrees** (`wt/gateway`, `wt/clinical-agenda`, `wt/billing-stock`, `wt/frontend`) para ejecución paralela sin conflictos de archivos.

---

## 1. Índice de Features (`pdd/tasks/`)

| Feature ID | Módulo SRS / Alcance | Requerimientos Cubiertos | Work Items Asociados | Estado | Especificación |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **F01** | Plataforma, API Gateway, Identidad/RBAC/MFA y Auditoría | `SRS §3`, `SRS §7`, `RNF-01`..`RNF-06`, `SRS §8.2` | `wi-01`, `wi-02` | `approved` | [`pdd/tasks/01-platform-gateway-auth.md`](./tasks/01-platform-gateway-auth.md) |
| **F02** | Módulo 4.1: Gestión de Pacientes, Filiación y Alertas Médicas | `RF-01`, `RF-02`, `RNF-01`, `RNF-05`, `RNF-06` | `wi-03` | `approved` | [`pdd/tasks/02-patients-anamnesis.md`](./tasks/02-patients-anamnesis.md) |
| **F03** | Módulo 4.2: Expediente Clínico, Odontograma 2D, Periodontograma, SOEP y Consentimientos | `RF-03`, `RF-04`, `RF-05`, `RF-06`, `RN-01`, `RNF-01`, `RNF-04`, `RNF-06` | `wi-04`, `wi-05` | `approved` | [`pdd/tasks/03-clinical-odontogram-soep.md`](./tasks/03-clinical-odontogram-soep.md) |
| **F04** | Módulo 4.3: Planes de Tratamiento, Presupuestos Multinivel y Pase a Citas | `RF-07`, `RF-08`, `RN-01`, `RNF-05`, `RNF-06` | `wi-06` | `approved` | [`pdd/tasks/04-treatments-budgets.md`](./tasks/04-treatments-budgets.md) |
| **F05** | Módulo 4.4: Agenda Multirrecinto, Estados de Cita y Recordatorios 48h/24h | `RF-09`, `RF-10`, `RF-11`, `RN-03`, `RNF-02`, `RNF-03`, `RNF-06` | `wi-07` | `approved` | [`pdd/tasks/05-scheduling-notifications.md`](./tasks/05-scheduling-notifications.md) |
| **F06** | Módulo 4.5: POS, Facturación Electrónica, Cierre Ciego, Liquidaciones e Inventario | `RF-12`, `RF-13`, `RF-14`, `RN-02`, `RNF-04`..`RNF-06`, `SRS §7.1` | `wi-08`, `wi-09` | `approved` | [`pdd/tasks/06-billing-cash-inventory.md`](./tasks/06-billing-cash-inventory.md) |
| **F07** | Portal Web / PWA Next.js y Workspaces Clínicos en Sillón | `RF-01`..`RF-14`, `RN-01`..`RN-03`, `RNF-01`, `RNF-05`, `RNF-07`, `RNF-08` | `wi-10`, `wi-11`, `wi-12` | `approved` | [`pdd/tasks/07-frontend-portal-ui.md`](./tasks/07-frontend-portal-ui.md) |

---

## 2. Matriz de Work Items y Topología de Git Worktrees (`pdd/<work-item>/plan.md`)

| Work Item | Feature | Git Worktree Branch | Directorio / Frontera de Aislamiento | Dependencias Previas | MaxCorrections | Fase Actual | Rol Activo | Estado | Plan |
| :--- | :--- | :--- | :--- | :--- | :---: | :--- | :--- | :--- | :--- |
| **Contextualización (`/init`)** | — | `main` | `pdd/context/` | Ninguna | — | `init` | Orchestrator | `completed` | [`pdd/context/project-context.md`](./context/project-context.md) |
| **`wi-01-gateway-ingress`** | `F01` | `wt/gateway` | `services/api-gateway/`, `deploy/`, `openapi/` | Ninguna (Wave 1) | `200` | `apply` | Implementer | `implementing` | [`pdd/wi-01-gateway-ingress/plan.md`](./wi-01-gateway-ingress/plan.md) |
| **`wi-02-clinical-identity-audit`** | `F01` | `wt/clinical-agenda` | `services/clinical-agenda/internal/identity`, `internal/audit`, `migrations/00001*` | Ninguna (Wave 1) | `200` | `apply` | Implementer | `implementing` | [`pdd/wi-02-clinical-identity-audit/plan.md`](./wi-02-clinical-identity-audit/plan.md) |
| **`wi-03-clinical-patients`** | `F02` | `wt/clinical-agenda` | `services/clinical-agenda/internal/patients`, `migrations/00002*` | `wi-02` (Wave 2) | `200` | `plan` | Implementer | `plan_approved` | [`pdd/wi-03-clinical-patients/plan.md`](./wi-03-clinical-patients/plan.md) |
| **`wi-04-clinical-odontogram-perio`** | `F03` | `wt/clinical-agenda` | `services/clinical-agenda/internal/clinical` (`odontogram*`, `periodontogram*`), `migrations/00003*` | `wi-03` (Wave 3) | `200` | `plan` | Implementer | `plan_approved` | [`pdd/wi-04-clinical-odontogram-perio/plan.md`](./wi-04-clinical-odontogram-perio/plan.md) |
| **`wi-05-clinical-soep-consents`** | `F03` | `wt/clinical-agenda` | `services/clinical-agenda/internal/clinical` (`soep*`, `consent*`), `migrations/00004*` | `wi-03` (Wave 3) | `200` | `plan` | Implementer | `plan_approved` | [`pdd/wi-05-clinical-soep-consents/plan.md`](./wi-05-clinical-soep-consents/plan.md) |
| **`wi-06-clinical-treatments`** | `F04` | `wt/clinical-agenda` | `services/clinical-agenda/internal/treatments`, `migrations/00005*` | `wi-03`, `wi-04` (Wave 4) | `200` | `plan` | Implementer | `plan_approved` | [`pdd/wi-06-clinical-treatments/plan.md`](./wi-06-clinical-treatments/plan.md) |
| **`wi-07-clinical-scheduling`** | `F05` | `wt/clinical-agenda` | `services/clinical-agenda/internal/scheduling`, `migrations/00006*` | `wi-02`, `wi-03`, `wi-06` (Wave 5) | `200` | `plan` | Implementer | `plan_approved` | [`pdd/wi-07-clinical-scheduling/plan.md`](./wi-07-clinical-scheduling/plan.md) |
| **`wi-08-billing-pos-cash`** | `F06` | `wt/billing-stock` | `services/billing-stock/cmd/`, `internal/billing`, `migrations/00001*` | Ninguna (Wave 1) | `200` | `apply` | Implementer | `implementing` | [`pdd/wi-08-billing-pos-cash/plan.md`](./wi-08-billing-pos-cash/plan.md) |
| **`wi-09-billing-settlements-inventory`** | `F06` | `wt/billing-stock` | `services/billing-stock/internal/settlements`, `internal/inventory`, `migrations/00002*` | `wi-08` (Wave 2) | `200` | `plan` | Implementer | `plan_approved` | [`pdd/wi-09-billing-settlements-inventory/plan.md`](./wi-09-billing-settlements-inventory/plan.md) |
| **`wi-10-frontend-shell-patients-agenda`** | `F07` | `wt/frontend` | `frontend/` (`shell`, `patients`, `agenda`, `auth`) | Ninguna en `frontend/` (Wave 1) | `200` | `apply` | Implementer | `implementing` | [`pdd/wi-10-frontend-shell-patients-agenda/plan.md`](./wi-10-frontend-shell-patients-agenda/plan.md) |
| **`wi-11-frontend-clinical-workspace`** | `F07` | `wt/frontend` | `frontend/src/components/clinical`, `components/treatments`, `patients/[id]` | `wi-10` (Wave 2) | `200` | `plan` | Implementer | `plan_approved` | [`pdd/wi-11-frontend-clinical-workspace/plan.md`](./wi-11-frontend-clinical-workspace/plan.md) |
| **`wi-12-frontend-billing-inventory`** | `F07` | `wt/frontend` | `frontend/src/components/billing`, `components/inventory`, `(portal)/billing`, `settlements`, `inventory` | `wi-10` (Wave 2) | `200` | `plan` | Implementer | `plan_approved` | [`pdd/wi-12-frontend-billing-inventory/plan.md`](./wi-12-frontend-billing-inventory/plan.md) |
| **`wi-13-frontend-next-port`** | `F07` | `wt/frontend` + `wt/gateway` | `frontend/src/app/`, `frontend/src/lib/`, `services/api-gateway/internal/ingress/adapters/http/proxy_router.go` | `wi-10`, `wi-11`, `wi-12`, `wi-01`, `wi-02` (Wave 3) | `200` | `review` | Reviewer | `implemented` | [`pdd/wi-13-frontend-next-port/plan.md`](./wi-13-frontend-next-port/plan.md) |
