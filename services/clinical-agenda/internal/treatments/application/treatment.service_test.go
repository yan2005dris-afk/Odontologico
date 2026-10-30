package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/application"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time { return f.now }

func TestTreatmentService_ApprovalAndSchedulableTasks(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 30, 0, 0, time.UTC)
	repo := postgres.NewTreatmentRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, fixedClock{now: now}, nil)
	svc := application.NewTreatmentService(repo, auditSvc, fixedClock{now: now})
	_ = application.NewTreatmentService(repo, nil, nil)

	for _, fn := range []func() error{
		func() error {
			_, err := svc.CreatePlan(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1", application.CreateTreatmentPlanInput{})
			return err
		},
		func() error {
			_, err := svc.ApprovePlan(context.Background(), application.ActorMeta{MFAVerified: false}, "plan-1", application.ApprovePlanInput{})
			return err
		},
		func() error {
			_, err := svc.CompleteItem(context.Background(), application.ActorMeta{MFAVerified: false}, "plan-1", "item-1")
			return err
		},
		func() error {
			return svc.DeleteItem(context.Background(), application.ActorMeta{MFAVerified: false}, "plan-1", "item-1")
		},
		func() error {
			_, err := svc.ListSchedulableTasks(context.Background(), application.ActorMeta{MFAVerified: false}, "p-1")
			return err
		},
	} {
		if !errors.Is(fn(), application.ErrTreatmentMFARequired) {
			t.Fatalf("expected ErrTreatmentMFARequired")
		}
	}
	if _, err := svc.CreatePlan(context.Background(), application.ActorMeta{MFAVerified: true, ActorRole: "receptionist"}, "p-1", application.CreateTreatmentPlanInput{}); !errors.Is(err, application.ErrTreatmentForbiddenRole) {
		t.Fatalf("expected ErrTreatmentForbiddenRole")
	}
	if _, err := svc.ApprovePlan(context.Background(), application.ActorMeta{MFAVerified: true, ActorRole: "auditor"}, "plan-1", application.ApprovePlanInput{}); !errors.Is(err, application.ErrTreatmentForbiddenRole) {
		t.Fatalf("expected ErrTreatmentForbiddenRole")
	}
	if _, err := svc.CompleteItem(context.Background(), application.ActorMeta{MFAVerified: true, ActorRole: "receptionist"}, "plan-1", "item-1"); !errors.Is(err, application.ErrTreatmentForbiddenRole) {
		t.Fatalf("expected ErrTreatmentForbiddenRole")
	}

	dentist := application.ActorMeta{
		ActorID:     "dent-1",
		ActorRole:   "dentist",
		MFAVerified: true,
		SourceIP:    "10.0.0.15",
	}

	plan, err := svc.CreatePlan(context.Background(), dentist, "p-1", application.CreateTreatmentPlanInput{
		Title:    "Rehabilitación 14",
		Currency: "USD",
		Phases: []application.CreateTreatmentPhaseInput{
			{
				PhaseNumber: 1,
				Name:        "Endodoncia",
				Items: []application.CreateTreatmentItemInput{
					{
						ID:                       "item-1",
						ProcedureCode:            "ENDO-01",
						ProcedureName:            "Endodoncia Birradicular",
						ToothFDI:                 "14",
						ProfessionalFeeCents:     40000,
						LabCostCents:             10000,
						DiscountCents:            5000,
						EstimatedDurationMinutes: 60,
					},
				},
			},
			{
				PhaseNumber: 2,
				Name:        "Prótesis",
				Items: []application.CreateTreatmentItemInput{
					{
						ProcedureCode:            "COR-ZIR",
						ProcedureName:            "Corona Zirconio",
						ToothFDI:                 "14",
						ProfessionalFeeCents:     60000,
						LabCostCents:             20000,
						DiscountCents:            0,
						EstimatedDurationMinutes: 45,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected plan created, got %v", err)
	}

	// S2: Partial approval of item-1 only
	approvedOut, err := svc.ApprovePlan(context.Background(), dentist, plan.ID, application.ApprovePlanInput{
		Mode:                 "PARTIAL",
		ApprovedItemIDs:      []string{"item-1"},
		PatientAcceptanceRef: "ACCEPT-001",
	})
	if err != nil || approvedOut.Status != "APPROVED_PARTIAL" || len(approvedOut.GeneratedSchedulableTasks) != 1 {
		t.Fatalf("expected partial approval with 1 task, got %+v err=%v", approvedOut, err)
	}

	// S5: ListSchedulableTasks returns only the approved item
	tasks, err := svc.ListSchedulableTasks(context.Background(), dentist, "p-1")
	if err != nil || len(tasks) != 1 || tasks[0].TreatmentItemID != "item-1" {
		t.Fatalf("expected 1 schedulable task for item-1, got %+v err=%v", tasks, err)
	}

	// S4: CompleteItem marks item-1 REALIZADO and blocks DeleteItem
	if _, err := svc.CompleteItem(context.Background(), dentist, plan.ID, "item-1"); err != nil {
		t.Fatalf("expected CompleteItem success, got %v", err)
	}
	if !errors.Is(svc.DeleteItem(context.Background(), dentist, plan.ID, "item-1"), domain.ErrClinicalRecordImmutable) {
		t.Fatalf("expected ErrClinicalRecordImmutable when deleting REALIZADO item")
	}
}
