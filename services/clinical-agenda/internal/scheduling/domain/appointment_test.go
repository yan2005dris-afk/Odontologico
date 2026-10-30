package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
)

func TestAppointment_OverlapAndStateTransitions(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	start := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 5, 10, 45, 0, 0, time.UTC)

	if _, err := domain.NewAppointment("a", "", "d", "c", "cl", "", "r", start, end, now, nil); !errors.Is(err, domain.ErrInvalidAppointmentFields) {
		t.Fatalf("expected ErrInvalidAppointmentFields")
	}
	if _, err := domain.NewAppointment("a", "p", "d", "c", "cl", "", "r", end, start, now, nil); !errors.Is(err, domain.ErrInvalidAppointmentWindow) {
		t.Fatalf("expected ErrInvalidAppointmentWindow")
	}

	appt, err := domain.NewAppointment("appt-1", "p-1", "dent-1", "chair-1", "clinic-1", "", "Control general", start, end, time.Time{}, []string{"WHATSAPP", "EMAIL", "INVALID"})
	if err != nil {
		t.Fatalf("expected valid appointment, got %v", err)
	}
	if len(appt.ScheduledReminders) != 2 || appt.ScheduledReminders[0].Type != "REMINDER_48H" || appt.ScheduledReminders[1].Type != "REMINDER_24H" {
		t.Fatalf("expected 48h and 24h reminders, got %+v", appt.ScheduledReminders)
	}

	// Reschedule
	if err := appt.Reschedule("chair-2", start, end); err != nil || appt.ChairID != "chair-2" {
		t.Fatalf("expected Reschedule success, got %v", err)
	}
	if err := appt.Reschedule("chair-2", end, start); !errors.Is(err, domain.ErrInvalidAppointmentWindow) {
		t.Fatalf("expected ErrInvalidAppointmentWindow on invalid reschedule")
	}

	// S1: Overlap check [10:00, 10:45) vs [10:30, 11:00)
	overlapStart := time.Date(2026, 11, 5, 10, 30, 0, 0, time.UTC)
	overlapEnd := time.Date(2026, 11, 5, 11, 0, 0, 0, time.UTC)
	if !appt.OverlapsWith(overlapStart, overlapEnd) {
		t.Fatalf("expected [10:00, 10:45) and [10:30, 11:00) to overlap")
	}

	// Adjacent [10:45, 11:30) does NOT overlap half-open interval [10:00, 10:45)
	adjStart := time.Date(2026, 11, 5, 10, 45, 0, 0, time.UTC)
	adjEnd := time.Date(2026, 11, 5, 11, 30, 0, 0, time.UTC)
	if appt.OverlapsWith(adjStart, adjEnd) {
		t.Fatalf("expected adjacent interval [10:45, 11:30) not to overlap [10:00, 10:45)")
	}

	// S3: Valid transitions PROGRAMADO -> CONFIRMADO -> EN_SALA_DE_ESPERA -> EN_ATENCION -> ATENDIDO
	for _, next := range []string{
		domain.StatusConfirmado,
		domain.StatusEnSalaDeEspera,
		domain.StatusEnAtencion,
		domain.StatusAtendido,
	} {
		if err := appt.TransitionTo(next, "step note"); err != nil {
			t.Fatalf("expected transition to %s to succeed, got %v", next, err)
		}
	}

	// S3 adversarial: ATENDIDO -> CANCELADO is forbidden
	if err := appt.TransitionTo(domain.StatusCancelado, "late cancel"); !errors.Is(err, domain.ErrInvalidAppointmentTransition) {
		t.Fatalf("expected ErrInvalidAppointmentTransition from ATENDIDO to CANCELADO, got %v", err)
	}
	if err := appt.Reschedule("chair-1", start, end); !errors.Is(err, domain.ErrInvalidAppointmentTransition) {
		t.Fatalf("expected ErrInvalidAppointmentTransition when rescheduling ATENDIDO")
	}

	// S3 adversarial: CANCELADO -> ATENDIDO is forbidden
	appt2, _ := domain.NewAppointment("appt-2", "p-1", "dent-1", "chair-1", "clinic-1", "", "Urgencia", start, end, now, []string{"BAD"})
	_ = appt2.TransitionTo(domain.StatusCancelado, "Patient cancelled")
	if appt2.BlocksChair() {
		t.Fatalf("CANCELADO appointment must not block chair")
	}
	if err := appt2.TransitionTo(domain.StatusAtendido, ""); !errors.Is(err, domain.ErrInvalidAppointmentTransition) {
		t.Fatalf("expected ErrInvalidAppointmentTransition from CANCELADO to ATENDIDO, got %v", err)
	}
}
