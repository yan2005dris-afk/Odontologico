package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

func TestNewPeriodontogramChart_NICAndBOPCalculation(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 15, 0, 0, time.UTC)

	chart, err := domain.NewPeriodontogramChart("c-1", "p-1", "dent-1", []domain.PeriodontogramTooth{
		{
			ToothFDI:          "16",
			ProbingDepthMM:    [6]int{3, 2, 4, 3, 2, 3},
			GingivalMarginMM:  [6]int{0, -1, -1, 0, 0, -1},
			BleedingOnProbing: [6]bool{false, false, true, false, false, false},
			FurcationGrade:    1,
			MobilityGrade:     0,
		},
	}, now)
	if err != nil {
		t.Fatalf("expected valid periodontogram chart, got %v", err)
	}

	expectedNIC := [6]int{3, 3, 5, 3, 2, 4}
	if chart.Teeth[0].NICMM != expectedNIC {
		t.Fatalf("expected NIC %v, got %v", expectedNIC, chart.Teeth[0].NICMM)
	}
	if chart.BOPPercentage != 16.67 {
		t.Fatalf("expected BOP 16.67%%, got %.2f", chart.BOPPercentage)
	}
	if chart.MeanNICMM != 3.33 {
		t.Fatalf("expected MeanNICMM 3.33, got %.2f", chart.MeanNICMM)
	}

	// Validation errors (S4)
	if _, err := domain.NewPeriodontogramChart("c-2", "p-1", "dent-1", []domain.PeriodontogramTooth{
		{
			ToothFDI:       "16",
			ProbingDepthMM: [6]int{20, 2, 2, 2, 2, 2},
		},
	}, now); !errors.Is(err, domain.ErrInvalidProbingDepth) {
		t.Fatalf("expected ErrInvalidProbingDepth, got %v", err)
	}
	if _, err := domain.NewPeriodontogramChart("c-2", "p-1", "dent-1", []domain.PeriodontogramTooth{
		{
			ToothFDI:       "16",
			FurcationGrade: 5,
		},
	}, now); !errors.Is(err, domain.ErrInvalidFurcationGrade) {
		t.Fatalf("expected ErrInvalidFurcationGrade, got %v", err)
	}
}
