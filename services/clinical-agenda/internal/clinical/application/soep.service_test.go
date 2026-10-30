package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type adjustableClock struct {
	now time.Time
}

func (a *adjustableClock) Now() time.Time { return a.now }

func TestSOEPService_24hLockAndAddendum(t *testing.T) {
	clock := &adjustableClock{now: time.Date(2026, 10, 30, 16, 0, 0, 0, time.UTC)}
	repo := postgres.NewSOEPRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, clock, nil)
	svc := application.NewSOEPService(repo, auditSvc, clock, 24*time.Hour)
	_ = application.NewSOEPService(repo, nil, nil, 0)

	dentist := application.ActorMeta{
		ActorID:     "dent-1",
		ActorRole:   "dentist",
		MFAVerified: true,
		SourceIP:    "10.0.0.12",
	}

	if _, err := svc.CreateNote(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1", application.CreateSOEPNoteInput{}); !errors.Is(err, application.ErrClinicalMFARequired) {
		t.Fatalf("expected ErrClinicalMFARequired")
	}
	if _, err := svc.UpdateNote(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1", "n-1", application.UpdateSOEPNoteInput{}); !errors.Is(err, application.ErrClinicalMFARequired) {
		t.Fatalf("expected ErrClinicalMFARequired")
	}
	if _, err := svc.AddAddendum(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1", "n-1", application.CreateSOEPAddendumInput{}); !errors.Is(err, application.ErrClinicalMFARequired) {
		t.Fatalf("expected ErrClinicalMFARequired")
	}
	if _, err := svc.ListNotes(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1"); !errors.Is(err, application.ErrClinicalMFARequired) {
		t.Fatalf("expected ErrClinicalMFARequired")
	}

	// S1: Create SOEP note
	note, err := svc.CreateNote(context.Background(), dentist, "p-1", application.CreateSOEPNoteInput{
		Subjective: "Paciente refiere sensibilidad térmica aguda.",
		Objective:  "Caries distal profunda en pieza 24.",
		Assessment: "Pulpitis reversible en pieza 24.",
		Plan:       "Recubrimiento pulpar indirecto y resina compuesta.",
	})
	if err != nil || note.LocksAt.Sub(note.CreatedAt) != 24*time.Hour {
		t.Fatalf("expected valid note with 24h lock window, got %+v err=%v", note, err)
	}

	// Edit within 24h succeeds
	clock.now = clock.now.Add(2 * time.Hour)
	if _, err := svc.UpdateNote(context.Background(), dentist, "p-1", note.ID, application.UpdateSOEPNoteInput{
		Subjective: "Paciente refiere sensibilidad térmica al frío.",
		Objective:  "Caries distal profunda en pieza 24.",
		Assessment: "Pulpitis reversible en pieza 24.",
		Plan:       "Recubrimiento pulpar indirecto y obturación.",
	}); err != nil {
		t.Fatalf("expected edit within 24h to succeed, got %v", err)
	}

	// S2: Advance clock by 25h -> UpdateNote rejected with ErrSOEPNoteLocked
	clock.now = clock.now.Add(25 * time.Hour)
	if _, err := svc.UpdateNote(context.Background(), dentist, "p-1", note.ID, application.UpdateSOEPNoteInput{
		Subjective: "Intento de edición posterior a 24h",
		Objective:  "Caries distal profunda en pieza 24.",
		Assessment: "Pulpitis irreversible.",
		Plan:       "Endodoncia.",
	}); !errors.Is(err, domain.ErrSOEPNoteLocked) {
		t.Fatalf("expected ErrSOEPNoteLocked after 25h, got %v", err)
	}

	// S2: DeleteNote always rejected
	if !errors.Is(svc.DeleteNote(context.Background(), dentist, "p-1", note.ID), domain.ErrClinicalRecordImmutable) {
		t.Fatalf("expected ErrClinicalRecordImmutable on DeleteNote")
	}

	// S3: AddAddendum succeeds after lock
	add, err := svc.AddAddendum(context.Background(), dentist, "p-1", note.ID, application.CreateSOEPAddendumInput{
		Reason:         "Evolución a las 48h",
		CorrectionText: "El paciente acude sin dolor espontáneo; pruebas de vitalidad normales.",
	})
	if err != nil || add.ParentNoteID != note.ID {
		t.Fatalf("expected valid addendum, got %+v err=%v", add, err)
	}

	notes, err := svc.ListNotes(context.Background(), dentist, "p-1")
	if err != nil || len(notes) != 1 || !notes[0].IsLocked {
		t.Fatalf("expected 1 locked note on ListNotes, got %+v err=%v", notes, err)
	}
}
