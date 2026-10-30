package domain_test

import (
	"testing"

	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

func TestCommissionRule_ThreeCalculationModes_RF13_S1(t *testing.T) {
	// Mode 1: FIXED_PERCENTAGE (40% = 4000 bps on 100,000 -> 40,000)
	ruleFixedPct, err := domain.NewCommissionRule("rule-1", "dentist-1", "*", domain.ModeFixedPercentage, 4000, 0)
	if err != nil {
		t.Fatalf("unexpected error creating FIXED_PERCENTAGE rule: %v", err)
	}
	if got := ruleFixedPct.CalculateCommission(100000, 20000, 10000); got != 40000 {
		t.Fatalf("expected 40000 for FIXED_PERCENTAGE, got %d", got)
	}

	// Mode 2: FIXED_AMOUNT (25,000 fixed per procedure)
	ruleFixedAmt, err := domain.NewCommissionRule("rule-2", "dentist-1", "EXO-SIMPLE", domain.ModeFixedAmount, 0, 25000)
	if err != nil {
		t.Fatalf("unexpected error creating FIXED_AMOUNT rule: %v", err)
	}
	if got := ruleFixedAmt.CalculateCommission(100000, 20000, 10000); got != 25000 {
		t.Fatalf("expected 25000 for FIXED_AMOUNT, got %d", got)
	}

	// Mode 3: NET_PERCENTAGE_AFTER_COSTS (gross 100,000 - lab 20,000 - mat 10,000 = 70,000 * 50% = 35,000)
	ruleNetPct, err := domain.NewCommissionRule("rule-3", "dentist-1", "PROTESIS-ZIRC", domain.ModeNetPercentageAfterCosts, 5000, 0)
	if err != nil {
		t.Fatalf("unexpected error creating NET_PERCENTAGE_AFTER_COSTS rule: %v", err)
	}
	if got := ruleNetPct.CalculateCommission(100000, 20000, 10000); got != 35000 {
		t.Fatalf("expected 35000 for NET_PERCENTAGE_AFTER_COSTS, got %d", got)
	}

	// Adversarial S1: If lab + material costs exceed gross fee, commission is 0 (never negative)
	if got := ruleNetPct.CalculateCommission(50000, 40000, 20000); got != 0 {
		t.Fatalf("expected 0 when costs exceed gross fee, got %d", got)
	}

	// Adversarial validation cases
	if _, err := domain.NewCommissionRule("rule-err", "dentist-1", "*", domain.ModeFixedPercentage, 10001, 0); err == nil {
		t.Fatalf("expected error for rate_bps > 10000")
	}
	if _, err := domain.NewCommissionRule("rule-err", "dentist-1", "*", domain.ModeFixedPercentage, -1, 0); err == nil {
		t.Fatalf("expected error for negative rate_bps")
	}
	if _, err := domain.NewCommissionRule("rule-err", "dentist-1", "*", domain.ModeFixedAmount, 0, -500); err == nil {
		t.Fatalf("expected error for negative fixed_amount_cents")
	}
	if _, err := domain.NewCommissionRule("rule-err", "dentist-1", "*", "INVALID_MODE", 1000, 0); err == nil {
		t.Fatalf("expected error for invalid mode")
	}
	if _, err := domain.NewCommissionRule("", "dentist-1", "*", domain.ModeFixedPercentage, 1000, 0); err == nil {
		t.Fatalf("expected error for empty rule id")
	}

	// BuildDentistSettlement S2 domain tests
	settlement, err := domain.BuildDentistSettlement(
		"set-1",
		"dentist-1",
		"2026-10-01",
		"2026-10-31",
		"admin-1",
		[]domain.CompletedProcedureInput{
			{
				CompletedProcedureID: "proc-1",
				ProcedureCode:        "PROTESIS-ZIRC",
				GrossFeeCents:        100000,
				LabCostCents:         20000,
				MaterialCostCents:    10000,
			},
			{
				CompletedProcedureID: "proc-2",
				ProcedureCode:        "EXO-SIMPLE",
				GrossFeeCents:        50000,
				LabCostCents:         0,
				MaterialCostCents:    0,
			},
		},
		func(code string) (*domain.CommissionRule, error) {
			if code == "EXO-SIMPLE" {
				return ruleFixedAmt, nil
			}
			return ruleNetPct, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error building settlement: %v", err)
	}
	if settlement.NetPayableCommissionCents != 60000 || settlement.GrossFeesCents != 150000 {
		t.Fatalf("unexpected settlement totals: %+v", settlement)
	}

	// Duplicate completed_procedure_id within batch returns ErrProcedureAlreadySettled
	_, err = domain.BuildDentistSettlement(
		"set-dup",
		"dentist-1",
		"2026-10-01",
		"2026-10-31",
		"admin-1",
		[]domain.CompletedProcedureInput{
			{CompletedProcedureID: "proc-1", ProcedureCode: "EXO-SIMPLE", GrossFeeCents: 50000},
			{CompletedProcedureID: "proc-1", ProcedureCode: "EXO-SIMPLE", GrossFeeCents: 50000},
		},
		func(_ string) (*domain.CommissionRule, error) { return ruleFixedAmt, nil },
	)
	if err != domain.ErrProcedureAlreadySettled {
		t.Fatalf("expected ErrProcedureAlreadySettled, got %v", err)
	}

	// Invalid settlement input checks
	if _, err := domain.BuildDentistSettlement("", "dentist-1", "2026-10-01", "2026-10-31", "admin-1", nil, nil); err == nil {
		t.Fatalf("expected error on empty settlement input")
	}
	if _, err := domain.BuildDentistSettlement("set-x", "dentist-1", "2026-10-01", "2026-10-31", "admin-1", []domain.CompletedProcedureInput{
		{CompletedProcedureID: "", ProcedureCode: "EXO", GrossFeeCents: 100},
	}, func(_ string) (*domain.CommissionRule, error) { return ruleFixedAmt, nil }); err == nil {
		t.Fatalf("expected error on empty completed_procedure_id")
	}
}

