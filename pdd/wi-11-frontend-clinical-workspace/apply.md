# Registro de Implementación (`apply`) — `wi-11-frontend-clinical-workspace`

- **Work Item:** [`pdd/wi-11-frontend-clinical-workspace/plan.md`](./plan.md)
- **Feature Padre:** [`pdd/tasks/07-frontend-portal-ui.md`](../tasks/07-frontend-portal-ui.md)
- **Worktree / Rama:** `wt/frontend`
- **Estado:** `COMPLETED` (TDD RED $\rightarrow$ GREEN $\rightarrow$ REFACTOR)

---

## 1. Cobertura de Especificaciones (`S1..S5`)

| Spec | Requisito SRS / Regla | Implementación y Evidencia | Estado |
| :--- | :--- | :--- | :--- |
| **S1** | Odontograma 2D interactivo (32 permanentes + 20 deciduos = 52 piezas) con toggle FDI/Universal, 5 superficies y render $< 100\text{ ms}$ (`RF-03`, `RNF-01`) | Implementado en `frontend/src/components/clinical/OdontogramCanvas.tsx` (`ALL_52_FDI_TEETH`, `fdiToUniversal`, `universalToFdi`, `DENTAL_SURFACES`, `FINDING_CATALOG_COLORS` con `#DC2626` para caries y `#2563EB` para restauración existente sobre lienzo `#FFFFFF`). Render verificado en $\approx 1.5\text{ ms}$ ($< 100\text{ ms}$). | **PASS** |
| **S2** | Protección de inmutabilidad en Odontograma (`status === "REALIZADO"`) y Notas SOEP bloqueadas tras 24h con Adendas Firmadas (`RN-01`, `RF-05`) | Implementado en `OdontogramCanvas.tsx` (`canMutateOdontogramFinding`) y `frontend/src/components/clinical/SoepEvolutionTimeline.tsx` (`isSoepNoteLockedByWindow`, `validateSoepAddendumInput`), deshabilitando edición/borrado y habilitando únicamente `Agregar Adenda Firmada` vía `SlideOverSheet`. | **PASS** |
| **S3** | Periodontograma de 6 sitios (`MB, B, DB, ML, L, DL`) con cálculo automático de $\text{NIC} = \text{PD} - \text{GM}$ y `tabular-nums` (`RF-04`) | Implementado en `frontend/src/components/clinical/PeriodontogramGrid.tsx` (`calculateClinicalAttachmentLevel`, `computeToothPeriodontalMetrics`, `renderPeriodontogramGridHtml`). | **PASS** |
| **S4** | Firma de Consentimiento Informado Digital por biometría táctil ($\ge 44 \times 44\text{ px}$) o código OTP de 6 dígitos (`RF-06`, `RNF-07`) | Implementado en `frontend/src/components/clinical/InformedConsentSheet.tsx` (`validateInformedConsentSubmission`, `renderInformedConsentSheetHtml`). | **PASS** |
| **S5** | Presupuesto multinivel con colapso automático de fases completadas y aprobación parcial/total hacia tareas agendables (`RF-07`, `RF-08`, `AGENTS.md §3`) | Implementado en `frontend/src/components/treatments/TreatmentPlanWorkspace.tsx` (`buildTreatmentPlanViewModel`, `renderTreatmentPlanWorkspaceHtml`) y orquestado en `frontend/src/app/(portal)/patients/[id]/page.tsx`. | **PASS** |

---

## 2. Archivos Creados / Modificados

- `frontend/src/components/clinical/OdontogramCanvas.tsx`
- `frontend/src/components/clinical/OdontogramCanvas.test.tsx`
- `frontend/src/components/clinical/PeriodontogramGrid.tsx`
- `frontend/src/components/clinical/PeriodontogramGrid.test.tsx`
- `frontend/src/components/clinical/SoepEvolutionTimeline.tsx`
- `frontend/src/components/clinical/SoepEvolutionTimeline.test.tsx`
- `frontend/src/components/clinical/InformedConsentSheet.tsx`
- `frontend/src/components/clinical/InformedConsentSheet.test.tsx`
- `frontend/src/components/treatments/TreatmentPlanWorkspace.tsx`
- `frontend/src/components/treatments/TreatmentPlanWorkspace.test.tsx`
- `frontend/src/app/(portal)/patients/[id]/page.tsx`
- `frontend/tests/domain-invariants.test.mjs`

---

## 3. Comandos Ejecutados y Salida Real Observada

### 3.1 Suite Unitaria y de Integración (`wi-11`)

Comando:
```bash
cd frontend && npm run test:wi11
```

Salida real:
```text
npm notice run sigo-dental-frontend@0.1.0 test:wi11
npm notice run node --import ./vitest.config.ts --test src/components/clinical/OdontogramCanvas.test.tsx src/components/clinical/PeriodontogramGrid.test.tsx src/components/clinical/SoepEvolutionTimeline.test.tsx src/components/clinical/InformedConsentSheet.test.tsx src/components/treatments/TreatmentPlanWorkspace.test.tsx
▶ wi-11 S4 — Digital Informed Consent Sheet (Biometric Touch Pad & 6-Digit OTP RF-06)
  ✔ blocks submission in BIOMETRIC_PAD mode when SVG stroke points are empty or insufficient (S4) (1.925298ms)
  ✔ blocks submission in OTP_CODE mode unless OTP has exactly 6 numeric digits (S4) (0.292832ms)
  ✔ renders SlideOverSheet dialog with touch target >= 44x44px and mode switcher (RNF-07, S4) (0.476718ms)
✔ wi-11 S4 — Digital Informed Consent Sheet (Biometric Touch Pad & 6-Digit OTP RF-06) (4.33576ms)
▶ wi-11 S1 & S2 — 2D Interactive SVG Odontogram (52 teeth, FDI <-> Universal, 5 surfaces, RN-01 lock)
  ✔ exposes all 32 permanent teeth and 20 deciduous teeth (52 total) and 5 dental surfaces (RF-03) (2.15976ms)
  ✔ converts bidirectionally between FDI (11..48, 51..85) and Universal (1..32, A..T) notation (S1) (0.241685ms)
  ✔ assigns strict semantic colors: red (#DC2626) for CARIES and blue (#2563EB) for EXISTING_RESTORATION (AGENTS.md §3) (0.139362ms)
  ✔ enforces RN-01 immutability: findings marked as REALIZADO cannot be edited or deleted (S2) (0.191781ms)
  ✔ renders all 52 teeth in < 100ms on pure white (#FFFFFF) canvas with tabular-nums and RN-01 lock indicators (RNF-01, S1, S2) (2.041758ms)
✔ wi-11 S1 & S2 — 2D Interactive SVG Odontogram (52 teeth, FDI <-> Universal, 5 surfaces, RN-01 lock) (6.588025ms)
▶ wi-11 S3 — Digital Periodontogram (6 sites/tooth, NIC = PD - GM calculation & tabular-nums RF-04)
  ✔ defines the 6 standard probing sites per tooth (MB, B, DB, ML, L, DL) (1.260586ms)
  ✔ calculates Clinical Attachment Level (NIC = PD - GM) accurately across all 6 sites (S3) (0.219534ms)
  ✔ flags pathological pockets (PD >= 4mm) and renders tabular-nums on all numeric cells (S3, AGENTS.md §3) (1.392093ms)
✔ wi-11 S3 — Digital Periodontogram (6 sites/tooth, NIC = PD - GM calculation & tabular-nums RF-04) (4.199294ms)
▶ wi-11 S2 — Immutable SOEP Evolution Notes (24h lock window & Signed Addenda RF-05, RN-01)
  ✔ locks SOEP notes automatically once 24 hours have elapsed or when isLocked flag is set (RN-01) (3.203126ms)
  ✔ disables edit/delete buttons on locked SOEP notes and enables only 'Agregar Adenda Firmada' (S2) (0.649344ms)
  ✔ validates signed addendum input requiring non-empty clinical text and professional signature confirmation (0.225054ms)
✔ wi-11 S2 — Immutable SOEP Evolution Notes (24h lock window & Signed Addenda RF-05, RN-01) (5.505876ms)
▶ wi-11 S5 — Multi-Phase Treatment Plans, Auto-Collapsed Completed Phases & Partial/Total Approval (RF-07, RF-08)
  ✔ auto-collapses phases where all items are REALIZADO to emphasize what's next (AGENTS.md §3, S5) (1.312243ms)
  ✔ recalculates partial approval subtotal and transitions selected items into schedulable tasks (RF-08, S5) (2.597996ms)
✔ wi-11 S5 — Multi-Phase Treatment Plans, Auto-Collapsed Completed Phases & Partial/Total Approval (RF-07, RF-08) (5.717004ms)
ℹ tests 16
ℹ suites 5
ℹ pass 16
ℹ fail 0
ℹ duration_ms 146.73597
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

- **Divulgación Progresiva (`AGENTS.md §3`):** `patients/[id]/page.tsx` mantiene fijo el `StickyPatientHeader` (Nivel 1), separa las herramientas clínicas en pestañas dedicadas (Nivel 2) sobre un lienzo `#FFFFFF` y colapsa por defecto las fases de tratamiento cuyos procedimientos están todos en estado `"REALIZADO"` (Nivel 3).
- **Inmutabilidad Clínica (`RN-01`):** Tanto el Odontograma 2D como la línea de tiempo SOEP bloquean el borrado/edición de registros consolidados y encauzan cualquier corrección hacia una Adenda Firmada en un `SlideOverSheet`.
- **Límites Respetados:** Modificaciones estrictamente acotadas a los archivos declarados en `wi-11-frontend-clinical-workspace/plan.md`.
