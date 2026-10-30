# Plan de Work Item — `wi-10-frontend-shell-patients-agenda`

- **Feature Padre:** [`pdd/tasks/07-frontend-portal-ui.md`](../tasks/07-frontend-portal-ui.md)
- **Worktree / Rama Asignada:** `wt/frontend`
- **Directorio Objetivo:** `frontend/` (App Shell, Auth/MFA, Pacientes y Agenda)
- **Referencias SRS:** `RF-01`, `RF-02`, `RF-09`, `RF-10`, `RN-03`, `RNF-01`, `RNF-05`, `RNF-07`, `RNF-08`, Interface Architecture (`AGENTS.md §3`)

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Bootstrap de la aplicación web / PWA en `frontend/` con Next.js (App Router, TypeScript estricto, Tailwind CSS, Vitest + Testing Library).
- Arquitectura de Interfaz Base (`AGENTS.md §3`, `RNF-07`):
  - Superficie base `bg-slate-50` y tarjetas activas `#FFFFFF` con borde `border-slate-200`, optimizada para escritorio ($1920 \times 1080$), laptops ($1366 \times 768$) y tablets en sillón odontológico ($\ge 10.5''$).
  - Sidebar izquierda colapsable filtrada según los 5 roles RBAC (`Agenda`, `Pacientes`, `Caja e Inventario`, `Reportes`) y componente reutilizable de panel deslizable (*Slide-over Sheet / Drawer*) para acciones transaccionales en contexto.
- Autenticación, Desafío MFA y Guardián de Inactividad de 15 Minutos (`RNF-05`):
  - Pantalla de Login + verificación TOTP de 6 dígitos (`tabular-nums`) y monitor de eventos de usuario (`pointerdown`, `keydown`, `touchstart`) que cierra automáticamente la sesión en estaciones clínicas tras 15 minutos (`900s`) de inactividad.
- Módulo de Pacientes y Cabecera Fija de Alertas Críticas (`RF-01`, `RF-02`):
  - Directorio de pacientes, formulario de alta/edición con exigencia dinámica de Tutor Legal cuando la edad calculada es $< 18$ años (`RF-01`).
  - Componente `StickyPatientHeader` que despliega de forma permanente e ineludible los *badges* semánticos de alta visibilidad para condiciones médicas críticas (`RF-02`: ej. Alergia a Penicilina, Anticoagulados).
- Módulo de Agenda Multirrecinto y Máquina de Estados (`RF-09`, `RF-10`, `RN-03`):
  - Vista de calendario por Día y Semana filtrable por consultorio/sillón físico (`sillón`) y por odontólogo especialista (`RF-09`).
  - *Sheet* de agendamiento con validación preventiva y visualización de error ante conflicto de sobreposición de sillón físico (`RN-03`), más selector de transición de los 7 estados de cita (`RF-10`).

### Fuera del alcance (Out of Scope)
- Workspaces clínicos de Odontograma 2D, Periodontograma, SOEP, Consentimientos y Presupuestos (cubiertos en `wi-11-frontend-clinical-workspace`).
- Pantallas de POS, Cierre de Caja Ciego, Liquidaciones e Inventario (cubiertas en `wi-12-frontend-billing-inventory`).

---

## 2. Dependencias previas

- **Work Items previos:** Contratos HTTP definidos en `wi-01-gateway-ingress`, `wi-02-clinical-identity-audit`, `wi-03-clinical-patients` y `wi-07-clinical-scheduling`. (Ejecutable en paralelo en `wt/frontend` contra cliente API tipado y mocks MSW/Vitest).

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | App Shell clínico, Sidebar colapsable por rol y monitor de 15 min de inactividad (`RNF-05`, `RNF-07`) | El layout renderiza los destinos permitidos por el rol del usuario, permite colapsar la barra lateral en tablets $\ge 10.5''$, y el hook `useIdleSessionTimeout(900_000)` invoca `logout()` y redirige a `/login?reason=idle_timeout` exactamente al cumplirse 15 minutos sin interacción. | Unit + Integration + Adversarial |
| **S2** | Formulario de Ficha Única con validación obligatoria de Tutor Legal en menores (`RF-01`) | Si la fecha de nacimiento ingresada corresponde a un paciente de $< 18$ años, la sección `Tutor Legal` se vuelve obligatoria y bloquea el envío si falta nombre, documento, teléfono o parentesco. | Unit + Adversarial |
| **S3** | Cabecera Fija (`StickyPatientHeader`) con *badges* de alerta médica crítica (`RF-02`) | Al abrir cualquier vista del expediente de un paciente con `critical_alerts` no vacío, `StickyPatientHeader` permanece fijo (`sticky top-0`) mostrando badges de alto contraste (`role="alert"`) con el detalle de alergias severas o anticoagulación. | Unit + Integration |
| **S4** | Calendario multirrecinto Día/Semana y bloqueo de sobreposición de sillón (`RF-09`, `RN-03`) | El calendario filtra turnos por `chair_id` y `dentist_id`; al intentar agendar desde el *Sheet* un turno en un sillón ocupado en ese rango horario (aun con otro odontólogo), bloquea el guardado y muestra alerta explícita de conflicto de sillón (`RN-03`). | Unit + Integration + Adversarial |
| **S5** | Control visual de los 7 estados de cita (`RF-10`) | Desde el *Sheet* de detalle del turno, el usuario puede transicionar únicamente a los estados válidos del grafo (`PROGRAMADO`, `CONFIRMADO`, `EN_SALA_DE_ESPERA`, `EN_ATENCION`, `ATENDIDO`, `CANCELADO`, `NO_SHOW`). | Unit + Integration |

---

## 4. Contrato de entrada

### Props y Estado de Entrada (Frontend / API Client)
- Configuración de entorno: `NEXT_PUBLIC_API_GATEWAY_URL`, `NEXT_PUBLIC_IDLE_TIMEOUT_MS=900000`.
- `PatientSummaryDTO` consumido por `StickyPatientHeader`:
  - `id: string`, `hcNumber: string`, `fullName: string`, `age: number`, `isMinor: boolean`, `insurancePlan?: { providerName: string; planName: string }`, `criticalAlerts: Array<{ code: string; label: string; severity: 'CRITICAL' | 'HIGH' }>`.
- Filtros de Agenda (`AgendaFilterState`):
  - `view: 'day' | 'week'`, `date: string (YYYY-MM-DD)`, `chairId?: string`, `dentistId?: string`.

---

## 5. Contrato de salida

### Eventos de Interfaz y Peticiones Emitidas al API Gateway
- `POST /api/v1/auth/login` y `POST /api/v1/auth/mfa/verify` $\rightarrow$ almacenamiento seguro de sesión en memoria/cookie `HttpOnly` y activación del timer de 15 min.
- `POST /api/v1/patients` y `PUT /api/v1/patients/{id}/anamnesis`.
- `GET /api/v1/scheduling/appointments`, `POST /api/v1/scheduling/appointments` y `POST /api/v1/scheduling/appointments/{id}/transition`.
- Garantía de Accesibilidad y Layout: landmarks ARIA (`navigation`, `main`, `banner`, `role="alert"` en badges `RF-02`, `role="dialog"` en *Sheets*).

---

## 6. Seguridad y resiliencia

- **Cierre Automático por Inactividad (`RNF-05`):** Limpieza inmediata del estado de sesión y caché clínica local al expirar los 15 minutos de inactividad o recibir `401 SESSION_IDLE_TIMEOUT` desde el gateway, evitando exposición de datos de salud en pantallas desatendidas en consultorio.
- **Protección XSS y Sanitización:** Ningún campo de notas o anamnesis usa `dangerouslySetInnerHTML`.
- **Resiliencia de Red (`RNF-01`):** Estados de carga tipo *skeleton* y manejo determinístico de errores `409 CHAIR_OVERLAP_CONFLICT` y `422 LEGAL_GUARDIAN_REQUIRED_FOR_MINOR`.

---

## 7. Archivos afectados

- `frontend/package.json`
- `frontend/tsconfig.json`
- `frontend/next.config.ts`
- `frontend/vitest.config.ts`
- `frontend/src/lib/api-client.ts`
- `frontend/src/lib/session-idle-guard.ts`
- `frontend/src/lib/session-idle-guard.test.ts`
- `frontend/src/components/shell/AppShell.tsx`
- `frontend/src/components/shell/SlideOverSheet.tsx`
- `frontend/src/components/patients/StickyPatientHeader.tsx`
- `frontend/src/components/patients/StickyPatientHeader.test.tsx`
- `frontend/src/components/patients/PatientRegistrationForm.tsx`
- `frontend/src/components/patients/PatientRegistrationForm.test.tsx`
- `frontend/src/components/agenda/MultiChairCalendar.tsx`
- `frontend/src/components/agenda/MultiChairCalendar.test.tsx`
- `frontend/src/app/layout.tsx`
- `frontend/src/app/(auth)/login/page.tsx`
- `frontend/src/app/(portal)/layout.tsx`
- `frontend/src/app/(portal)/patients/page.tsx`
- `frontend/src/app/(portal)/agenda/page.tsx`
