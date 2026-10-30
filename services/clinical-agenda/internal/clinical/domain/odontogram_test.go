package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

func TestResolveToothCode_All52TeethAndInvalidCodes(t *testing.T) {
	// Permanent 16 <-> Universal 3
	fdi, univ, dent, err := domain.ResolveToothCode("16", "FDI")
	if err != nil || fdi != "16" || univ != "3" || dent != "PERMANENT" {
		t.Fatalf("unexpected resolution for 16: fdi=%s univ=%s dent=%s err=%v", fdi, univ, dent, err)
	}
	fdi2, univ2, dent2, err := domain.ResolveToothCode("3", "UNIVERSAL")
	if err != nil || fdi2 != "16" || univ2 != "3" || dent2 != "PERMANENT" {
		t.Fatalf("unexpected resolution for Universal 3: fdi=%s univ=%s dent=%s err=%v", fdi2, univ2, dent2, err)
	}

	// Deciduous 55 <-> Universal A
	fdiD, univD, dentD, err := domain.ResolveToothCode("55", "FDI")
	if err != nil || fdiD != "55" || univD != "A" || dentD != "DECIDUOUS" {
		t.Fatalf("unexpected resolution for 55: fdi=%s univ=%s dent=%s err=%v", fdiD, univD, dentD, err)
	}

	// Invalid codes (S1: 19, 56, 99)
	for _, invalid := range []string{"19", "56", "99", "00"} {
		if _, _, _, err := domain.ResolveToothCode(invalid, "FDI"); !errors.Is(err, domain.ErrInvalidToothCode) {
			t.Fatalf("expected ErrInvalidToothCode for %s, got %v", invalid, err)
		}
	}
}

func TestOdontogramFinding_ColorsAndRN01Immutability(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 10, 0, 0, time.UTC)

	caries, err := domain.NewOdontogramFinding("f-1", "p-1", "16", "FDI", "OCLUSAL_INCISAL", "CARIES", "DIAGNOSED", "Caries profunda", "dent-1", now)
	if err != nil {
		t.Fatalf("expected valid finding, got %v", err)
	}
	if caries.ColorHex != "#DC2626" || caries.Immutable {
		t.Fatalf("expected red #DC2626 and immutable=false, got %+v", caries)
	}

	restoration, err := domain.NewOdontogramFinding("f-2", "p-1", "26", "FDI", "MESIAL", "EXISTING_RESTORATION", "REALIZADO", "Resina", "dent-1", now)
	if err != nil {
		t.Fatalf("expected valid restoration, got %v", err)
	}
	if restoration.ColorHex != "#2563EB" || !restoration.Immutable {
		t.Fatalf("expected blue #2563EB and immutable=true, got %+v", restoration)
	}

	// RN-01: REALIZADO cannot be mutated or deleted (S3)
	if !errors.Is(restoration.UpdateStatus("PLANNED", "revert"), domain.ErrClinicalRecordImmutable) {
		t.Fatalf("expected ErrClinicalRecordImmutable on UpdateStatus of REALIZADO")
	}
	if !errors.Is(restoration.CanDelete(), domain.ErrClinicalRecordImmutable) {
		t.Fatalf("expected ErrClinicalRecordImmutable on CanDelete of REALIZADO")
	}

	// Addendum is permitted on REALIZADO finding (S3)
	add, err := restoration.AddAddendum("add-1", "Corrección de superficie: también abarca oclusal", "dent-1", now)
	if err != nil || add.FindingID != "f-2" || len(restoration.Addenda) != 1 {
		t.Fatalf("expected valid addendum, got %+v err=%v", add, err)
	}
}
