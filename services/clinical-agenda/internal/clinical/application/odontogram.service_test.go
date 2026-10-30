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

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time { return f.now }

func TestOdontogramService_LifecycleAndRN01(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 10, 0, 0, time.UTC)
	repo := postgres.NewOdontogramRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, fixedClock{now: now}, nil)
	svc := application.NewOdontogramService(repo, auditSvc, fixedClock{now: now})
	_ = application.NewOdontogramService(repo, nil, nil)

	if _, err := svc.RecordFinding(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1", application.RecordFindingInput{}); !errors.Is(err, application.ErrClinicalMFARequired) {
		t.Fatalf("expected ErrClinicalMFARequired")
	}
	if _, err := svc.RecordFinding(context.Background(), application.ActorMeta{MFAVerified: true, ActorRole: "receptionist"}, "p-1", application.RecordFindingInput{}); !errors.Is(err, application.ErrClinicalForbiddenRole) {
		t.Fatalf("expected ErrClinicalForbiddenRole")
	}

	dentist := application.ActorMeta{
		ActorID:     "dent-1",
		ActorRole:   "dentist",
		MFAVerified: true,
		SourceIP:    "10.0.0.1",
	}

	// Record PLANNED finding and delete it cleanly before it becomes REALIZADO
	tempFinding, err := svc.RecordFinding(context.Background(), dentist, "p-1", application.RecordFindingInput{
		ToothCode:      "24",
		NotationSystem: "FDI",
		Surface:        "DISTAL",
		FindingType:    "SEALANT",
		Status:         "PLANNED",
	})
	if err != nil {
		t.Fatalf("expected temp finding created, got %v", err)
	}
	if err := svc.DeleteFinding(context.Background(), dentist, "p-1", tempFinding.ID); err != nil {
		t.Fatalf("expected delete of PLANNED finding to succeed, got %v", err)
	}

	// S2: Record finding with DIAGNOSED status
	finding, err := svc.RecordFinding(context.Background(), dentist, "p-1", application.RecordFindingInput{
		ToothCode:      "16",
		NotationSystem: "FDI",
		Surface:        "OCLUSAL_INCISAL",
		FindingType:    "CARIES",
		Status:         "DIAGNOSED",
		Notes:          "Caries activa",
	})
	if err != nil || finding.ColorHex != "#DC2626" {
		t.Fatalf("expected finding created with #DC2626, got %+v err=%v", finding, err)
	}

	// Transition to REALIZADO
	updated, err := svc.UpdateFinding(context.Background(), dentist, "p-1", finding.ID, application.UpdateFindingInput{
		Status: "REALIZADO",
		Notes:  "Restauración completada",
	})
	if err != nil || !updated.Immutable {
		t.Fatalf("expected immutable REALIZADO finding, got %+v err=%v", updated, err)
	}

	// S3 / RN-01: Cannot delete or update REALIZADO finding
	if err := svc.DeleteFinding(context.Background(), dentist, "p-1", finding.ID); !errors.Is(err, domain.ErrClinicalRecordImmutable) {
		t.Fatalf("expected ErrClinicalRecordImmutable on DeleteFinding, got %v", err)
	}
	if _, err := svc.UpdateFinding(context.Background(), dentist, "p-1", finding.ID, application.UpdateFindingInput{Status: "PLANNED"}); !errors.Is(err, domain.ErrClinicalRecordImmutable) {
		t.Fatalf("expected ErrClinicalRecordImmutable on UpdateFinding, got %v", err)
	}

	// S3: Addendum works on REALIZADO finding
	if _, err := svc.AddFindingAddendum(context.Background(), dentist, "p-1", finding.ID, application.FindingAddendumInput{
		Note: "Se utilizó ionómero vítreo como base cavitaria",
	}); err != nil {
		t.Fatalf("expected addendum to succeed, got %v", err)
	}

	// S5: GetOdontogram snapshot
	snap, err := svc.GetOdontogram(context.Background(), dentist, "p-1")
	if err != nil || len(snap.Findings) != 1 {
		t.Fatalf("expected 1 finding in snapshot, got %+v err=%v", snap, err)
	}
}
