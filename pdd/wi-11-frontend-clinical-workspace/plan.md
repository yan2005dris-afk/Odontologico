# Plan de Work Item — `wi-11-frontend-clinical-workspace`

- **Feature Padre:** [`pdd/tasks/07-frontend-portal-ui.md`](../tasks/07-frontend-portal-ui.md)
- **Worktree / Rama Asignada:** `wt/frontend`
- **Directorio Objetivo:** `frontend/src/components/clinical/`, `frontend/src/components/treatments/`, `frontend/src/app/(portal)/patients/[id]/`
- **Referencias SRS:** `RF-03`, `RF-04`, `RF-05`, `RF-06`, `RF-07`, `RF-08`, `RN-01`, `RNF-01`, `RNF-07`, `RNF-08`, Interface Architecture (`AGENTS.md §3`)

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Workspace Clínico del Paciente en `frontend/src/app/(portal)/patients/[id]/` con divulgación progresiva (`AGENTS.md §3`):
  - **Nivel 1:** `StickyPatientHeader` siempre visible con alertas críticas `RF-02`.
  - **Nivel 2:** Pestañas clínicas dedicadas (`Odontograma 2D`, `Periodontograma`, `Evolución SOEP`, `Presupuestos`, `Consentimientos`) sobre lienzo activo blanco puro (`#FFFFFF`) para evitar sobrecarga cognitiva.
- Odontograma Gráfico Interactivo 2D (`RF-03`, `RN-01`, `RNF-01`):
  - Lienzo vectorial SVG de alto rendimiento ($< 1.5\text{ s}$ de carga y renderizado total según `RNF-01`) con dentición permanente (32 dientes) y temporal/decidua (20 dientes).
  - Conmutador instantáneo de nomenclatura **FDI** (`11..48`, `51..85`) $\leftrightarrow$ **Universal** (`1..32`, `A..T`) con tipografía `tabular-nums`.
  - Selección interactiva por las 5 superficies dentales (`mesial`, `distal`, `oclusal/incisal`, `vestibular`, `lingual/palatino`) y pieza completa.
  - Catálogo visual estandarizado: caries en rojo (`#DC2626`), restauraciones existentes en azul (`#2563EB`), endodoncias, implantes, prótesis fijas/removibles, exodoncias y sellantes.
  - Bloqueo visual e interactivo contra borrado/edición de procedimientos en estado `"REALIZADO"` (`RN-01`).
- Periodontograma Digital (`RF-04`):
  - Grilla clínica táctil/teclado con `tabular-nums` obligatorio para captura de los 6 puntos por diente (`MB, B, DB, ML, L, DL`), margen gingival, cálculo en vivo del Nivel de Inserción Clínica ($\text{NIC} = \text{PD} - \text{GM}$), furca, movilidad y sangrado al sondaje (`BOP`).
- Notas de Evolución SOEP y Adendas (`RF-05`, `RN-01`):
  - Bitácora cronológica estructurada (`Subjetivo`, `Objetivo`, `Evaluación`, `Plan`) con indicador de bloqueo de 24h y *Sheet* para agregar Adendas firmadas sobre notas bloqueadas (`RN-01`).
- Consentimiento Informado Digital (`RF-06`):
  - *Slide-over Sheet* con lienzo táctil HTML5/SVG para captura de firma biométrica en tablet ($\ge 10.5''$) o validación por código OTP de 6 dígitos.
- Planes de Tratamiento y Presupuestos Multinivel (`RF-07`, `RF-08`):
  - Constructor por fases donde las fases completadas se colapsan automáticamente (`AGENTS.md §3`), con soporte de aprobación total o parcial (`RF-08`) y transición directa a tareas agendables.

### Fuera del alcance (Out of Scope)
- App Shell base, directorio de pacientes y calendario general (cubiertos en `wi-10-frontend-shell-patients-agenda`).
- Punto de venta, cierre de caja ciego e inventario (cubiertos en `wi-12-frontend-billing-inventory`).

---

## 2. Dependencias previas

- **Work Items previos:** `wi-10-frontend-shell-patients-agenda` (provee `AppShell`, `SlideOverSheet`, `StickyPatientHeader` y `api-client.ts`). Contratos backend de `wi-04`, `wi-05` y `wi-06`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Odontograma 2D interactivo (32 permanentes + 20 deciduos) con toggle FDI/Universal y 5 superficies (`RF-03`, `RNF-01`) | `OdontogramCanvas` renderiza los 52 dientes, alterna etiquetas entre FDI (`16`, `55`) y Universal (`3`, `A`) sin recargar, permite marcar cada una de las 5 superficies con el catálogo oficial (`CARIES` `#DC2626`, `EXISTING_RESTORATION` `#2563EB`, etc.) y completa su render en $< 100\text{ ms}$ en cliente. | Unit + Integration |
| **S2** | Protección de inmutabilidad en Odontograma y Notas SOEP (`RN-01`, `RF-05`) | Los hallazgos con `status === "REALIZADO"` y las notas SOEP con `is_locked === true` deshabilitan cualquier botón de eliminar o editar y habilitan únicamente el botón `Agregar Adenda Firmada` en un `SlideOverSheet`. | Unit + Integration + Adversarial |
| **S3** | Periodontograma de 6 sitios con cálculo automático de NIC y `tabular-nums` (`RF-04`) | Al ingresar profundidad de sondaje `[3, 2, 4, 3, 2, 3]` y margen gingival `[0, -1, -1, 0, 0, -1]`, `PeriodontogramGrid` calcula y muestra inmediatamente `NIC: [3, 3, 5, 3, 2, 4]` aplicando la clase `tabular-nums` en todas las celdas numéricas. | Unit + Adversarial |
| **S4** | Firma de Consentimiento Informado por biometría táctil u OTP (`RF-06`) | `InformedConsentSheet` permite alternar entre captura de trazos en el pad táctil y código OTP de 6 dígitos; impide enviar el formulario si los trazos están vacíos o el OTP no tiene 6 dígitos. | Unit + Integration |
| **S5** | Presupuesto multinivel con colapso de fases completadas y aprobación parcial/total (`RF-07`, `RF-08`) | `TreatmentPlanWorkspace` colapsa por defecto las fases con todos sus ítems `"REALIZADO"`, recalcula subtotales al seleccionar un subconjunto de ítems para aprobación parcial y dispara la generación de tareas agendables (`RF-08`). | Unit + Integration |

---

## 4. Contrato de entrada

### Props de Componentes Clínicos Principales
- `OdontogramCanvasProps`:
  - `patientId: string`, `initialNotation: 'FDI' | 'UNIVERSAL'`, `dentitionMode: 'PERMANENT' | 'DECIDUOUS' | 'MIXED'`, `findings: OdontogramFindingDTO[]`, `onApplyFinding: (payload: CreateFindingInput) => Promise<void>`.
- `PeriodontogramGridProps`:
  - `patientId: string`, `teethData: PeriodontogramToothDTO[]`, `onSaveChart: (chart: PeriodontogramChartInput) => Promise<void>`.
- `TreatmentPlanWorkspaceProps`:
  - `patientId: string`, `plan: TreatmentPlanDTO`, `onApprove: (mode: 'TOTAL' | 'PARTIAL', itemIds: string[]) => Promise<void>`.

---

## 5. Contrato de salida

### Interacciones y Mutaciones Emitidas
- `POST /api/v1/patients/{id}/odontogram/findings` (superficie + código de hallazgo).
- `POST /api/v1/patients/{id}/periodontograms` (6 sitios por diente + NIC + BOP + furca + movilidad).
- `POST /api/v1/patients/{id}/soep-notes` y `POST /api/v1/patients/{id}/soep-notes/{noteId}/addenda`.
- `POST /api/v1/patients/{id}/consents/{consentId}/sign` (trazos SVG biométricos u OTP).
- `POST /api/v1/treatment-plans/{planId}/approve` (`TOTAL` | `PARTIAL`).

---

## 6. Seguridad y resiliencia

- **Cumplimiento Estricto de `RN-01` en UI:** La interfaz nunca expone acciones de borrado para procedimientos `"REALIZADO"` ni notas SOEP bloqueadas, conduciendo toda rectificación por el flujo de Adenda firmada.
- **Ergonomía Clínica en Sillón (`RNF-07`, `AGENTS.md §3`):** Objetivos táctiles $\ge 44 \times 44\text{ px}$ en superficies dentales y pad de firma para uso con guantes en tablets $\ge 10.5''$; uso estricto de paleta neutra (`slate-50` / `#FFFFFF`) reservando rojo y azul saturados exclusivamente para hallazgos clínicos (`RF-03`) y alertas (`RF-02`).

---

## 7. Archivos afectados

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
