package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/messaging"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/application"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
)

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time { return f.now }

func TestSchedulingService_RN03ChairOverlapAndNotifications(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	repo := postgres.NewSchedulingRepository()
	dispatcher := messaging.NewNotificationDispatcher()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, fixedClock{now: now}, nil)
	svc := application.NewSchedulingService(repo, dispatcher, auditSvc, fixedClock{now: now})
	_ = application.NewSchedulingService(repo, nil, nil, nil)

	actor := application.ActorMeta{}

	if _, err := svc.CreateAppointment(context.Background(), actor, application.CreateAppointmentInput{StartAt: "bad"}); !errors.Is(err, domain.ErrInvalidAppointmentWindow) {
		t.Fatalf("expected ErrInvalidAppointmentWindow")
	}
	if _, err := svc.CreateAppointment(context.Background(), actor, application.CreateAppointmentInput{StartAt: "2026-11-05T10:00:00Z", EndAt: "bad"}); !errors.Is(err, domain.ErrInvalidAppointmentWindow) {
		t.Fatalf("expected ErrInvalidAppointmentWindow")
	}

	// 1. Book Chair X [10:00, 10:45) with Dentist A
	appt1, err := svc.CreateAppointment(context.Background(), actor, application.CreateAppointmentInput{
		PatientID:            "p-1",
		DentistID:            "dentist-A",
		ChairID:              "chair-X",
		ClinicID:             "clinic-1",
		StartAt:              "2026-11-05T10:00:00Z",
		EndAt:                "2026-11-05T10:45:00Z",
		Reason:               "Endodoncia",
		NotificationChannels: []string{"WHATSAPP", "EMAIL"},
	})
	if err != nil {
		t.Fatalf("expected appointment 1 created, got %v", err)
	}
	if dispatcher.CountDispatched() != 3 {
		t.Fatalf("expected 3 notification jobs dispatched (immediate, 48h, 24h), got %d", dispatcher.CountDispatched())
	}

	// Reschedule within valid window
	rescheduled, err := svc.RescheduleAppointment(context.Background(), actor, appt1.ID, application.RescheduleAppointmentInput{
		ChairID: "chair-X",
		StartAt: "2026-11-05T10:00:00Z",
		EndAt:   "2026-11-05T10:45:00Z",
	})
	if err != nil || rescheduled.ID != appt1.ID {
		t.Fatalf("expected reschedule to succeed, got %v", err)
	}

	// QueryCalendar day and week
	calItems, err := svc.QueryCalendar(context.Background(), application.CalendarFilter{
		Date:    time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC),
		ChairID: "chair-X",
	})
	if err != nil || len(calItems) != 1 {
		t.Fatalf("expected 1 appointment in calendar, got %d err=%v", len(calItems), err)
	}

	// 2. S1 / RN-03: Book Chair X [10:30, 11:00) with DIFFERENT Dentist B -> MUST fail with ErrChairOverlapConflict
	_, err = svc.CreateAppointment(context.Background(), actor, application.CreateAppointmentInput{
		PatientID: "p-2",
		DentistID: "dentist-B",
		ChairID:   "chair-X",
		ClinicID:  "clinic-1",
		StartAt:   "2026-11-05T10:30:00Z",
		EndAt:     "2026-11-05T11:00:00Z",
		Reason:    "Limpieza",
	})
	if !errors.Is(err, domain.ErrChairOverlapConflict) {
		t.Fatalf("expected ErrChairOverlapConflict on overlapping chair even with different dentist, got %v", err)
	}

	// 3. S5: Patient reply webhook confirms appointment idempotently
	confirmed, err := svc.HandlePatientReply(context.Background(), actor, application.PatientReplyWebhookInput{
		AppointmentID: appt1.ID,
		ReplyAction:   "CONFIRM",
		Channel:       "WHATSAPP",
	})
	if err != nil || confirmed.Status != domain.StatusConfirmado {
		t.Fatalf("expected CONFIRMADO after webhook, got %+v err=%v", confirmed, err)
	}
	// Idempotent second confirmation
	if _, err := svc.HandlePatientReply(context.Background(), actor, application.PatientReplyWebhookInput{
		AppointmentID: appt1.ID,
		ReplyAction:   "CONFIRM",
		Channel:       "WHATSAPP",
	}); err != nil {
		t.Fatalf("expected idempotent confirmation to succeed, got %v", err)
	}

	// 4. S1 & S4: Cancel appt1 via webhook -> pending reminders become CANCELLED and Chair X becomes available
	_, err = svc.HandlePatientReply(context.Background(), actor, application.PatientReplyWebhookInput{
		AppointmentID: appt1.ID,
		ReplyAction:   "CANCEL",
		Channel:       "WHATSAPP",
	})
	if err != nil {
		t.Fatalf("expected cancellation to succeed, got %v", err)
	}
	jobs, _ := repo.ListReminderJobs(context.Background(), appt1.ID)
	for _, j := range jobs {
		if j.Status != "CANCELLED" {
			t.Fatalf("expected reminder job %s to be CANCELLED after appointment cancellation, got %s", j.Type, j.Status)
		}
	}

	// Now Dentist B can book Chair X [10:30, 11:00)
	if _, err := svc.CreateAppointment(context.Background(), actor, application.CreateAppointmentInput{
		PatientID: "p-2",
		DentistID: "dentist-B",
		ChairID:   "chair-X",
		ClinicID:  "clinic-1",
		StartAt:   "2026-11-05T10:30:00Z",
		EndAt:     "2026-11-05T11:00:00Z",
		Reason:    "Limpieza",
	}); err != nil {
		t.Fatalf("expected booking on Chair X to succeed after previous appointment was CANCELADO, got %v", err)
	}
}
