package domain_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

func TestTreatmentPlan_MultiPhaseCalculationApprovalAndRN01(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 30, 0, 0, time.UTC)

	phases := []domain.TreatmentPhase{
		{
			PhaseNumber: 1,
			Name:        "Fase 1: Urgencia y Endodoncia",
			Items: []domain.TreatmentItem{
				{
					ID:                       "item-1",
					ProcedureCode:            "ENDO-01",
					ProcedureName:            "Endodoncia Birradicular",
					ToothFDI:                 "14",
					ProfessionalFeeCents:     40000,
					LabCostCents:             10000,
					DiscountCents:            5000, // 10% <= 20% limit
					EstimatedDurationMinutes: 60,
				},
			},
		},
		{
			PhaseNumber: 2,
			Name:        "Fase 2: Rehabilitación Protésica",
			Items: []domain.TreatmentItem{
				{
					ID:                       "item-2",
					ProcedureCode:            "COR-ZIR",
					ProcedureName:            "Corona Libre de Metal Zirconio",
					ToothFDI:                 "14",
					ProfessionalFeeCents:     55000,
					LabCostCents:             25000,
					DiscountCents:            0,
					EstimatedDurationMinutes: 45,
				},
			},
		},
	}

	// S1: Create multi-phase budget
	plan, err := domain.NewTreatmentPlan("plan-1", "p-1", "Rehabilitación Integral 14", "USD", "dent-1", "dentist", phases, now)
	if err != nil {
		t.Fatalf("expected valid plan, got %v", err)
	}
	if plan.TotalNetCents != 125000 || plan.Phases[0].SubtotalNetCents != 45000 || plan.Phases[1].SubtotalNetCents != 80000 {
		t.Fatalf("unexpected totals: net=%d ph1=%d ph2=%d", plan.TotalNetCents, plan.Phases[0].SubtotalNetCents, plan.Phases[1].SubtotalNetCents)
	}

	// S1 adversarial: Dentist exceeding 20% discount is rejected with ErrDiscountLimitExceeded
	badPhases := []domain.TreatmentPhase{
		{
			PhaseNumber: 1,
			Name:        "Fase 1",
			Items: []domain.TreatmentItem{
				{
					ID:                       "item-x",
					ProcedureCode:            "ENDO-01",
					ProcedureName:            "Endo",
					ProfessionalFeeCents:     10000,
					DiscountCents:            5000, // 50% > 20%
					EstimatedDurationMinutes: 30,
				},
			},
		},
	}
	if _, err := domain.NewTreatmentPlan("plan-x", "p-1", "Plan", "USD", "dent-1", "dentist", badPhases, now); !errors.Is(err, domain.ErrDiscountLimitExceeded) {
		t.Fatalf("expected ErrDiscountLimitExceeded, got %v", err)
	}

	// S2: Partial approval of item-1 only generates 1 schedulable task
	seq := 0
	tasks, err := plan.Approve("PARTIAL", []string{"item-1"}, "SIG-PATIENT-001", func() string {
		seq++
		return fmt.Sprintf("task-%d", seq)
	}, now)
	if err != nil {
		t.Fatalf("expected partial approval success, got %v", err)
	}
	if plan.Status != "APPROVED_PARTIAL" || plan.ApprovedTotalCents != 45000 || plan.PendingTotalCents != 80000 || len(tasks) != 1 {
		t.Fatalf("unexpected partial approval state: status=%s approved=%d pending=%d tasks=%d",
			plan.Status, plan.ApprovedTotalCents, plan.PendingTotalCents, len(tasks))
	}

	// S3: Subsequent TOTAL approval approves remaining item-2 without duplicating task for item-1
	tasksTotal, err := plan.Approve("TOTAL", nil, "SIG-PATIENT-002", func() string {
		seq++
		return fmt.Sprintf("task-%d", seq)
	}, now)
	if err != nil || plan.Status != "APPROVED_TOTAL" || len(tasksTotal) != 1 || tasksTotal[0].TreatmentItemID != "item-2" {
		t.Fatalf("unexpected total approval result: status=%s tasks=%+v err=%v", plan.Status, tasksTotal, err)
	}

	// S4 / RN-01: Complete item-1 as REALIZADO -> immutable against delete/revert
	if _, err := plan.CompleteItem("item-1", "dent-1", now); err != nil {
		t.Fatalf("expected CompleteItem success, got %v", err)
	}
	if !errors.Is(plan.DeleteOrRevertItem("item-1"), domain.ErrClinicalRecordImmutable) {
		t.Fatalf("expected ErrClinicalRecordImmutable on REALIZADO item")
	}
}
