# Plan de Work Item — `wi-05-clinical-soep-consents`

- **Feature Padre:** [`pdd/tasks/03-clinical-odontogram-soep.md`](../tasks/03-clinical-odontogram-soep.md)
- **Worktree / Rama Asignada:** `wt/clinical-agenda`
- **Directorio Objetivo:** `services/clinical-agenda/internal/clinical/` (archivos `soep*` y `consent*`), `services/clinical-agenda/migrations/00004_soep_consents.sql`
- **Referencias SRS:** `RF-05`, `RF-06`, `RN-01`, `RNF-04`, `RNF-05`, `RNF-06`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Implementación de los subdominios de Evolución Clínica SOEP (`RF-05`, `RN-01`) y Consentimiento Informado Digital (`RF-06`) dentro de `services/clinical-agenda/internal/clinical/` en archivos disjuntos a `wi-04`.
- Migración SQL `00004_soep_consents.sql` creando `soep_notes`, `soep_addenda`, `consent_templates` y `patient_consents`.
- Notas de Evolución SOEP (`RF-05`, `RN-01`):
  - Registro cronológico estructurado en `Subjective`, `Objective`, `Assessment (Evaluación)` y `Plan`.
  - Firma digital del profesional (`signature_hash` SHA-256 sobre contenido + `dentist_id` + `signed_at`).
  - Bloqueo automático contra edición tras un plazo configurable (`lock_window_hours`, por defecto `24` horas).
  - Prohibición absoluta de borrado (`DELETE` nunca permitido) y prohibición de edición una vez vencida la ventana de 24h (`RN-01`).
  - Creación de notas de adenda (`soep_addenda`) firmadas por el profesional responsable para rectificaciones posteriores al bloqueo (`RN-01`).
- Consentimiento Informado Digital (`RF-06`):
  - Generación de documento legal parametrizado por procedimiento y paciente.
  - Firma electrónica válida por dos vías: (a) captura de firma biométrica táctil (`biometric_strokes_svg` + metadatos de presión/velocidad en tablet) o (b) código OTP de 6 dígitos enviado al paciente y validado en sesión.
  - Sellado criptográfico SHA-256 y almacenamiento del artefacto PDF en el puerto `BlobStoragePort` (S3/MinIO con cifrado en reposo `RNF-04`).

### Fuera del alcance (Out of Scope)
- Odontograma 2D y periodontograma (cubiertos en `wi-04-clinical-odontogram-perio`).
- Renderizado de canvas táctil en el navegador (cubierto en `wi-11-frontend-clinical-workspace`).

---

## 2. Dependencias previas

- **Work Items previos:** `wi-02-clinical-identity-audit` y `wi-03-clinical-patients`.
- **Migraciones previas:** `00001_identity_audit.sql`, `00002_patients_anamnesis.sql`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Creación y firma digital de nota SOEP con ventana de bloqueo de 24h (`RF-05`) | `POST /api/v1/patients/{id}/soep-notes` exige los 4 campos (`subjective`, `objective`, `assessment`, `plan`) y rol `dentist` con MFA activo; calcula `locks_at = created_at + 24h` (configurable) y genera `signature_hash` SHA-256. | Unit + Integration |
| **S2** | Bloqueo inmutable de nota SOEP tras 24h y prohibición de borrado (`RF-05`, `RN-01`) | `PUT /api/v1/patients/{id}/soep-notes/{noteId}` funciona únicamente si `now < locks_at` y el autor es el mismo profesional; si `now >= locks_at`, retorna `409 Conflict` (`SOEP_NOTE_LOCKED`). Cualquier intento de `DELETE` retorna `405 Method Not Allowed` / `409 Conflict` (`CLINICAL_RECORD_IMMUTABLE`). | Unit + Integration + Adversarial |
| **S3** | Adenda clínica firmada sobre nota bloqueada (`RN-01`) | `POST /api/v1/patients/{id}/soep-notes/{noteId}/addenda` adjunta una nota de adenda inmutable vinculada a `noteId` con su propio `signature_hash` y `signed_by` sin alterar el texto original de la nota padre. | Unit + Integration |
| **S4** | Emisión y firma de Consentimiento Informado por biometría táctil u OTP (`RF-06`) | `POST /api/v1/patients/{id}/consents/{consentId}/sign` acepta `method: "BIOMETRIC"` (con trazos SVG no vacíos) o `method: "OTP"` (validando código de 6 dígitos vigente), sella el PDF con SHA-256, lo guarda en `BlobStoragePort` y transiciona el consentimiento a `SIGNED`. | Unit + Integration + Adversarial |
| **S5** | Registro de auditoría en operaciones SOEP y Consentimientos (`RNF-06`) | Toda creación, lectura, adenda o firma digital genera una entrada en `audit_logs` con IP de origen y timestamp ISO-8601. | Integration |

---

## 4. Contrato de entrada

### `POST /api/v1/patients/{id}/soep-notes`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `appointment_id` (`string`, UUID opcional).
  - `procedure_code` (`string`, opcional).
  - `subjective` (`string`, min 5 chars, requerido).
  - `objective` (`string`, min 5 chars, requerido).
  - `assessment` (`string`, min 5 chars, requerido).
  - `plan` (`string`, min 5 chars, requerido).

### `POST /api/v1/patients/{id}/soep-notes/{noteId}/addenda`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `reason` (`string`, min 5 chars, requerido).
  - `correction_text` (`string`, min 10 chars, requerido).

### `POST /api/v1/patients/{id}/consents/{consentId}/sign`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`dentist` | `assistant` | `receptionist`), `X-MFA-Verified: true`, `X-Source-IP`.
- Body JSON:
  - `signature_method` (`string`: `"BIOMETRIC"` | `"OTP"`, requerido).
  - `biometric_svg_strokes` (`string`, obligatorio si `signature_method == "BIOMETRIC"`).
  - `device_metadata` (`object`, opcional: `{ "screen_inches": float64, "touch_points": int }`).
  - `otp_code` (`string`, `^[0-9]{6}$`, obligatorio si `signature_method == "OTP"`).
  - `signed_by_name` (`string`, nombre del paciente o tutor legal si es menor, requerido).
  - `signed_by_document` (`string`, requerido).

---

## 5. Contrato de salida

### Respuesta `POST /api/v1/patients/{id}/soep-notes` (`201 Created`)
```json
{
  "id": "e0123456-789a-4bcd-8ef0-123456789abc",
  "patient_id": "2c5ea4c0-4067-11e9-8bad-9b1deb4d3b7d",
  "author_dentist_id": "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
  "subjective": "Paciente refiere dolor pulsátil en cuadrante superior derecho al frío.",
  "objective": "Caries profunda oclusal en pieza 16, percusión vertical positiva.",
  "assessment": "Pulpitis irreversible sintomática en pieza 16.",
  "plan": "Biopulpectomía en pieza 16 bajo anestesia infiltrativa.",
  "signature_hash": "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "created_at": "2026-10-30T16:00:00Z",
  "locks_at": "2026-10-31T16:00:00Z",
  "is_locked": false,
  "addenda": []
}
```

### Respuesta `POST /api/v1/patients/{id}/consents/{consentId}/sign` (`200 OK`)
```json
{
  "consent_id": "b1234567-89ab-4cde-8f01-23456789abcd",
  "status": "SIGNED",
  "signature_method": "BIOMETRIC",
  "document_sha256": "3a7bd3e2360a3d29eea436fcfb7e44c735d117c42d1c1835420b6b9942dd4f1b",
  "storage_object_key": "consents/2026/10/b1234567-89ab-4cde-8f01-23456789abcd.pdf",
  "signed_at": "2026-10-30T16:05:12Z"
}
```

### Matriz de Estados HTTP
- `200 OK` / `201 Created`: Nota SOEP, adenda o firma de consentimiento procesada.
- `403 Forbidden`: Rol distinto de `dentist` intentando firmar notas SOEP o `MFA_REQUIRED`.
- `404 Not Found`: Nota o consentimiento inexistente.
- `409 Conflict`: `SOEP_NOTE_LOCKED` (pasadas las 24h), `CONSENT_ALREADY_SIGNED` o intento de eliminar registro (`RN-01`).
- `422 Unprocessable Entity`: Campos SOEP vacíos, código OTP inválido/expirado o trazos biométricos vacíos.

---

## 6. Seguridad y resiliencia

- **Inmutabilidad Estricta (`RN-01`, `RF-05`):** Trigger SQL en `soep_notes` y `soep_addenda` que impide `DELETE` siempre e impide `UPDATE` cuando `NOW() >= locks_at`.
- **Cifrado y No Repudio (`RNF-04`, `RF-06`):** El PDF del consentimiento firmado se almacena en S3/Blob con SSE-AES256 y su digest `document_sha256` queda registrado tanto en la tabla `patient_consents` como en `audit_logs` (`RNF-06`).
- **Reloj Inyectable (`ClockPort`):** El cálculo de la ventana de 24 horas utiliza una interfaz `Clock` inyectada para permitir pruebas determinísticas de bloqueo sin `time.Sleep`.

---

## 7. Archivos afectados

- `services/clinical-agenda/migrations/00004_soep_consents.sql`
- `services/clinical-agenda/internal/clinical/domain/soep_note.go`
- `services/clinical-agenda/internal/clinical/domain/consent.go`
- `services/clinical-agenda/internal/clinical/domain/soep_note_test.go`
- `services/clinical-agenda/internal/clinical/domain/consent_test.go`
- `services/clinical-agenda/internal/clinical/application/soep.port.go`
- `services/clinical-agenda/internal/clinical/application/soep.service.go`
- `services/clinical-agenda/internal/clinical/application/soep.service_test.go`
- `services/clinical-agenda/internal/clinical/application/consent.port.go`
- `services/clinical-agenda/internal/clinical/application/consent.service.go`
- `services/clinical-agenda/internal/clinical/application/consent.service_test.go`
- `services/clinical-agenda/internal/clinical/adapters/postgres/soep_repository.go`
- `services/clinical-agenda/internal/clinical/adapters/postgres/consent_repository.go`
- `services/clinical-agenda/internal/clinical/adapters/s3/consent_blob_store.go`
- `services/clinical-agenda/internal/clinical/adapters/http/soep_consent_handler.go`
- `services/clinical-agenda/internal/clinical/adapters/http/soep_consent_handler_test.go`
