package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

func TestSOEPNote_24hLockImmutabilityAndAddendum(t *testing.T) {
	created := time.Date(2026, 10, 30, 16, 0, 0, 0, time.UTC)

	note, err := domain.NewSOEPNote(
		"note-1",
		"p-1",
		"appt-1",
		"ENDO-16",
		"dent-1",
		"Paciente refiere dolor pulsátil en cuadrante superior derecho.",
		"Caries profunda oclusal en pieza 16, percusión positiva.",
		"Pulpitis irreversible sintomática en pieza 16.",
		"Biopulpectomía bajo anestesia infiltrativa.",
		24*time.Hour,
		created,
	)
	if err != nil {
		t.Fatalf("expected valid SOEP note, got %v", err)
	}
	if !strings.HasPrefix(note.SignatureHash, "sha256:") || note.IsLocked {
		t.Fatalf("unexpected initial SOEP note state: %+v", note)
	}

	// Edit within 24h by same author succeeds (S1/S2)
	if err := note.UpdateWithinWindow(
		"dent-1",
		"Paciente refiere dolor pulsátil intenso al frío.",
		"Caries profunda oclusal en pieza 16.",
		"Pulpitis irreversible sintomática.",
		"Biopulpectomía inmediata en pieza 16.",
		created.Add(2*time.Hour),
	); err != nil {
		t.Fatalf("expected edit at +2h to succeed, got %v", err)
	}

	// Edit by a different dentist fails
	if err := note.UpdateWithinWindow(
		"dent-2",
		"Intento de otro profesional",
		"Caries profunda oclusal",
		"Pulpitis irreversible",
		"Plan modificado",
		created.Add(3*time.Hour),
	); !errors.Is(err, domain.ErrSOEPDifferentAuthor) {
		t.Fatalf("expected ErrSOEPDifferentAuthor, got %v", err)
	}

	// Edit at or after +24h fails with ErrSOEPNoteLocked (S2 / RN-01)
	if err := note.UpdateWithinWindow(
		"dent-1",
		"Intento tardío tras 24h",
		"Caries profunda oclusal",
		"Pulpitis irreversible",
		"Plan modificado",
		created.Add(24*time.Hour),
	); !errors.Is(err, domain.ErrSOEPNoteLocked) {
		t.Fatalf("expected ErrSOEPNoteLocked at +24h, got %v", err)
	}

	// Delete is never allowed (S2 / RN-01)
	if !errors.Is(note.Delete(), domain.ErrClinicalRecordImmutable) {
		t.Fatalf("expected ErrClinicalRecordImmutable on Delete")
	}

	// Signed addendum on locked note succeeds without mutating original text (S3 / RN-01)
	origPlan := note.Plan
	add, err := note.AppendAddendum("add-1", "dent-1", "Rectificación de longitud de trabajo", "Se confirma longitud de trabajo palatina en 21.5 mm con localizador apical.", created.Add(26*time.Hour))
	if err != nil {
		t.Fatalf("expected addendum to succeed, got %v", err)
	}
	if note.Plan != origPlan || len(note.Addenda) != 1 || !strings.HasPrefix(add.SignatureHash, "sha256:") {
		t.Fatalf("unexpected addendum state: %+v", note)
	}
}
