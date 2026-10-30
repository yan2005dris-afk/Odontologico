# Plan de Work Item — `wi-03-clinical-patients`

- **Feature Padre:** [`pdd/tasks/02-patients-anamnesis.md`](../tasks/02-patients-anamnesis.md)
- **Worktree / Rama Asignada:** `wt/clinical-agenda`
- **Directorio Objetivo:** `services/clinical-agenda/internal/patients/`, `services/clinical-agenda/migrations/00002_patients_anamnesis.sql`
- **Referencias SRS:** `RF-01`, `RF-02`, `RNF-01`, `RNF-05`, `RNF-06`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Implementación del contexto `patients` en `services/clinical-agenda/internal/patients/` siguiendo arquitectura Hexagonal Pattern B (`domain`, `application`, `adapters`).
- Migración SQL `00002_patients_anamnesis.sql` con tablas `patients`, `patient_anamnesis` y `patient_critical_alerts`, con índices de búsqueda rápida por documento y nombre para cumplir $< 1.5\text{ s}$ (`RNF-01`).
- Registro unificado de paciente (`RF-01`): alta, consulta, búsqueda paginada y actualización de datos demográficos, contacto de emergencia obligatorio, validación estricta de tutor legal (`legal_guardian`) cuando el paciente es menor de 18 años, y datos de seguro dental/convenio.
- Anamnesis e historial médico general obligatorio (`RF-02`): captura estructurada de enfermedades sistémicas, alergias a fármacos, antecedentes quirúrgicos y coagulopatías/anticoagulantes.
- Motor de derivación automática de alertas médicas críticas de alta visibilidad (`RF-02`): generación de `critical_alerts[]` (ej. `PENICILLIN_ALLERGY`, `ANTICOAGULANT_THERAPY`, `COAGULOPATHY`, `BISPHOSPHONATES`, `ENDOCARDITIS_RISK`, `UNCONTROLLED_SYSTEMIC`) expuestas permanentemente en el resumen de cabecera del expediente.
- Integración obligatoria con el puerto `audit.Recorder` (`RNF-06`) en cada alta, lectura y modificación de ficha o anamnesis.

### Fuera del alcance (Out of Scope)
- Odontograma 2D, periodontograma, notas SOEP y consentimientos (cubiertos en `wi-04` y `wi-05`).
- Componentes visuales en Next.js (cubiertos en `wi-10-frontend-shell-patients-agenda`).

---

## 2. Dependencias previas

- **Work Items previos:** `wi-02-clinical-identity-audit` (provee `internal/audit/application/audit.port.go` y middleware RBAC/MFA).
- **Migraciones previas:** `00001_identity_audit.sql`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Alta y modificación de ficha única con regla de menor de edad (`RF-01`) | `POST /api/v1/patients` y `PUT /api/v1/patients/{id}` calculan la edad exacta a partir de `birth_date`. Si `age < 18` y `legal_guardian` está vacío o incompleto (`full_name`, `document_number`, `phone`, `relationship`), retorna `422 Unprocessable Entity` (`LEGAL_GUARDIAN_REQUIRED_FOR_MINOR`). Si `document_number` ya existe, retorna `409 Conflict` (`DUPLICATE_PATIENT_DOCUMENT`). | Unit + Integration + Adversarial |
| **S2** | Consulta y búsqueda indexada de pacientes (`RF-01`, `RNF-01`) | `GET /api/v1/patients?q=<term>&limit=&offset=` filtra por número de documento, número de historia clínica (`hc_number`) o nombre/apellido en $< 100\text{ ms}$. | Integration |
| **S3** | Registro de anamnesis estructurada (`RF-02`) | `PUT /api/v1/patients/{id}/anamnesis` exige sesión `mfa_verified=true` con rol `dentist` o `assistant`, valida la estructura de `systemic_diseases[]`, `drug_allergies[]`, `surgical_history[]` y `coagulopathies[]`, y guarda una nueva versión firmada por el profesional. | Unit + Integration |
| **S4** | Derivación de *badges* de alerta médica crítica en cabecera (`RF-02`) | Si `drug_allergies` contiene alergia severa/anafiláctica (ej. penicilina, AINEs, anestésicos locales, látex) o `coagulopathies` / `current_medications` incluye anticoagulantes/trastornos de coagulación, `DeriveCriticalAlerts` genera badges con `severity: "CRITICAL"` o `"HIGH"`, disponibles inmediatamente en `GET /api/v1/patients/{id}` y `GET /api/v1/patients/{id}/header`. | Unit + Integration + Adversarial |
| **S5** | Trazabilidad completa en bitácora de auditoría (`RNF-06`) | Toda invocación a `CreatePatient`, `GetPatient`, `UpdatePatient` y `UpsertAnamnesis` registra un evento inmutable en `audit_logs` con `actor_id`, `action`, `resource_id`, `timestamp_iso8601` y `source_ip`. | Unit + Integration |

---

## 4. Contrato de entrada

### `POST /api/v1/patients` & `PUT /api/v1/patients/{id}`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`admin` | `receptionist` | `dentist` | `assistant`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `document_type` (`string`: `DNI` | `PASSPORT` | `FOREIGN_ID`, requerido).
  - `document_number` (`string`, 5..30 chars, requerido).
  - `first_name` (`string`, requerido), `last_name` (`string`, requerido).
  - `birth_date` (`string`, `YYYY-MM-DD`, requerido, no futuro).
  - `sex` (`string`: `F` | `M` | `X`, requerido).
  - `phone` (`string`, requerido), `email` (`string`, opcional).
  - `emergency_contact` (`object`, requerido): `{ "full_name": string, "phone": string, "relationship": string }`.
  - `legal_guardian` (`object`, obligatorio si `age < 18`): `{ "full_name": string, "document_number": string, "phone": string, "relationship": string }`.
  - `insurance_plan` (`object`, opcional): `{ "provider_name": string, "plan_name": string, "member_number": string }`.

### `PUT /api/v1/patients/{id}/anamnesis`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist` | `assistant`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `systemic_diseases`: `[{ "code": string, "name": string, "controlled": bool, "notes": string }]`
  - `drug_allergies`: `[{ "substance": string, "reaction": string, "severity": "MILD" | "MODERATE" | "CRITICAL" }]`
  - `surgical_history`: `[{ "procedure": string, "year": int, "complications": string }]`
  - `coagulopathies`: `[{ "condition": string, "medication": string, "inr_last_value": float64, "is_anticoagulated": bool }]`
  - `is_pregnant` (`bool`), `notes` (`string`).

---

## 5. Contrato de salida

### Respuesta `GET /api/v1/patients/{id}` (`200 OK`)
```json
{
  "id": "2c5ea4c0-4067-11e9-8bad-9b1deb4d3b7d",
  "hc_number": "HC-2026-000142",
  "document_type": "DNI",
  "document_number": "40123987",
  "first_name": "Martín",
  "last_name": "Echeverría",
  "birth_date": "1982-04-15",
  "age": 44,
  "is_minor": false,
  "emergency_contact": {
    "full_name": "Laura Echeverría",
    "phone": "+5491144556677",
    "relationship": "Spouse"
  },
  "insurance_plan": {
    "provider_name": "OSDE",
    "plan_name": "310",
    "member_number": "610293847"
  },
  "anamnesis_recorded": true,
  "critical_alerts": [
    {
      "code": "ALLERGY_PENICILLIN",
      "label": "ALERGIA CRÍTICA: PENICILINA (Anafilaxia)",
      "severity": "CRITICAL",
      "category": "DRUG_ALLERGY"
    },
    {
      "code": "ANTICOAGULANT_ACTIVE",
      "label": "ANTICOAGULADO: Warfarina (INR 2.8)",
      "severity": "CRITICAL",
      "category": "COAGULOPATHY"
    }
  ],
  "updated_at": "2026-10-30T15:00:00Z"
}
```

### Matriz de Estados HTTP
- `200 OK`: Consulta, búsqueda o actualización exitosa.
- `201 Created`: Paciente creado exitosamente.
- `400 Bad Request`: JSON inválido o fecha malformada.
- `403 Forbidden`: Rol sin permiso (ej. `receptionist` intentando modificar `anamnesis`) o `MFA_REQUIRED`.
- `404 Not Found`: Paciente inexistente.
- `409 Conflict`: `DUPLICATE_PATIENT_DOCUMENT`.
- `422 Unprocessable Entity`: `LEGAL_GUARDIAN_REQUIRED_FOR_MINOR` o campos obligatorios vacíos.

---

## 6. Seguridad y resiliencia

- **Control RBAC y MFA (`SRS §3`, `RNF-05`):** La lectura/escritura de anamnesis médica exige `mfa_verified=true`. Los recepcionistas pueden gestionar la filiación administrativa (`RF-01`) y visualizar los *badges* de alerta para precaución administrativa, pero sólo `dentist` y `assistant` pueden alterar los datos clínicos de `RF-02`.
- **Auditoría Inmutable (`RNF-06`):** La actualización de paciente o anamnesis y el registro en `audit_logs` se ejecutan de manera consistente registrando el `actor_id` y `source_ip`.
- **Prevención de Inyección y Sanitización:** Consultas SQL 100% parametrizadas; normalización de números de documento y escape de strings.

---

## 7. Archivos afectados

- `services/clinical-agenda/migrations/00002_patients_anamnesis.sql`
- `services/clinical-agenda/internal/patients/domain/patient.go`
- `services/clinical-agenda/internal/patients/domain/anamnesis.go`
- `services/clinical-agenda/internal/patients/domain/patient_test.go`
- `services/clinical-agenda/internal/patients/application/patient.port.go`
- `services/clinical-agenda/internal/patients/application/patient.service.go`
- `services/clinical-agenda/internal/patients/application/patient.service_test.go`
- `services/clinical-agenda/internal/patients/adapters/postgres/patient_repository.go`
- `services/clinical-agenda/internal/patients/adapters/http/patient_handler.go`
- `services/clinical-agenda/internal/patients/adapters/http/patient_handler_test.go`
