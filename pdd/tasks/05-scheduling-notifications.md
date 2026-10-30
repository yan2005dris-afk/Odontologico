# Feature 05 — Module 4.4: Multi-Chair Scheduling, Appointment State Machine & Automated Notifications

- **Feature ID:** `05-scheduling-notifications`
- **SRS References:** `RF-09`, `RF-10`, `RF-11`, `RN-03`, `RNF-02`, `RNF-03`, `RNF-06`, `SRS §7.1`
- **Status:** `awaiting_plan_approval`
- **Work Items:**
  - [`wi-07-clinical-scheduling`](../wi-07-clinical-scheduling/plan.md) (`wt/clinical-agenda` — `services/clinical-agenda/internal/scheduling`)

---

## 1. Purpose & Scope Boundaries

### In Scope
- Multi-Venue / Multi-Chair Calendar (`RF-09`): management of dental clinics/branches (`recintos`), physical dental chairs (`sillones`), and specialist schedules; day and week calendar queries filterable by `chair_id`, `dentist_id`, `specialty`, and `status`.
- Non-Negotiable Dental Chair Overlap Prevention (`RN-03`): both application-level validation and PostgreSQL exclusion constraint (`EXCLUDE USING gist (chair_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&) WHERE (status NOT IN ('CANCELADO', 'NO_SHOW'))`) guaranteeing that two simultaneous appointments can NEVER be booked on the same physical dental chair (`sillón`), even when assigned to different dentists or created concurrently.
- Appointment State Machine (`RF-10`): strict transition graph across all 7 canonical states: `PROGRAMADO`, `CONFIRMADO`, `EN_SALA_DE_ESPERA`, `EN_ATENCION`, `ATENDIDO`, `CANCELADO`, `NO_SHOW`.
- Automated Transactional Notifications (`RF-11`, `SRS §7.1`): Redis-backed asynchronous worker and outbox dispatcher sending confirmation messages and automated 48h and 24h pre-appointment reminders via WhatsApp Business (Meta Cloud API adapter), SMS (Twilio adapter), and Email, with patient webhook/reply handling to transition `PROGRAMADO` $\rightarrow$ `CONFIRMADO` or `CANCELADO`.

### Out of Scope
- Frontend calendar grid drag-and-drop and status drawer UI (covered in Feature `07` / `wi-10-frontend-shell-patients-agenda`).

---

## 2. Numbered Specifications (`S1..S6`)

- **S1 — Dental Chairs & Multi-Filter Calendar Query (`RF-09`):** `GET /api/v1/scheduling/appointments?view=day|week&date=YYYY-MM-DD&chair_id=&dentist_id=&clinic_id=` returns appointments grouped by physical chair (`sillón`) and specialist with patient critical alert flags (`RF-02`) and linked schedulable task (`RF-08`).
- **S2 — Physical Dental Chair Overlap Prevention (`RN-03`):** `POST /api/v1/scheduling/appointments` and `PATCH /api/v1/scheduling/appointments/{id}/reschedule` reject any overlapping interval `[start_at, end_at)` on the same `chair_id` (regardless of `dentist_id`) with `409 Conflict` (`CHAIR_OVERLAP_CONFLICT`), backed by a PostgreSQL `btree_gist` exclusion constraint to prevent race conditions under concurrent requests (`RNF-03`). Also prevents double-booking the same `dentist_id` across two chairs simultaneously (`DENTIST_OVERLAP_CONFLICT`).
- **S3 — 7-State Appointment Lifecycle Machine (`RF-10`):** `POST /api/v1/scheduling/appointments/{id}/transition` enforces valid state transitions:
  - `PROGRAMADO` $\rightarrow$ `CONFIRMADO` | `EN_SALA_DE_ESPERA` | `CANCELADO` | `NO_SHOW`
  - `CONFIRMADO` $\rightarrow$ `EN_SALA_DE_ESPERA` | `CANCELADO` | `NO_SHOW`
  - `EN_SALA_DE_ESPERA` $\rightarrow$ `EN_ATENCION` | `CANCELADO` | `NO_SHOW`
  - `EN_ATENCION` $\rightarrow$ `ATENDIDO`
  - Terminal states (`ATENDIDO`, `CANCELADO`, `NO_SHOW`) reject further transitions with `422 Unprocessable Entity` (`INVALID_APPOINTMENT_TRANSITION`).
- **S4 — Automated 48h & 24h Multi-Channel Reminders (`RF-11`, `SRS §7.1`):** Scheduling an appointment enqueues notification jobs for immediate booking confirmation, $T - 48\text{h}$, and $T - 24\text{h}$ reminders across configured channels (`WHATSAPP`, `SMS`, `EMAIL`). Rescheduling or canceling automatically revokes pending reminder jobs.
- **S5 — Delivery Idempotency, Retry & Patient Confirmation Webhook (`RF-11`):** Notification worker uses an idempotency key `(appointment_id, reminder_window, channel)` and exponential backoff retries. Inbound webhook `POST /api/v1/scheduling/webhooks/confirmation` processes patient confirmation/cancellation responses and updates appointment status accordingly.
- **S6 — Audit Trail & RBAC (`RNF-05`, `RNF-06`):** `receptionist`, `dentist`, `assistant`, and `admin` can manage appointments within their role scope; every state change is recorded with actor and timestamp.

---

## 3. Worktree & Parallelization Strategy

| Work Item | Git Worktree Branch | Target Directory Boundary | Upstream Dependency |
| :--- | :--- | :--- | :--- |
| `wi-07-clinical-scheduling` | `wt/clinical-agenda` | `services/clinical-agenda/internal/scheduling/**`, `services/clinical-agenda/migrations/00006_scheduling_notifications.sql` | `wi-02-clinical-identity-audit`, `wi-03-clinical-patients`, `wi-06-clinical-treatments` |
