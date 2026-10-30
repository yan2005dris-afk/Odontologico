# Plan de Work Item — `wi-04-clinical-odontogram-perio`

- **Feature Padre:** [`pdd/tasks/03-clinical-odontogram-soep.md`](../tasks/03-clinical-odontogram-soep.md)
- **Worktree / Rama Asignada:** `wt/clinical-agenda`
- **Directorio Objetivo:** `services/clinical-agenda/internal/clinical/` (archivos `odontogram*` y `periodontogram*`), `services/clinical-agenda/migrations/00003_odontogram_perio.sql`
- **Referencias SRS:** `RF-03`, `RF-04`, `RN-01`, `RNF-01`, `RNF-05`, `RNF-06`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Implementación de los subdominios de Odontograma 2D (`RF-03`) y Periodontograma Digital (`RF-04`) dentro de `services/clinical-agenda/internal/clinical/` siguiendo el patrón Hexagonal Pattern B.
- Migración SQL `00003_odontogram_perio.sql` con tablas `odontogram_snapshots`, `odontogram_findings`, `periodontogram_charts` y `periodontogram_sites`.
- Odontograma Gráfico Interactivo (`RF-03`):
  - Soporte completo para dentición permanente (32 dientes: FDI `11..18, 21..28, 31..38, 41..48` $\leftrightarrow$ Universal `1..32`) y dentición temporal/decidua (20 dientes: FDI `51..55, 61..65, 71..75, 81..85` $\leftrightarrow$ Universal `A..T`).
  - Traductor bidireccional exacto entre nomenclatura FDI y Universal americana.
  - Marcado por superficies dentales (`MESIAL`, `DISTAL`, `OCLUSAL_INCISAL`, `VESTIBULAR`, `LINGUAL_PALATINO`, `WHOLE_TOOTH`).
  - Catálogo visual estandarizado: `CARIES` (color semántico `#DC2626` rojo), `EXISTING_RESTORATION` (`#2563EB` azul), `ENDODONTICS_DONE`, `ENDODONTICS_REQUIRED`, `IMPLANT`, `FIXED_PROSTHESIS`, `REMOVABLE_PROSTHESIS`, `EXTRACTION_DONE`, `EXTRACTION_INDICATED`, `SEALANT`.
- Regla de negocio `RN-01` en hallazgos/procedimientos del odontograma: ningún hallazgo o procedimiento marcado con estado `"REALIZADO"` puede ser eliminado ni sobrescrito; sólo admite adenda correctiva firmada.
- Periodontograma Digital (`RF-04`):
  - Captura por diente de los 6 puntos de sondaje (`MB`, `B`, `DB`, `ML`, `L`, `DL`) para profundidad al sondaje (`probing_depth_mm`) y margen gingival (`gingival_margin_mm`).
  - Cálculo automático del Nivel de Inserción Clínica ($\text{NIC} = \text{probing\_depth\_mm} - \text{gingival\_margin\_mm}$) por punto y promedio general, lesión de furca (`0`, `I`, `II`, `III`), movilidad dental (`0`, `1`, `2`, `3`) y sangrado al sondaje (`bleeding_on_probing` booleano por sitio + porcentaje global).
- Endpoint consolidado de alta velocidad para cumplir `RNF-01` ($< 1.5\text{ s}$ de carga de expediente + odontograma).

### Fuera del alcance (Out of Scope)
- Notas de evolución SOEP y consentimientos informados digitales (cubiertos en `wi-05-clinical-soep-consents` en archivos independientes dentro de `internal/clinical/`).

---

## 2. Dependencias previas

- **Work Items previos:** `wi-02-clinical-identity-audit` (auditoría `RNF-06` y RBAC/MFA) y `wi-03-clinical-patients` (existencia de `patients`).
- **Migraciones previas:** `00001_identity_audit.sql`, `00002_patients_anamnesis.sql`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Validación anatómica de 52 dientes y conversión bidireccional FDI $\leftrightarrow$ Universal (`RF-03`) | El dominio valida los 32 dientes permanentes (`11..48` $\leftrightarrow$ `1..32`) y los 20 deciduos (`51..85` $\leftrightarrow$ `A..T`). Códigos FDI inexistentes (ej. `19`, `56`, `99`) retornan `422 Unprocessable Entity` (`INVALID_TOOTH_CODE`). | Unit + Adversarial |
| **S2** | Marcado por 5 superficies y catálogo visual estandarizado (`RF-03`) | `POST /api/v1/patients/{id}/odontogram/findings` registra hallazgos sobre `MESIAL`, `DISTAL`, `OCLUSAL_INCISAL`, `VESTIBULAR`, `LINGUAL_PALATINO` o `WHOLE_TOOTH` asignando automáticamente el código cromático estandarizado (`#DC2626` para `CARIES`, `#2563EB` para `EXISTING_RESTORATION`, etc.). | Unit + Integration |
| **S3** | Inmutabilidad de hallazgos/procedimientos `"REALIZADO"` (`RN-01`) | Intentar `DELETE /api/v1/patients/{id}/odontogram/findings/{findingId}` o `PUT` sobre un registro con `status == "REALIZADO"` falla con `409 Conflict` (`CLINICAL_RECORD_IMMUTABLE`). Sólo se permite registrar una adenda vinculada mediante `POST .../findings/{findingId}/addendum`. | Unit + Integration + Adversarial |
| **S4** | Periodontograma de 6 sitios por diente y cálculo automático de NIC (`RF-04`) | `POST /api/v1/patients/{id}/periodontograms` exige exactamente 6 sitios (`MB, B, DB, ML, L, DL`) por diente evaluado, calcula automáticamente $\text{nic\_mm}[i] = \text{probing\_depth\_mm}[i] - \text{gingival\_margin\_mm}[i]$ para cada sitio $i \in \{0..5\}$, valida furca (`0..3`) y movilidad (`0..3`), y calcula `% BOP`. | Unit + Integration + Adversarial |
| **S5** | Rendimiento de carga clínica $< 1.5\text{ s}$ (`RNF-01`) | `GET /api/v1/patients/{id}/odontogram` devuelve los 52 dientes con sus hallazgos vigentes y notación dual FDI/Universal en $< 100\text{ ms}$ en pruebas de integración locales, emitiendo registro en `audit_logs` (`RNF-06`). | Integration |

---

## 4. Contrato de entrada

### `POST /api/v1/patients/{id}/odontogram/findings`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist` | `assistant`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `tooth_code` (`string`, FDI `"11".."85"` o Universal `"1".."32"`, `"A".."T"`, requerido).
  - `notation_system` (`string`: `"FDI"` | `"UNIVERSAL"`, requerido).
  - `surface` (`string`: `"MESIAL"` | `"DISTAL"` | `"OCLUSAL_INCISAL"` | `"VESTIBULAR"` | `"LINGUAL_PALATINO"` | `"WHOLE_TOOTH"`, requerido).
  - `finding_type` (`string`: `"CARIES"` | `"EXISTING_RESTORATION"` | `"ENDODONTICS_DONE"` | `"ENDODONTICS_REQUIRED"` | `"IMPLANT"` | `"FIXED_PROSTHESIS"` | `"REMOVABLE_PROSTHESIS"` | `"EXTRACTION_DONE"` | `"EXTRACTION_INDICATED"` | `"SEALANT"`, requerido).
  - `status` (`string`: `"DIAGNOSED"` | `"PLANNED"` | `"REALIZADO"`, requerido).
  - `notes` (`string`, opcional).

### `POST /api/v1/patients/{id}/periodontograms`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist` | `assistant`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `teeth`: array de objetos por diente evaluado:
    - `tooth_fdi` (`string`, requerido).
    - `probing_depth_mm` (`[6]int`, orden `[MB, B, DB, ML, L, DL]`, rango `0..15`, requerido).
    - `gingival_margin_mm` (`[6]int`, orden `[MB, B, DB, ML, L, DL]`, rango `-10..10`, requerido).
    - `bleeding_on_probing` (`[6]bool`, orden `[MB, B, DB, ML, L, DL]`, requerido).
    - `furcation_grade` (`int`, `0..3`, requerido).
    - `mobility_grade` (`int`, `0..3`, requerido).

---

## 5. Contrato de salida

### Respuesta `GET /api/v1/patients/{id}/odontogram` (`200 OK`)
```json
{
  "patient_id": "2c5ea4c0-4067-11e9-8bad-9b1deb4d3b7d",
  "dentition_mode": "MIXED",
  "findings": [
    {
      "id": "7f9c2b10-12a4-4f88-9b01-112233445566",
      "tooth_fdi": "16",
      "tooth_universal": "3",
      "dentition_type": "PERMANENT",
      "surface": "OCLUSAL_INCISAL",
      "finding_type": "CARIES",
      "color_hex": "#DC2626",
      "status": "DIAGNOSED",
      "immutable": false,
      "recorded_by": "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
      "recorded_at": "2026-10-30T15:10:00Z"
    }
  ]
}
```

### Respuesta `POST /api/v1/patients/{id}/periodontograms` (`201 Created`)
```json
{
  "id": "9a8b7c6d-5e4f-3a2b-1c0d-fedcba987654",
  "patient_id": "2c5ea4c0-4067-11e9-8bad-9b1deb4d3b7d",
  "bop_percentage": 16.67,
  "mean_nic_mm": 3.5,
  "teeth": [
    {
      "tooth_fdi": "16",
      "probing_depth_mm": [3, 2, 4, 3, 2, 3],
      "gingival_margin_mm": [0, -1, -1, 0, 0, -1],
      "nic_mm": [3, 3, 5, 3, 2, 4],
      "bleeding_on_probing": [false, false, true, false, false, false],
      "furcation_grade": 1,
      "mobility_grade": 0
    }
  ],
  "recorded_at": "2026-10-30T15:15:00Z"
}
```

### Matriz de Estados HTTP
- `200 OK` / `201 Created`: Lectura o registro exitoso.
- `403 Forbidden`: Rol no clínico (`receptionist`) intentando modificar odontograma/periodontograma o `MFA_REQUIRED`.
- `404 Not Found`: Paciente o hallazgo inexistente.
- `409 Conflict`: Intento de borrar o mutar un hallazgo/procedimiento con estado `"REALIZADO"` (`RN-01`).
- `422 Unprocessable Entity`: Diente fuera de catálogo FDI/Universal, superficie inválida, o arreglo de sondaje distinto de 6 puntos.

---

## 6. Seguridad y resiliencia

- **Inmutabilidad Clínica (`RN-01`):** Validación en la entidad de dominio `OdontogramFinding` y trigger/constraint en PostgreSQL que impide `DELETE` o cambio destructivo cuando `status = 'REALIZADO'`.
- **Auditoría (`RNF-06`):** Cada lectura y mutación del odontograma y periodontograma invoca `audit.Recorder` con IP de origen y timestamp ISO-8601.
- **Presupuesto de Latencia (`RNF-01`):** Índices compuestos `(patient_id, recorded_at DESC)` en PostgreSQL para asegurar lecturas en una sola consulta sin $N+1$.

---

## 7. Archivos afectados

- `services/clinical-agenda/migrations/00003_odontogram_perio.sql`
- `services/clinical-agenda/internal/clinical/domain/odontogram.go`
- `services/clinical-agenda/internal/clinical/domain/periodontogram.go`
- `services/clinical-agenda/internal/clinical/domain/odontogram_test.go`
- `services/clinical-agenda/internal/clinical/domain/periodontogram_test.go`
- `services/clinical-agenda/internal/clinical/application/odontogram.port.go`
- `services/clinical-agenda/internal/clinical/application/odontogram.service.go`
- `services/clinical-agenda/internal/clinical/application/odontogram.service_test.go`
- `services/clinical-agenda/internal/clinical/application/periodontogram.port.go`
- `services/clinical-agenda/internal/clinical/application/periodontogram.service.go`
- `services/clinical-agenda/internal/clinical/application/periodontogram.service_test.go`
- `services/clinical-agenda/internal/clinical/adapters/postgres/odontogram_repository.go`
- `services/clinical-agenda/internal/clinical/adapters/postgres/periodontogram_repository.go`
- `services/clinical-agenda/internal/clinical/adapters/http/odontogram_handler.go`
- `services/clinical-agenda/internal/clinical/adapters/http/odontogram_handler_test.go`
