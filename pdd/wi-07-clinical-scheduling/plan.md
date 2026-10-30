# Plan de Work Item — `wi-07-clinical-scheduling`

- **Feature Padre:** [`pdd/tasks/05-scheduling-notifications.md`](../tasks/05-scheduling-notifications.md)
- **Worktree / Rama Asignada:** `wt/clinical-agenda`
- **Directorio Objetivo:** `services/clinical-agenda/internal/scheduling/`, `services/clinical-agenda/migrations/00006_scheduling_notifications.sql`
- **Referencias SRS:** `RF-09`, `RF-10`, `RF-11`, `RN-03`, `RNF-02`, `RNF-03`, `RNF-06`, `SRS §7.1`

---

## 1. Descripción y alcance

### Dentro del alcance (In Scope)
- Implementación del contexto `scheduling` en `services/clinical-agenda/internal/scheduling/` bajo arquitectura Hexagonal Pattern B.
- Migración SQL `00006_scheduling_notifications.sql` creando `dental_chairs`, `appointments` (con constraint de exclusión GiST `btree_gist` para `RN-03`) y `notification_jobs`.
- Agendamiento Multirrecinto (`RF-09`): consulta de calendario por día (`day`) y semana (`week`) filtrable por consultorio/sillón dental (`chair_id`), profesional especialista (`dentist_id`) y estado.
- Validación de Sobreposición de Sillones (`RN-03`): bloqueo estricto que impide agendar o reprogramar dos citas cuyos intervalos `[start_at, end_at)` se solapen en el mismo sillón dental físico (`chair_id`), **incluso si los odontólogos son diferentes**, siempre que la cita existente no esté en estado `CANCELADO` o `NO_SHOW`.
- Control de Estados de Cita (`RF-10`): máquina de estados determinística para los 7 estados del SRS: `PROGRAMADO`, `CONFIRMADO`, `EN_SALA_DE_ESPERA`, `EN_ATENCION`, `ATENDIDO`, `CANCELADO`, `NO_SHOW`.
- Notificaciones Automáticas (`RF-11`, `SRS §7.1`): programación automática de mensajes transaccionales de confirmación y recordatorios 48h y 24h antes del turno a través de puertos de mensajería (`WhatsApp` Meta Cloud API, `SMS` Twilio, `Email`), con manejo de respuesta de confirmación del paciente.

### Fuera del alcance (Out of Scope)
- Interfaz gráfica del calendario en Next.js (cubierta en `wi-10-frontend-shell-patients-agenda`).

---

## 2. Dependencias previas

- **Work Items previos:** `wi-02-clinical-identity-audit`, `wi-03-clinical-patients`, `wi-06-clinical-treatments` (vínculo opcional con `schedulable_tasks` de `RF-08`).
- **Migraciones previas:** `00001` a `00005` en `services/clinical-agenda/migrations/`.

---

## 3. Specs numeradas

| Spec | Descripción | Criterio de aceptación verificable | Nivel de test |
| :--- | :--- | :--- | :--- |
| **S1** | Bloqueo de sobreposición en el mismo sillón dental (`RN-03`) | Intentar crear o reprogramar una cita en `chair_id = X` con intervalo `[10:00, 10:45)` cuando ya existe una cita activa en `chair_id = X` en `[10:30, 11:00)` (con el mismo o diferente `dentist_id`) es rechazado con `409 Conflict` (`CHAIR_OVERLAP_CONFLICT`). Si la cita previa está `CANCELADO` o `NO_SHOW`, el turno se permite. | Unit + Integration + Adversarial |
| **S2** | Calendario multirrecinto filtrable por día/semana, sillón y especialista (`RF-09`) | `GET /api/v1/scheduling/appointments?view=day\|week&date=YYYY-MM-DD&chair_id=&dentist_id=` retorna las citas ordenadas cronológicamente dentro de la ventana solicitada con metadatos del sillón, odontólogo y paciente. | Unit + Integration |
| **S3** | Máquina de 7 estados de cita (`RF-10`) | `POST /api/v1/scheduling/appointments/{id}/transition` permite transiciones válidas (`PROGRAMADO` $\rightarrow$ `CONFIRMADO` $\rightarrow$ `EN_SALA_DE_ESPERA` $\rightarrow$ `EN_ATENCION` $\rightarrow$ `ATENDIDO`, así como cancelación o `NO_SHOW` antes de atención) y rechaza transiciones inválidas (ej. `CANCELADO` $\rightarrow$ `ATENDIDO` o `ATENDIDO` $\rightarrow$ `CANCELADO`) con `422 Unprocessable Entity` (`INVALID_APPOINTMENT_TRANSITION`). | Unit + Adversarial |
| **S4** | Programación y despacho de recordatorios 48h y 24h (`RF-11`) | Al crear una cita, se encolan trabajos de notificación para `CONFIRMATION_IMMEDIATE`, `REMINDER_48H` (`start_at - 48h`) y `REMINDER_24H` (`start_at - 24h`) en los canales solicitados (`WHATSAPP`, `SMS`, `EMAIL`). Si la cita se cancela, los recordatorios pendientes pasan a `CANCELLED`. | Unit + Integration |
| **S5** | Webhook de confirmación/cancelación por el paciente (`RF-11`) | `POST /api/v1/scheduling/webhooks/patient-reply` con respuesta de confirmación transiciona una cita `PROGRAMADO` a `CONFIRMADO` de forma idempotente. | Unit + Integration |

---

## 4. Contrato de entrada

### `POST /api/v1/scheduling/appointments`
- Headers: `X-Actor-ID`, `X-Actor-Role` (`receptionist` | `dentist` | `assistant` | `admin`), `X-Source-IP`.
- Body JSON:
  - `patient_id` (`string`, UUID, requerido).
  - `dentist_id` (`string`, UUID, requerido).
  - `chair_id` (`string`, UUID del sillón físico, requerido).
  - `clinic_id` (`string`, UUID de la sede/recinto, requerido).
  - `schedulable_task_id` (`string`, UUID opcional proveniente de `RF-08`).
  - `start_at` (`string`, ISO-8601 con zona horaria, requerido).
  - `end_at` (`string`, ISO-8601 con zona horaria, `end_at > start_at`, requerido).
  - `reason` (`string`, requerido).
  - `notification_channels` (`[]string`, subconjunto de `["WHATSAPP", "SMS", "EMAIL"]`, default `["WHATSAPP", "EMAIL"]`).

### `POST /api/v1/scheduling/appointments/{id}/transition`
- Body JSON:
  - `target_status` (`string`: `"PROGRAMADO"` | `"CONFIRMADO"` | `"EN_SALA_DE_ESPERA"` | `"EN_ATENCION"` | `"ATENDIDO"` | `"CANCELADO"` | `"NO_SHOW"`, requerido).
  - `notes` (`string`, opcional).

---

## 5. Contrato de salida

### Respuesta `POST /api/v1/scheduling/appointments` (`201 Created`)
```json
{
  "id": "d4e5f6a7-b8c9-4d0e-9f1a-2b3c4d5e6f70",
  "patient_id": "2c5ea4c0-4067-11e9-8bad-9b1deb4d3b7d",
  "dentist_id": "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
  "chair_id": "sillon-01-uuid",
  "clinic_id": "sede-central-uuid",
  "start_at": "2026-11-05T10:00:00Z",
  "end_at": "2026-11-05T11:00:00Z",
  "status": "PROGRAMADO",
  "scheduled_reminders": [
    { "type": "REMINDER_48H", "scheduled_for": "2026-11-03T10:00:00Z", "channels": ["WHATSAPP", "EMAIL"] },
    { "type": "REMINDER_24H", "scheduled_for": "2026-11-04T10:00:00Z", "channels": ["WHATSAPP", "EMAIL"] }
  ]
}
```

### Matriz de Estados HTTP
- `200 OK` / `201 Created`: Consulta de agenda, creación de turno o cambio de estado exitoso.
- `400 Bad Request`: `end_at <= start_at` o formato de fecha inválido.
- `404 Not Found`: Turno o sillón inexistente.
- `409 Conflict`: `CHAIR_OVERLAP_CONFLICT` (`RN-03`) o `DENTIST_OVERLAP_CONFLICT`.
- `422 Unprocessable Entity`: `INVALID_APPOINTMENT_TRANSITION` (`RF-10`).

---

## 6. Seguridad y resiliencia

- **Garantía Concurrente de No Sobreposición de Sillón (`RN-03`, `RNF-03`):** Además de la verificación en la capa de aplicación, la tabla `appointments` declara `EXCLUDE USING gist (chair_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&) WHERE (status NOT IN ('CANCELADO', 'NO_SHOW'))`. Incluso si dos recepcionistas envían un `POST` en el mismo milisegundo para el mismo sillón y horario con distintos odontólogos, PostgreSQL serializa y rechaza el segundo con error de constraint mapeado a `409 Conflict` (`CHAIR_OVERLAP_CONFLICT`).
- **Idempotencia y Reintentos en Mensajería (`RF-11`):** El despachador de notificaciones utiliza claves de idempotencia `(appointment_id, reminder_type, channel)` y backoff exponencial ante caídas transitorias de WhatsApp/Twilio/SMTP.
- **Auditoría (`RNF-06`):** Cada cambio de estado de cita se registra con `actor_id`, `timestamp_iso8601` y `source_ip`.

---

## 7. Archivos afectados

- `services/clinical-agenda/migrations/00006_scheduling_notifications.sql`
- `services/clinical-agenda/internal/scheduling/domain/appointment.go`
- `services/clinical-agenda/internal/scheduling/domain/notification_job.go`
- `services/clinical-agenda/internal/scheduling/domain/appointment_test.go`
- `services/clinical-agenda/internal/scheduling/application/scheduling.port.go`
- `services/clinical-agenda/internal/scheduling/application/scheduling.service.go`
- `services/clinical-agenda/internal/scheduling/application/scheduling.service_test.go`
- `services/clinical-agenda/internal/scheduling/adapters/postgres/scheduling_repository.go`
- `services/clinical-agenda/internal/scheduling/adapters/messaging/notification_dispatcher.go`
- `services/clinical-agenda/internal/scheduling/adapters/http/scheduling_handler.go`
- `services/clinical-agenda/internal/scheduling/adapters/http/scheduling_handler_test.go`
