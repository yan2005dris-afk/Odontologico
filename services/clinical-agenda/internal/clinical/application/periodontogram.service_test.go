package application_test

import (
	"context"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

func TestPeriodontogramService_RecordAndList(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 15, 0, 0, time.UTC)
	repo := postgres.NewPeriodontogramRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, fixedClock{now: now}, nil)
	svc := application.NewPeriodontogramService(repo, auditSvc, fixedClock{now: now})
	_ = application.NewPeriodontogramService(repo, nil, nil)

	if _, err := svc.RecordChart(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1", application.RecordPeriodontogramInput{}); err == nil {
		t.Fatalf("expected MFA error")
	}
	if _, err := svc.ListCharts(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1"); err == nil {
		t.Fatalf("expected MFA error")
	}

	dentist := application.ActorMeta{
		ActorID:     "dent-1",
		ActorRole:   "dentist",
		MFAVerified: true,
		SourceIP:    "10.0.0.1",
	}

	chart, err := svc.RecordChart(context.Background(), dentist, "p-1", application.RecordPeriodontogramInput{
		Teeth: []domain.PeriodontogramTooth{
			{
				ToothFDI:          "16",
				ProbingDepthMM:    [6]int{3, 2, 4, 3, 2, 3},
				GingivalMarginMM:  [6]int{0, -1, -1, 0, 0, -1},
				BleedingOnProbing: [6]bool{false, false, true, false, false, false},
				FurcationGrade:    1,
				MobilityGrade:     0,
			},
		},
	})
	if err != nil || chart.BOPPercentage != 16.67 {
		t.Fatalf("expected BOP=16.67, got %+v err=%v", chart, err)
	}

	charts, err := svc.ListCharts(context.Background(), dentist, "p-1")
	if err != nil || len(charts) != 1 {
		t.Fatalf("expected 1 chart, got %d err=%v", len(charts), err)
	}
}
