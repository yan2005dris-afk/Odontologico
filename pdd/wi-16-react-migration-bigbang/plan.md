# Plan de Work Item — `wi-16-react-migration-bigbang`

Feature: `F08` (`08-frontend-shadcn-admin-lists`) — migración total del portal a React real + pantallas clínicas de lista.
Worktree: `wt/frontend` (`.worktrees/frontend`).
Rol activo: Planner → Implementer. `MaxCorrections`: `200`.
Estado: `plan_pending_approval`.
Base: commit `b4e60cf` (checkpoint pre-refactor). Spec: [`pdd/tasks/08-frontend-shadcn-admin-lists.md`](../tasks/08-frontend-shadcn-admin-lists.md) (`S4`..`S8`).

---

## 1. Descripción y alcance

Decisión humana registrada: **big bang**. Los 14 archivos que hoy producen HTML string se borran en un solo movimiento y sus 12 suites de componente se reescriben como tests de componente React. No hay fase intermedia donde un módulo tenga dos implementaciones, ni suites que convivan afirmando sobre HTML y sobre DOM.

Verificación del estado actual (no de la documentación):

- **14 archivos** exponen 15 funciones `render*Html(...)`: `shell/AppShell.tsx:47`, `shell/SlideOverSheet.tsx:18`, `patients/StickyPatientHeader.tsx:64,80`, `patients/PatientRegistrationForm.tsx:139`, `agenda/MultiChairCalendar.tsx:141`, `clinical/OdontogramCanvas.tsx:117`, `clinical/PeriodontogramGrid.tsx:64`, `clinical/SoepEvolutionTimeline.tsx:62`, `clinical/InformedConsentSheet.tsx:57`, `treatments/TreatmentPlanWorkspace.tsx:118`, `billing/PosCheckoutSheet.tsx:97`, `billing/BlindCashCloseView.tsx:87`, `billing/DentistSettlementPanel.tsx:68`, `inventory/InventoryWorkspace.tsx:87`.
- **5 puntos de `dangerouslySetInnerHTML`** en el portal: `patients/page.tsx:199,215`, `patients/[id]/page.tsx:177,232,266,298`, `agenda/page.tsx:294`.
- **`portal-views.ts` es puro modelado de datos**: no contiene una sola cadena de HTML (verificado: `grep -nE "Html|html"` sobre el archivo no arroja coincidencias). Sus 78 aserciones sobreviven intactas. La única fuga de HTML hacia la capa de datos es `renderCriticalAlertBadgesHtml` en `StickyPatientHeader.tsx:64`, consumida en `patients/page.tsx:199`.
- **Las 5 pantallas sin dato real** muestran "Endpoint de lectura pendiente": `billing/page.tsx:35`, `settlements/page.tsx:35`, `inventory/page.tsx:35` (y el modelo compartido `portal-views.ts:304`).

### Dentro del alcance (In Scope)

- **S1.** Borrado de los 14 renderers y de los 5 `dangerouslySetInnerHTML`; cada archivo conserva su ruta y su nombre de archivo y pasa a exportar componentes React reales.
- **S2.** Reescritura de las 12 suites de componente (`src/components/**/*.test.tsx`) como tests de componente con Testing Library.
- **S3.** `StickyPatientHeader` real: cabecera fija con las alertas críticas `RF-02` como badges semánticos con `role="alert"` y `aria-label` (reemplaza `renderCriticalAlertBadgesHtml`).
- **S4.** `PatientRegistrationForm` → `PatientFormSheet`: el alta y la edición de paciente salen de la página y pasan a un `Sheet` controlado.
- **S5.** `/patients` como pantalla de lista: `DataTable` + botón "Nuevo Paciente".
- **S6.** `/patients/[id]` como expediente clínico con `Tabs` reales: Odontograma 2D, Periodontograma, Evolución SOEP, Presupuestos, Consentimientos.
- **S7.** `/agenda` conmutable entre Lista de Turnos (`DataTable`) y Calendario Multirrecinto; alta y transición de estado en `Sheet`.
- **S8.** `/treatments` (ruta nueva): lista de planos y presupuestos con estado de aprobación.
- **S9.** `/consents` (ruta nueva): lista de consentimientos informados con estado de firma.
- **S10.** Migración de los componentes clínicos y financieros a React real: `OdontogramCanvas`, `PeriodontogramGrid`, `SoepEvolutionTimeline`, `InformedConsentSheet`, `TreatmentPlanWorkspace`, `MultiChairCalendar`, `PosCheckoutSheet`, `BlindCashCloseView`, `DentistSettlementPanel`, `InventoryWorkspace`, `AppShell`, `SlideOverSheet`.

### Fuera del alcance (Out of Scope)

- **Las pantallas financieras y de administración** (`/billing`, `/cash-registers`, `/settlements`, `/inventory`, `/users`, `/audit`) y el **dashboard** (`/`): son `wi-17`. Este work item migra sus *componentes* a React, pero no construye sus pantallas de lista.
- Lafoundation de `shadcn/ui` (primitivos, shell, `DataTable`): es `wi-14`, que debe estar merged antes de empezar.
- Los endpoints de lectura: es `wi-15`.
- Persistencia real en PostgreSQL (los repos del backend siguen siendo en memoria).
- Interactividad clínica profunda (clic en una pieza del odontograma para abrir el formulario de hallazgo): la primera versión dibuja y muestra; la interacción completa se aborda en `wi-17` si el tiempo lo permite, y si no, en un work item posterior.

---

## 2. Dependencias previas

- **`wi-14-shadcn-admin-foundation`** (merged): primitivos `ui/`, shell `shadcn-admin`, núcleo `DataTable`, `nav-config.ts`, y el arnés de tests con Vitest + jsdom + Testing Library. Sin la fundación no hay con qué construir.
- **`wi-15-read-contract-listings`** (merged): `GET /api/v1/treatment-plans`, `GET /api/v1/consents`, `GET /api/v1/patients/{id}/treatment-plans`, `GET /api/v1/patients/{id}/consents` y la corrección de ruteo del gateway. Sin ellos, `/treatments` y `/consents` no tienen contrato.
- **Checkpoint `b4e60cf`**: 111 tests verdes y `next build` OK, verificados antes de arrancar.
- **Worktree `wt/frontend`** desde `b4e60cf`, con `pnpm install` ejecutado (el lockfile lo crea `wi-14`).

---

## 3. Specs numeradas

- **S1 — Cero HTML string.** Los 14 archivos de componente dejan de exportar `render*Html(...)` y exportan componentes React. Desaparecen los 5 `dangerouslySetInnerHTML` de `patients/page.tsx:199,215`, `patients/[id]/page.tsx:177,232,266,298` y `agenda/page.tsx:294`. Los modelos de datos de `portal-views.ts` se conservan tal cual: son la frontera entre el servidor y la vista, y no contienen presentación.
- **S2 — Suites de componente reales.** Las 12 suites de `src/components/**/*.test.tsx` se reescriben con Testing Library: se borra toda aserción sobre *substrings* de HTML y se reemplaza por aserciones sobre el árbol accesible (`getByRole`, `getByText`, `toHaveTextContent`, `toBeVisible`). Cada suite mantiene el nombre de su `describe` y la referencia al requisito (`RF-xx` / `RN-xx`) para que la trazabilidad con el SRS sobreviva al refactor. La cobertura funcional no baja: lo que antes se comprobaba sobre el string se comprueba ahora sobre el DOM.
- **S3 — Alertas críticas `RF-02` como badges semánticos.** `<CriticalAlertBadges alerts={...} />` reemplaza `renderCriticalAlertBadgesHtml`. Cada alerta se renderiza como elemento con `role="alert"`, `aria-label` descriptivo y nivel (`CRITICAL` / `HIGH`) distinguible **sin depender sólo del color** (texto + icono), porque el color por sí solo no es accesible. La cabecera del expediente (`StickyPatientHeader`) queda fija (`sticky top-0`) y mantiene la jerarquía visual de `AGENTS.md §3`: la alerta médica crítica es el elemento de mayor visibilidad de la pantalla.
- **S4 — `PatientFormSheet`.** El alta y la edición de paciente se ejecutan en un `Sheet` (Drawer) controlado por el estado del cliente. Al guardar, invoca la server action existente (`src/app/(portal)/patients/create/route.ts`) y refresca la lista. Reglas de negocio intactas: `LEGAL_GUARDIAN_REQUIRED_FOR_MINOR` (`422`) bloquea el alta de un menor sin tutor legal y el mensaje se muestra **dentro** del `Sheet` sin perder lo escrito.
- **S5 — `/patients` como lista.** `DataTable` con columnas: `HC`, `Paciente`, `Documento`, `Edad / Menor`, `Teléfono`, `Alertas Críticas` (badge por cada alerta `RF-02`) y `Acciones` (`DataTableRowActions`: ver, editar en `Sheet`). Encabezado de página con título, subtítulo del requerimiento y acción primaria "+ Nuevo Paciente". Búsqueda instantánea por nombre/DNI/HC y filtro facetado por "alertas críticas" y "menor de edad". La tabla no muestra el formulario de alta incrustado: el formulario vive en el `Sheet`.
- **S6 — `/patients/[id]` como expediente clínico.** Cabecera fija + `Tabs` de `shadcn/ui` con cinco pestañas reales: Odontograma 2D, Periodontograma, Evolución SOEP, Presupuestos, Consentimientos. Cada pestaña carga sus datos desde el servidor y muestra un estado vacío propio cuando el módulo no tiene registro. Las **pestañas de Presupuestos y Consentimientos dejan de decir "No disponible"**: se alimentan de los `GET` de `wi-15`. Las acciones ("+ Hallazgo", "+ Nota SOEP", "+ Adenda", "+ Presupuesto", "+ Firmar Consentimiento") abren `Sheet`.
- **S7 — `/agenda` conmutable.** Conmutador entre **Lista de Turnos** (`DataTable`: paciente, sillón, especialista, hora, estado, acciones) y **Calendario Multirrecinto** por sillón. Filtros por fecha, sillón y especialista. El alta de cita y la transición de estado (`RF-10`) operan desde `Sheet`. El conflicto `409 CHAIR_OVERLAP_CONFLICT` (`RN-03`) se muestra como banner dentro del `Sheet` **sin perder los datos escritos**, y la disponibilidad del sillón se calcula con la misma regla del backend.
- **S8 — `/treatments`.** Ruta nueva con `DataTable` de planos de tratamiento: paciente, código de procedimiento, estado (`DRAFT`, `PARTIALLY_APPROVED`, `APPROVED`, `COMPLETED`), total presupuestado y total aprobado, ambos con `tabular-nums`. Filtro por estado y por paciente. `Sheet` para crear plan y para aprobar ítems; la aprobación parcial o total (`RF-08`) usa el endpoint que `wi-15` dejó alcanzable a través del gateway.
- **S9 — `/consents`.** Ruta nueva con `DataTable` de consentimientos: paciente, procedimiento, estado de firma (`PENDING`, `SIGNED_BIOMETRIC`, `SIGNED_OTP`), fecha y estado del documento firmado. `Sheet` de captura de firma (OTP de 6 dígitos o biométrica) que **no** expone el TOTP en logs ni lo persiste. `RN-01`: un documento firmado no se edita; la corrección exige un documento nuevo.
- **S10 — Componentes clínicos y financieros a React.** Cada componente conserva su semántica de dominio: `OdontogramCanvas` mantiene la paleta de hallazgos de `RF-03` (rojo = caries, azul = restauración) **tomada de los tokens de `globals.css`** — no de constantes duplicadas — y los números de pieza en notación FDI/Universal con `tabular-nums`; `PeriodontogramGrid` muestra las profundidades de sondaje con `tabular-nums`; `SoepEvolutionTimeline` respeta el bloqueo de 24 h de `RN-01` (una nota fuera de ventana se muestra como sólo lectura, con su addendum); `BlindCashCloseView` mantiene la garantía de `RN-02` (no pinta total teórico antes del arqueo).
- **S11 — Modelo de render: RSC + islas cliente.** Las páginas siguen siendo Server Components: leen los datos en el servidor con el cliente de `src/lib/gateway-server.ts:450` y pasan props serializables. Las islas cliente (`DataTable`, `Sheet`, `CommandMenu`, formularios) son Client Components acotados. El navegador **nunca** llama al gateway directamente: todo va same-origin por Next (invariante de `wi-13`, `S4`). La URL conserva el estado de la tabla (búsqueda, filtros, página) para que el estado sea compartible y el botón atrás del navegador no lo pierda.

---

## 4. Contrato de entrada

- **Endpoints consumidos** (todos servidos por `wi-15` salvo los ya existentes):
  - `GET /api/v1/patients` · `GET /api/v1/patients/{id}/header` · `GET /api/v1/patients/{id}/odontogram` · `GET /api/v1/patients/{id}/periodontograms` · `GET /api/v1/patients/{id}/soep-notes`
  - `GET /api/v1/patients/{id}/treatment-plans` · `GET /api/v1/patients/{id}/consents` · `GET /api/v1/treatment-plans` · `GET /api/v1/consents`
  - `GET /api/v1/scheduling/appointments` · `POST /api/v1/scheduling/appointments` · `POST /api/v1/scheduling/appointments/{id}/transition`
  - `POST /api/v1/patients` · `POST /api/v1/patients/{id}/anamnesis` · `POST /api/v1/patients/{id}/treatment-plans` · `POST /api/v1/treatment-plans/{planId}/approve` · `POST /api/v1/patients/{id}/consents` · `POST /api/v1/patients/{id}/consents/{consentId}/sign`
- **Variables**: `GATEWAY_INTERNAL_URL` (server-side), `NEXT_PUBLIC_API_GATEWAY_URL` (compatibilidad), `SESSION_COOKIE_NAME`.
- **Tipos**: se reusan `PatientSummaryDTO`, `CriticalAlertDTO`, `AppointmentStatus`, `UserRole` (`src/lib/session-idle-guard.ts`), `WirePatient`, `WireAppointment` (`src/lib/gateway-server.ts`). Los DTO nuevos de listados se tipan en `src/lib/api-types.ts`, derivados de la forma real de la respuesta `{items, total}` de `wi-15`.
- **Restricción**: los componentes de `shadcn/ui` y el `DataTable` vienen de `wi-14`; este work item no crea primitivos nuevos salvo los estrictamente necesarios para el dominio clínico.

---

## 5. Contrato de salida

- `pnpm test` verde: las 12 suites de componente reescritas + las 6 de `src/lib` (sin cambios) + las nuevas de las 5 pantallas.
- `pnpm build` sale 0 con las 14 rutas del portal (11 existentes + `/treatments` + `/consents`).
- `grep -rn "dangerouslySetInnerHTML" frontend/src` no arroja coincidencias.
- `grep -rn "render.*Html" frontend/src` no arroja coincidencias.
- Las 5 pantallas que hoy dicen "Endpoint de lectura pendiente" ya no lo dicen: `/patients`, `/patients/[id]`, `/agenda`, `/treatments` y `/consents` muestran datos reales o un estado vacío honesto.
- El expediente `/patients/[id]` no muestra "No disponible" en las pestañas de Presupuestos ni de Consentimientos.

---

## 6. Seguridad y resiliencia

- **Sesión `RNF-05`**: las islands cliente reciben datos ya filtrados por el servidor. Ningún token ni cookie `sigo_session` (`httpOnly`) llega al bundle del navegador. El `401 SESSION_IDLE_TIMEOUT` se resuelve en el servidor y redirige a `/login?reason=idle_timeout`.
- **`RN-02` (cierre ciego)**: `BlindCashCloseView` renderizado en React **nunca** muestra el total teórico del sistema antes de que la receptora envíe su arqueo físico. La caja abierta no trae el campo desde el servidor (`wi-15`, `S6`), de modo que la garantía es del contrato de datos, no sólo de la vista.
- **`RN-03` (solapamiento de sillón)**: el `409 CHAIR_OVERLAP_CONFLICT` se muestra como banner en el `Sheet` sin descartar lo escrito; no se reintenta a ciegas.
- **`RN-01` (inmutabilidad)**: la UI de la nota SOEP y del odontograma marca como bloqueada toda nota fuera de la ventana de 24 h y ofrece la vía del addendum firmado; no ofrece un botón de edición que el backend vaya a rechazar.
- **`RF-02` (alertas)**: las badges llevan `role="alert"` y texto, no sólo color. Los valores de las alertas se escapan por React (el escape de strings manual desaparece con el fin de los renderers), lo que elimina por construcción la clase de fallo XSS que `escapeHtml` cubría a mano.
- **MFA**: la firma de consentimiento exige `mfa_verified=true` en el gateway (`wi-15`, `S4`); la UI muestra el error `403 MFA_REQUIRED` con un enlace a re-verificar, sin exponer el reto.
- **Sin datos simulados**: ante un error de gateway se muestra un estado de error recuperable con reintento. Nunca se fabrica una lista, un total ni un éxito.

---

## 7. Archivos afectados

**Componentes — se reescriben (14)**
- `frontend/src/components/shell/AppShell.tsx`, `shell/SlideOverSheet.tsx`
- `frontend/src/components/patients/StickyPatientHeader.tsx`, `patients/PatientRegistrationForm.tsx`
- `frontend/src/components/agenda/MultiChairCalendar.tsx`
- `frontend/src/components/clinical/OdontogramCanvas.tsx`, `clinical/PeriodontogramGrid.tsx`, `clinical/SoepEvolutionTimeline.tsx`, `clinical/InformedConsentSheet.tsx`
- `frontend/src/components/treatments/TreatmentPlanWorkspace.tsx`
- `frontend/src/components/billing/PosCheckoutSheet.tsx`, `billing/BlindCashCloseView.tsx`, `billing/DentistSettlementPanel.tsx`
- `frontend/src/components/inventory/InventoryWorkspace.tsx`

**Suites — se reescriben (12)**
- Los 12 `frontend/src/components/**/*.test.tsx` listados en `S2`.

**Páginas — se construyen o se rehacen (5)**
- `frontend/src/app/(portal)/patients/page.tsx` · `patients/[id]/page.tsx` · `agenda/page.tsx`
- `frontend/src/app/(portal)/treatments/page.tsx` (nueva) · `consents/page.tsx` (nueva)

**Nuevos**
- `frontend/src/components/patients/PatientFormSheet.tsx` · `frontend/src/components/patients/CriticalAlertBadges.tsx`
- `frontend/src/components/agenda/AppointmentSheet.tsx` · `frontend/src/components/treatments/TreatmentPlanSheet.tsx` · `frontend/src/components/clinical/ConsentSignSheet.tsx`
- `frontend/src/components/data-table/ColumnsToolbar.tsx` (columnas por módulo)
- `frontend/src/lib/api-types.ts` (DTO de listados de `wi-15`)

**Sin tocar**
- `frontend/src/lib/portal-views.ts` y `frontend/src/lib/portal-views.test.ts` (modelado puro: sobreviven).
- `frontend/src/lib/session-idle-guard.ts`, `auth-flow.ts`, `gateway-server.ts`, `session-cookie.ts` (no hay cambios funcionales previstos).
- `frontend/src/app/(portal)/billing|settlements|inventory/page.tsx` (pantallas de `wi-17`).

---

## 8. Orden de ejecución (TDD)

| Task | Specs | Archivos | Prueba RED |
| :--- | :--- | :--- | :--- |
| T1 | S1, S2 | `StickyPatientHeader.tsx` + suite | la suite no puede afirmar sobre `renderStickyPatientHeaderHtml` (ya no existe) → se escribe el test DOM primero |
| T2 | S3 | `CriticalAlertBadges.tsx` + suite | 2 alertas ⇒ 2 elementos `role="alert"` con texto y nivel, sin depender del color |
| T3 | S10 | `OdontogramCanvas`, `PeriodontogramGrid` + suites | render DOM con findings y profundidades; `tabular-nums` en números de pieza y sondaje |
| T4 | S10 | `SoepEvolutionTimeline`, `InformedConsentSheet` + suites | nota fuera de 24 h marcada como sólo lectura; `Sheet` de firma sin exponer el TOTP |
| T5 | S10 | `MultiChairCalendar`, `TreatmentPlanWorkspace` + suites | calendario por sillón; plan con estados de aprobación y totales con `tabular-nums` |
| T6 | S10 | `PosCheckoutSheet`, `BlindCashCloseView`, `DentistSettlementPanel`, `InventoryWorkspace` + suites | la vista de caja ciega **no** muestra total teórico; `InventoryWorkspace` marca stock bajo |
| T7 | S4, S5 | `PatientFormSheet`, `patients/page.tsx` + suite | `422 LEGAL_GUARDIAN_REQUIRED_FOR_MINOR` se muestra en el `Sheet` sin perder lo escrito |
| T8 | S5 | `patients/page.tsx` | la tabla lista pacientes con badges de alerta; el formulario **no** está en la página |
| T9 | S6 | `patients/[id]/page.tsx` + suite | las 5 pestañas montan; presupuestos y consentimientos ya no dicen "No disponible" |
| T10 | S7 | `agenda/page.tsx`, `AppointmentSheet` + suite | conmutador lista/calendario; `409` no pierde el formulario |
| T11 | S8 | `treatments/page.tsx`, `TreatmentPlanSheet` + suite | listado con estado y totales; la aprobación parcial actualiza el estado del plan |
| T12 | S9 | `consents/page.tsx`, `ConsentSignSheet` + suite | listado por estado de firma; la firma OTP deja el estado en `SIGNED_OTP` |
| T13 | S11 | páginas | los datos viajan del servidor a la isla cliente; el bundle no contiene el token de sesión |
| T14 | S1..S11 | todo | `pnpm test` + `pnpm build` verdes; **cero** `dangerouslySetInnerHTML` y **cero** `render*Html` en `frontend/src` |

**Criterio de salida**: T14 verde, `review` de 4 lentes en `pdd/wi-16-react-migration-bigbang/review.md` con citas `file:line`, y `apply.md` con los comandos ejecutados y sus salidas.
