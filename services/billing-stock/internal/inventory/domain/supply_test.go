package domain_test

import (
	"testing"

	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

func TestSupplyAndRecipe_DeductionAndLowStockAlerts_RF14(t *testing.T) {
	sup1, err := domain.NewSupply("sup-1", "clinic-1", "ANES-LIDO-2", "Lidocaína 2%", "cartridge", "LOT-A1", "2027-12-31", 49, 10, 1200)
	if err != nil {
		t.Fatalf("unexpected error creating supply 1: %v", err)
	}
	sup2, err := domain.NewSupply("sup-2", "clinic-1", "RESINA-A2-3M", "Resina 3M A2", "syringe", "LOT-R9", "2027-06-30", 4, 3, 2000)
	if err != nil {
		t.Fatalf("unexpected error creating supply 2: %v", err)
	}

	recipe, err := domain.NewProcedureRecipe("REST-RESINA-1S", []domain.RecipeItem{
		{SupplySKU: "ANES-LIDO-2", QuantityPerProcedure: 1},
		{SupplySKU: "RESINA-A2-3M", QuantityPerProcedure: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error creating recipe: %v", err)
	}
	if len(recipe.Items) != 2 {
		t.Fatalf("expected 2 items in recipe, got %d", len(recipe.Items))
	}

	// Deduct 1 unit from ANES-LIDO-2 (49 -> 48, min_stock 10 => low_stock_alert = false)
	cost1, alert1, err := sup1.Deduct(1)
	if err != nil {
		t.Fatalf("unexpected error deducting supply 1: %v", err)
	}
	if cost1 != 1200 || alert1 || sup1.CurrentStock != 48 {
		t.Fatalf("unexpected deduction result for sup1: cost=%d alert=%v stock=%d", cost1, alert1, sup1.CurrentStock)
	}

	// Deduct 1 unit from RESINA-A2-3M (4 -> 3, min_stock 3 => low_stock_alert = true)
	cost2, alert2, err := sup2.Deduct(1)
	if err != nil {
		t.Fatalf("unexpected error deducting supply 2: %v", err)
	}
	if cost2 != 2000 || !alert2 || sup2.CurrentStock != 3 {
		t.Fatalf("unexpected deduction result for sup2: cost=%d alert=%v stock=%d", cost2, alert2, sup2.CurrentStock)
	}

	// S4 Adversarial: Insufficient stock returns ErrInsufficientSupplyStock without mutating CurrentStock
	if _, _, err := sup2.Deduct(5); err != domain.ErrInsufficientSupplyStock {
		t.Fatalf("expected ErrInsufficientSupplyStock, got %v", err)
	}
	if sup2.CurrentStock != 3 {
		t.Fatalf("expected CurrentStock to remain 3 after failed deduction, got %d", sup2.CurrentStock)
	}

	// Adversarial validation
	if _, err := domain.NewSupply("", "clinic-1", "SKU", "Name", "unit", "", "", 1, 0, 100); err == nil {
		t.Fatalf("expected error on empty supply ID")
	}
	if _, err := domain.NewProcedureRecipe("PROC", []domain.RecipeItem{{SupplySKU: "SKU", QuantityPerProcedure: 0}}); err == nil {
		t.Fatalf("expected error on non-positive recipe quantity")
	}
}
