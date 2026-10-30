# Feature 08 — Refactorización de Arquitectura Frontend (`shadcn-admin`) y Desacoplamiento de Pantallas de Lista

- **Feature ID:** `08-frontend-shadcn-admin-lists`
- **SRS References:** `SRS §3` (RBAC), `SRS §4.1..4.5` (`RF-01`..`RF-14`), `SRS §5` (`RNF-01`, `RNF-05`, `RNF-06`, `RNF-07`, `RNF-08`), `SRS §6` (`RN-01`, `RN-02`, `RN-03`), Interface Architecture (`AGENTS.md §3`)
- **Inspiration / Architecture Reference:** [`satnaing/shadcn-admin`](https://github.com/satnaing/shadcn-admin) adapted to Next.js 15 App Router + React 19 + Tailwind CSS v4.
- **Status:** `awaiting_plan_approval`
- **Work Items:**
  - [`wi-14-shadcn-admin-refactor`](../wi-14-shadcn-admin-refactor/plan.md) (`wt/frontend` + `wt/clinical-agenda` + `wt/billing-stock`)

---

## 1. Diagnóstico del Estado Actual vs. `Documento de Requerimientos de Software - Sistema Odontológico.md`

### 1.1 ¿Qué está acoplado o incompleto hoy en `frontend/`?
1. **Renderizado por strings crudos (`dangerouslySetInnerHTML`) en lugar de componentes React/shadcn:**
   - Casi todos los módulos (`PatientRegistrationForm`, `StickyPatientHeader`, `MultiChairCalendar`, `OdontogramCanvas`, `PeriodontogramGrid`, `SoepEvolutionTimeline`, `InformedConsentSheet`, `TreatmentPlanWorkspace`, `PosCheckoutSheet`, `BlindCashCloseView`, `DentistSettlementPanel`, `InventoryWorkspace`) exportan funciones `render*Html(...)` que devuelven strings HTML planos.
   - Esto impide tener tablas interactivas (`@tanstack/react-table`), ordenamiento, filtros facetados, paginación real, `Dialog` / `Sheet` controlados y búsqueda rápida ($\⌘K$).
2. **Pantallas acopladas (Lista + Formulario en la misma vista):**
   - En `/patients` (`src/app/(portal)/patients/page.tsx`), el listado de pacientes y el formulario completo de **Alta de Paciente** conviven pegados en la misma página.
   - En `/agenda` (`src/app/(portal)/agenda/page.tsx`), los controles de cambio de estado están embebidos como formularios HTML por tarjeta sin una vista de tabla/lista de turnos desacoplada ni un `Sheet` para agendar/reprogramar citas.
3. **Módulos sin pantalla de lista propia o con estado `"Sin endpoint de lectura"`:**
   - **Presupuestos / Planes de Tratamiento (`RF-07`, `RF-08`):** No tienen pantalla de lista dedicada (`/treatments`), y dentro de `/patients/[id]?tab=budgets` figura como `"No disponible"`.
   - **Consentimientos Informados (`RF-06`):** No tienen pantalla de lista dedicada (`/consents`), y dentro de `/patients/[id]?tab=consents` figura como `"No disponible"`.
   - **Facturación y Comprobantes (`RF-12`):** `/billing` solo muestra un cartel estático porque falta `GET /api/v1/billing/invoices`.
   - **Cierre de Caja Ciego (`RN-02`):** No está separado de la lista de comprobantes en su propia vista/flujo operativo (`/cash-registers`).
   - **Liquidaciones de Honorarios (`RF-13`):** `/settlements` solo muestra un cartel estático porque falta `GET /api/v1/settlements` y `GET /api/v1/settlements/rules`.
   - **Inventario de Insumos y Recetas (`RF-14`):** `/inventory` solo muestra un cartel estático porque falta `GET /api/v1/inventory/items` y `GET /api/v1/inventory/recipes`.
   - **Auditoría Inmutable (`RNF-06`):** El backend YA TIENE `GET /api/v1/audit/logs` implementado en `clinical-agenda`, pero **no existe la pantalla `/audit` en el frontend**.
   - **Usuarios y Permisos (`SRS §3`, `RNF-05`):** No existe la pantalla `/users` para el rol `admin` ni `GET /api/v1/users` en `clinical-agenda`.

---

## 2. Arquitectura Objetivo: Patrón `shadcn-admin` Desacoplado

Adoptamos la estructura visual y ergonómica de [`satnaing/shadcn-admin`](https://github.com/satnaing/shadcn-admin), respetando estrictamente las invariantes clínicas de `AGENTS.md §3` (fondo `bg-slate-50`, lienzos clínicos en blanco puro `#FFFFFF`, `tabular-nums` obligatorio en odontograma/periodontograma/finanzas, y alta visibilidad en alertas `RF-02`).

### 2.1 App Shell (`shadcn-admin` Layout)
- **`SidebarProvider` + `AppSidebar` colapsable a íconos:**
  - Cabecera de clínica (`TeamSwitcher` / Sede SIGO-Dental).
  - Grupos de navegación (`NavGroup`) filtrados por rol (`RBAC`):
    - **Clínica & Turnos:** `Dashboard` (`/`), `Agenda & Turnos` (`/agenda`), `Pacientes` (`/patients`), `Planes y Presupuestos` (`/treatments`), `Consentimientos` (`/consents`).
    - **Caja & Operaciones:** `Facturación POS` (`/billing`), `Cierres de Caja` (`/cash-registers`), `Liquidaciones` (`/settlements`), `Inventario & Recetas` (`/inventory`).
    - **Administración & Seguridad:** `Usuarios y Roles` (`/users`), `Auditoría Clínica` (`/audit`).
  - Pie de barra lateral (`NavUser`) con indicador en vivo del timeout de inactividad de 15 minutos (`RNF-05`).
- **`Header` Superior Unificado:**
  - Botón `SidebarTrigger` + separador vertical.
  - `Breadcrumb` dinámico según la ruta activa.
  - Buscador global `Search` ($\⌘K$ `CommandMenu`) para saltar directo a un paciente, turno o módulo.
  - Badge de estado de sesión (`15m idle guard`) y menú de perfil (`ProfileDropdown`).

### 2.2 Regla de Oro: Una Pantalla de Lista (`DataTable`) por Entidad + Acciones en `Sheet` / `Dialog`
Ninguna pantalla de listado vuelve a tener formularios de creación/edición incrustados abajo. Cada módulo sigue la estructura estándar de `shadcn-admin`:
1. **Encabezado de Página (`PageHeader`):** Título, subtítulo descriptivo del requerimiento SRS y **Botones de Acción Primaria** arriba a la derecha (ej. `+ Nuevo Paciente`, `+ Agendar Turno`, `+ Emitir Comprobante`).
2. **Barra de Herramientas de Tabla (`DataTableToolbar`):** Input de búsqueda instantánea, filtros facetados (`DataTableFacetedFilter` por estado, severidad, profesional, sillón, método de pago) y selector de columnas (`DataTableViewOptions`).
3. **Tabla de Datos (`DataTable`):** Celdas tipadas, números con `tabular-nums`, badges semánticos y menú de acciones por fila (`DataTableRowActions`: Ver detalle, Editar en `Sheet`, Cambiar estado).
4. **Pie de Paginación (`DataTablePagination`):** Total de registros, tamaño de página y navegación.
5. **Drawers / Modals Desacoplados (`Sheet` / `Dialog`):** Se abren bajo demanda al pulsar el botón primario o una acción de fila, manteniendo al usuario anclado en su lista (`AGENTS.md §3.1`).

---

## 3. Especificaciones Numeradas (`S1..S12`)

- **S1 — Primitivos `shadcn/ui`, `DataTable` y `AppShell` estilo `shadcn-admin` (`RNF-05`, `RNF-07`):** Incorporar primitivos (`Sidebar`, `Header`, `Breadcrumb`, `CommandMenu`, `Table`, `DataTable`, `Badge`, `Button`, `Input`, `Select`, `Sheet`, `Dialog`, `Tabs`, `DropdownMenu`, `Card`) sobre Tailwind CSS v4.
- **S2 — Endpoints `GET` de Listado Faltantes en Backend (`clinical-agenda` & `billing-stock`):**
  - `clinical-agenda`:
    - `GET /api/v1/users` (para `/users`, rol `admin`).
    - `GET /api/v1/treatment-plans` y `GET /api/v1/patients/{id}/treatment-plans` (para `/treatments` y tab de paciente).
    - `GET /api/v1/consents` y `GET /api/v1/patients/{id}/consents` (para `/consents` y tab de paciente).
  - `billing-stock`:
    - `GET /api/v1/billing/invoices` y `GET /api/v1/billing/cash-registers` (respetando `RN-02`: oculta `theoretical_cash` si la caja sigue `OPEN` para recepcionistas).
    - `GET /api/v1/settlements` y `GET /api/v1/settlements/rules`.
    - `GET /api/v1/inventory/items` y `GET /api/v1/inventory/recipes`.
- **S3 — Dashboard Resumen Operativo (`/`):** Tarjetas de métricas operativas reales (citas del día, pacientes en sala de espera, alertas de stock crítico, presupuestos pendientes) + accesos rápidos y lista de próximos turnos en sillón.
- **S4 — Pantalla de Lista de Pacientes (`/patients`) + `PatientFormSheet` Desacoplado (`RF-01`, `RF-02`):** `DataTable` con columnas (`HC`, `Paciente`, `Documento`, `Edad / Menor`, `Teléfono`, `Alertas Críticas RF-02`, `Acciones`). El formulario de alta/edición se mueve exclusivamente a un `Sheet` deslizante (`+ Nuevo Paciente`).
- **S5 — Workspace de Expediente Clínico (`/patients/[id]`) en React Real (`RF-02`..`RF-08`, `RN-01`):** Cabecera fija `StickyPatientHeader` + `Tabs` de `shadcn/ui` (`Odontograma 2D`, `Periodontograma`, `Evolución SOEP`, `Presupuestos`, `Consentimientos`), todos conectados a datos reales y con acciones en `Sheet` (`+ Hallazgo`, `+ Nota SOEP`, `+ Adenda`, `+ Presupuesto`, `+ Firmar Consentimiento`).
- **S6 — Pantalla de Agenda y Lista de Turnos (`/agenda`) + `AppointmentSheet` (`RF-09`, `RF-10`, `RN-03`):** Vista conmutable entre **Lista (`DataTable`)** y **Calendario Multirrecinto por Sillón**, filtrable por fecha, sillón dental y especialista. El alta de cita y la transición de estado (`RF-10`) operan desde `Sheet` / acciones rápidas con validación de solapamiento `RN-03`.
- **S7 — Pantalla de Lista de Planes de Tratamiento y Presupuestos (`/treatments`) (`RF-07`, `RF-08`):** `DataTable` de presupuestos con estado (`DRAFT`, `PARTIALLY_APPROVED`, `APPROVED`, `COMPLETED`), total presupuestado (`tabular-nums`), y `Sheet` para crear plan, aprobar ítems y convertirlos en tareas agendables.
- **S8 — Pantalla de Lista de Consentimientos Informados (`/consents`) (`RF-06`):** `DataTable` de consentimientos emitidos por paciente/procedimiento, estado de firma (`PENDING`, `SIGNED_BIOMETRIC`, `SIGNED_OTP`) y `Sheet` de captura de firma digital / OTP.
- **S9 — Pantalla de Lista de Facturación POS (`/billing`) + `PosCheckoutSheet` (`RF-12`):** `DataTable` de comprobantes electrónicos emitidos, estado fiscal, método de pago mixto y `Sheet` de cobro POS.
- **S10 — Pantalla de Lista de Cierres de Caja (`/cash-registers`) + `BlindCashCloseSheet` (`RN-02`):** `DataTable` de aperturas/cierres de caja y `Sheet` de **Cierre de Caja Ciego** que jamás revela el monto teórico del sistema antes de enviar el arqueo físico (`RN-02`).
- **S11 — Pantallas de Lista de Liquidaciones (`/settlements`) e Inventario (`/inventory`) (`RF-13`, `RF-14`):**
  - `/settlements`: `DataTable` de liquidaciones a profesionales + `Sheet` para configurar reglas de comisión y generar liquidación.
  - `/inventory`: `DataTable` de catálogo de insumos clínicos (stock actual, stock mínimo, consultorio, estado de alerta) + `Sheet` para alta/reposición de insumo y configuración de receta estándar por procedimiento.
- **S12 — Pantallas de Lista de Usuarios (`/users`) y Auditoría Clínica (`/audit`) (`SRS §3`, `RNF-05`, `RNF-06`):**
  - `/users`: `DataTable` de personal de la clínica, rol RBAC, estado MFA (`shadcn-admin` Users pattern).
  - `/audit`: `DataTable` inmutable de eventos de auditoría (`Timestamp ISO-8601`, `Actor`, `Rol`, `Acción`, `Recurso`, `IP de Origen`).
