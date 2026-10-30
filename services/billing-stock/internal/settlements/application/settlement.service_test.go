package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sigo-dental/billing-stock/internal/settlements/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/settlements/application"
	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

func TestSettlementService_GenerateAndPreventDuplicate_S1_S2(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewInMemorySettlementRepository()
	svc := application.NewSettlementService(repo)

	adminActor := application.ActorContext{
		ActorID:     "admin-01",
		ActorRole:   "admin",
		MFAVerified: true,
		SourceIP:    "10.0.0.1",
	}

	// Configure wildcard NET_PERCENTAGE_AFTER_COSTS (50%) and specific FIXED_AMOUNT (25,000)
	_, err := svc.ConfigureCommissionRule(ctx, adminActor, application.ConfigureRuleCommand{
		DentistID:       "dentist-01",
		ProcedureCode:   "*",
		CalculationMode: domain.ModeNetPercentageAfterCosts,
		RateBPS:         5000,
	})
	if err != nil {
		t.Fatalf("unexpected error configuring wildcard rule: %v", err)
	}

	_, err = svc.ConfigureCommissionRule(ctx, adminActor, application.ConfigureRuleCommand{
		DentistID:        "dentist-01",
		ProcedureCode:    "EXO-SIMPLE",
		CalculationMode:  domain.ModeFixedAmount,
		FixedAmountCents: 25000,
	})
	if err != nil {
		t.Fatalf("unexpected error configuring specific rule: %v", err)
	}

	// Generate settlement for 2 procedures:
	// 1. CORONA-ZIRC: gross 400,000 - lab 80,000 - mat 20,000 = 300,000 * 50% = 150,000
	// 2. EXO-SIMPLE: gross 100,000, fixed commission = 25,000
	// Total gross = 500,000, lab = 80,000, mat = 20,000, net commission = 175,000
	settlement, err := svc.GenerateSettlement(ctx, adminActor, application.GenerateSettlementCommand{
		SettlementID: "settle-001",
		DentistID:    "dentist-01",
		PeriodStart:  "2026-10-01",
		PeriodEnd:    "2026-10-31",
		Procedures: []domain.CompletedProcedureInput{
			{
				CompletedProcedureID: "proc-101",
				ProcedureCode:        "CORONA-ZIRC",
				GrossFeeCents:        400000,
				LabCostCents:         80000,
				MaterialCostCents:    20000,
			},
			{
				CompletedProcedureID: "proc-102",
				ProcedureCode:        "EXO-SIMPLE",
				GrossFeeCents:        100000,
				LabCostCents:         0,
				MaterialCostCents:    0,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error generating settlement: %v", err)
	}
	if settlement.GrossFeesCents != 500000 || settlement.TotalLabDeductionsCents != 80000 || settlement.TotalMaterialDeductionsCents != 20000 || settlement.NetPayableCommissionCents != 175000 {
		t.Fatalf("unexpected settlement totals: %+v", settlement)
	}
	if settlement.Status != domain.SettlementStatusDraft {
		t.Fatalf("expected DRAFT status, got %s", settlement.Status)
	}

	// S2 Adversarial: Attempting to settle proc-101 again returns ErrProcedureAlreadySettled
	_, err = svc.GenerateSettlement(ctx, adminActor, application.GenerateSettlementCommand{
		SettlementID: "settle-002",
		DentistID:    "dentist-01",
		PeriodStart:  "2026-10-01",
		PeriodEnd:    "2026-10-31",
		Procedures: []domain.CompletedProcedureInput{
			{
				CompletedProcedureID: "proc-101",
				ProcedureCode:        "CORONA-ZIRC",
				GrossFeeCents:        400000,
				LabCostCents:         80000,
				MaterialCostCents:    20000,
			},
		},
	})
	if !errors.Is(err, domain.ErrProcedureAlreadySettled) {
		t.Fatalf("expected ErrProcedureAlreadySettled, got %v", err)
	}

	// Adversarial MFA and Role checks
	noMFA := application.ActorContext{ActorID: "admin-01", ActorRole: "admin", MFAVerified: false}
	if _, err := svc.ConfigureCommissionRule(ctx, noMFA, application.ConfigureRuleCommand{}); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	if _, err := svc.GenerateSettlement(ctx, noMFA, application.GenerateSettlementCommand{}); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	badRole := application.ActorContext{ActorID: "recep-01", ActorRole: "receptionist", MFAVerified: true}
	if _, err := svc.GenerateSettlement(ctx, badRole, application.GenerateSettlementCommand{}); !errors.Is(err, application.ErrForbiddenRole) {
		t.Fatalf("expected ErrForbiddenRole, got %v", err)
	}
	// Auto-generated settlement ID
	sAuto, err := svc.GenerateSettlement(ctx, adminActor, application.GenerateSettlementCommand{
		DentistID:   "dentist-01",
		PeriodStart: "2026-11-01",
		PeriodEnd:   "2026-11-30",
		Procedures: []domain.CompletedProcedureInput{
			{CompletedProcedureID: "proc-201", ProcedureCode: "EXO-SIMPLE", GrossFeeCents: 100000},
		},
	})
	if err != nil || sAuto.SettlementID == "" {
		t.Fatalf("expected auto-generated settlement ID, got err=%v", err)
	}
}

