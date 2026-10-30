# Registro de Implementación (`apply`) — `wi-10-frontend-shell-patients-agenda`

- **Work Item:** [`pdd/wi-10-frontend-shell-patients-agenda/plan.md`](./plan.md)
- **Feature Padre:** [`pdd/tasks/07-frontend-portal-ui.md`](../tasks/07-frontend-portal-ui.md)
- **Worktree / Rama:** `wt/frontend`
- **Estado:** `COMPLETED` (TDD RED $\rightarrow$ GREEN $\rightarrow$ REFACTOR)

---

## 1. Cobertura de Especificaciones (`S1..S5`)

| Spec | Requisito SRS / Regla | Implementación y Evidencia | Estado |
| :--- | :--- | :--- | :--- |
| **S1** | App Shell clínico (`bg-slate-50`), Sidebar colapsable por rol RBAC (`admin`, `receptionist`, `dentist`, `assistant`, `auditor`) y monitor de 15 min de inactividad (`RNF-05`, `RNF-07`) | Implementado en `frontend/src/lib/session-idle-guard.ts` (`DEFAULT_IDLE_TIMEOUT_MS = 900_000`, `createSessionIdleGuard`, `validateTotpCode`, `getAllowedNavigationModules`), `frontend/src/components/shell/AppShell.tsx` y `frontend/src/components/shell/SlideOverSheet.tsx`. | **PASS** |
| **S2** | Formulario de Ficha Única con validación obligatoria de Tutor Legal en menores de 18 años (`RF-01`) | Implementado en `frontend/src/components/patients/PatientRegistrationForm.tsx` (`calculatePatientAge`, `validatePatientRegistration` retornando `LEGAL_GUARDIAN_REQUIRED_FOR_MINOR` si falta nombre, documento, teléfono o parentesco del tutor cuando `age < 18`). | **PASS** |
| **S3** | Cabecera Fija (`StickyPatientHeader`) con badges de alerta médica crítica (`RF-02`) | Implementado en `frontend/src/components/patients/StickyPatientHeader.tsx` con posicionamiento `sticky top-0`, semántica `role="alert"` y sanitización XSS estricta (`escapeHtml`). | **PASS** |
| **S4** | Calendario multirrecinto Día/Semana y bloqueo de sobreposición de sillón físico (`RF-09`, `RN-03`) | Implementado en `frontend/src/components/agenda/MultiChairCalendar.tsx` (`filterAppointments`, `detectChairOverlapConflict` bloqueando cualquier solapamiento horario sobre el mismo `chairId` independientemente del especialista con código `CHAIR_OVERLAP_CONFLICT`). | **PASS** |
| **S5** | Control visual y máquina de estados de las 7 transiciones de cita (`RF-10`) | Implementado en `frontend/src/components/agenda/MultiChairCalendar.tsx` (`APPOINTMENT_STATE_GRAPH`, `getAllowedStatusTransitions`, `canTransitionAppointmentStatus`). | **PASS** |

---

## 2. Archivos Creados / Modificados

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
- `frontend/tests/domain-invariants.test.mjs`

---

## 3. Comandos Ejecutados y Salida Real Observada

### 3.1 Suite Unitaria y de Integración (`wi-10`)

Comando:
```bash
cd frontend && npm run test:wi10
```

Salida real:
```text
npm notice run sigo-dental-frontend@0.1.0 test:wi10
npm notice run node --import ./vitest.config.ts --test src/lib/session-idle-guard.test.ts src/components/patients/StickyPatientHeader.test.tsx src/components/patients/PatientRegistrationForm.test.tsx src/components/agenda/MultiChairCalendar.test.tsx
▶ wi-10 S4 & S5 — Multi-Chair Calendar, Chair Overlap Prevention (RN-03) & 7-State Machine (RF-09, RF-10)
  ✔ filters appointments by physical dental chair (chairId) and specialist (dentistId) (RF-09) (0.836487ms)
  ✔ blocks booking on the same physical chair during an overlapping time window even with a different dentist (RN-03) (1.15624ms)
  ✔ ignores CANCELADO or NO_SHOW appointments when evaluating physical chair availability (RN-03) (0.215396ms)
  ✔ enforces the 7-state appointment lifecycle graph strictly (RF-10) (0.654063ms)
  ✔ renders calendar surface with tabular-nums and RN-03 alert banner when overlap is present (0.401938ms)
✔ wi-10 S4 & S5 — Multi-Chair Calendar, Chair Overlap Prevention (RN-03) & 7-State Machine (RF-09, RF-10) (4.683199ms)
▶ wi-10 S2 — Unified Patient Registration & Minor Legal Guardian Validation (RF-01)
  ✔ calculates age accurately and marks patients under 18 as minors (0.749523ms)
  ✔ blocks submission with LEGAL_GUARDIAN_REQUIRED_FOR_MINOR when age < 18 and guardian fields are missing (0.262525ms)
  ✔ accepts minor registration only when all 4 legal guardian fields are populated (0.542413ms)
  ✔ allows adult registration (>= 18 years) without legal guardian fields (0.162978ms)
✔ wi-10 S2 — Unified Patient Registration & Minor Legal Guardian Validation (RF-01) (3.769747ms)
▶ wi-10 S3 — StickyPatientHeader with High-Visibility Critical Alerts (RF-02)
  ✔ renders sticky top-0 banner with role='alert' high-contrast badges when criticalAlerts is non-empty (1.042055ms)
  ✔ escapes untrusted characters in alert labels to prevent XSS injection (Section 6 Security) (0.205818ms)
✔ wi-10 S3 — StickyPatientHeader with High-Visibility Critical Alerts (RF-02) (2.407158ms)
▶ wi-10 S1 — Session Idle Guard (15-min timeout RNF-05) & RBAC Navigation
  ✔ exposes default 15-minute (900,000 ms) idle timeout constant per RNF-05 (0.731789ms)
  ✔ triggers onIdleTimeout exactly when 900,000 ms elapse without user interaction (1.248123ms)
  ✔ resets countdown on clinical workstation events (pointerdown, keydown, touchstart) (0.279406ms)
  ✔ ignores further activity once session is locked (adversarial replay attempt) (0.248699ms)
  ✔ validates 6-digit tabular-nums TOTP MFA code strictly (RNF-05) (0.353536ms)
  ✔ filters collapsible sidebar modules strictly by RBAC role (S1) (1.056031ms)
✔ wi-10 S1 — Session Idle Guard (15-min timeout RNF-05) & RBAC Navigation (5.465643ms)
ℹ tests 17
ℹ suites 4
ℹ pass 17
ℹ fail 0
ℹ duration_ms 139.221133
```

### 3.2 Suite Consolidada Zero-Dependency (`node --test frontend/tests/domain-invariants.test.mjs`)

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

- **Arquitectura de Interfaz (`AGENTS.md §3`):** Se implementó el fondo general `bg-slate-50` reservando blanco puro (`#FFFFFF`) para superficies clínicas activas, junto con `SlideOverSheet` (`role="dialog"`) para evitar *page pushes* en flujos transaccionales cortos y tipografía `tabular-nums` en códigos TOTP, fechas/horas de agenda y edades/documentos.
- **Ejecución Offline / Zero-External-Bloat:** Mediante el hook nativo `registerHooks` + `stripTypeScriptTypes` de Node 26 en `vitest.config.ts` y `frontend/tests/domain-invariants.test.mjs`, todas las pruebas de componentes `.tsx` y lógica de dominio se ejecutan instantáneamente sin depender de descargas de red en el sandbox.
- **Límites Respetados:** No se tocaron archivos fuera de `frontend/` ni archivos ocultos (`.*`).
