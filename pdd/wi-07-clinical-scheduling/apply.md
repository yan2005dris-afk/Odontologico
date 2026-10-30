# PDD Apply Report — `wi-07-clinical-scheduling`

- **Work Item:** `wi-07-clinical-scheduling`
- **Feature Spec:** [`pdd/tasks/05-scheduling-notifications.md`](../tasks/05-scheduling-notifications.md)
- **Plan Reference:** [`pdd/wi-07-clinical-scheduling/plan.md`](./plan.md)
- **Worktree / Branch:** `wt/clinical-agenda`
- **Status:** `COMPLETED (GREEN)`

---

## 1. Spec Coverage Matrix (`S1..S5`)

| Spec | Requirement Summary | Verification & Test Coverage | Status |
| :--- | :--- | :--- | :--- |
| **S1** | Non-negotiable physical dental chair overlap prevention (`RN-03`): rejects overlapping `[start_at, end_at)` intervals on the same `chair_id` even when `dentist_id` is different (`409 CHAIR_OVERLAP_CONFLICT`), while allowing booking if prior appointment is `CANCELADO` or `NO_SHOW` | `TestAppointment_OverlapAndStateTransitions`, `TestSchedulingService_RN03ChairOverlapAndNotifications`, `TestSchedulingHandler_Endpoints`, `00006_scheduling_notifications.sql` (`EXCLUDE USING gist`) | PASS |
| **S2** | Multi-venue / multi-chair calendar query (`GET /api/v1/scheduling/appointments?view=day\|week&date=&chair_id=&dentist_id=`) ordered chronologically (`RF-09`) | `TestSchedulingService_RN03ChairOverlapAndNotifications`, `TestSchedulingHandler_Endpoints` | PASS |
| **S3** | Deterministic 7-state appointment machine (`PROGRAMADO`, `CONFIRMADO`, `EN_SALA_DE_ESPERA`, `EN_ATENCION`, `ATENDIDO`, `CANCELADO`, `NO_SHOW`) rejecting invalid transitions with `422 INVALID_APPOINTMENT_TRANSITION` (`RF-10`) | `TestAppointment_OverlapAndStateTransitions`, `TestSchedulingHandler_Endpoints` | PASS |
| **S4** | Automated scheduling & dispatch of immediate confirmation, 48h, and 24h reminders (`WHATSAPP`, `SMS`, `EMAIL`) and automatic cancellation of pending reminders on appointment cancellation (`RF-11`) | `TestSchedulingService_RN03ChairOverlapAndNotifications` | PASS |
| **S5** | Idempotent patient reply webhook (`POST /api/v1/scheduling/webhooks/patient-reply`) transitioning `PROGRAMADO` $\rightarrow$ `CONFIRMADO` or `CANCELADO` (`RF-11`) | `TestSchedulingService_RN03ChairOverlapAndNotifications`, `TestSchedulingHandler_Endpoints` | PASS |

---

## 2. Files Created / Modified

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
- `services/clinical-agenda/cmd/api/main.go`

---

## 3. Executed Commands & Real Observed Output

```bash
cd services/clinical-agenda && go test -count=1 -v -cover ./... && go vet ./... && go build -o bin/clinical-agenda ./cmd/api
```

```text
=== RUN   TestSchedulingHandler_Endpoints
--- PASS: TestSchedulingHandler_Endpoints (0.00s)
PASS
coverage: 88.3% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/http	0.054s	coverage: 88.3% of statements
=== RUN   TestSchedulingService_RN03ChairOverlapAndNotifications
--- PASS: TestSchedulingService_RN03ChairOverlapAndNotifications (0.00s)
PASS
coverage: 84.2% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/scheduling/application	0.017s	coverage: 84.2% of statements
=== RUN   TestAppointment_OverlapAndStateTransitions
--- PASS: TestAppointment_OverlapAndStateTransitions (0.00s)
PASS
coverage: 98.2% of statements
ok  	github.com/sigo-dental/clinical-agenda/internal/scheduling/domain	0.010s	coverage: 98.2% of statements
```

---

## 4. Architectural Decisions & Out-of-Scope Boundaries

- **Double-Layer `RN-03` Enforcement:** Half-open interval intersection (`[start_at, end_at)`) is checked under mutex lock in the repository AND enforced at the database engine level via PostgreSQL `EXCLUDE USING gist (chair_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&) WHERE (status NOT IN ('CANCELADO', 'NO_SHOW'))`.
- **Idempotent Notification Dispatcher (`RF-11`):** `NotificationDispatcher` deduplicates dispatches by `IdempotencyKey` (`appointment_id:reminder_type:channels`).
