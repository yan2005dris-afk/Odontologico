package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sigo-dental/billing-stock/internal/inventory/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/inventory/application"
	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

func TestInventoryService_AtomicDeductionAndIdempotency_S3_S4(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewInMemoryInventoryRepository()
	svc := application.NewInventoryService(repo)

	adminActor := application.ActorContext{
		ActorID:     "admin-01",
		ActorRole:   "admin",
		MFAVerified: true,
		SourceIP:    "10.0.0.1",
	}
	dentistActor := application.ActorContext{
		ActorID:     "dentist-01",
		ActorRole:   "dentist",
		MFAVerified: true,
		SourceIP:    "10.0.0.12",
	}

	// Register 2 supplies in clinic-01
	_, err := svc.UpsertSupply(ctx, adminActor, application.UpsertSupplyCommand{
		ClinicID:         "clinic-01",
		SKU:              "ANES-LIDO-2",
		Name:             "Lidocaína 2%",
		Unit:             "cartridge",
		CurrentStock:     49,
		MinStockQuantity: 10,
		UnitCostCents:    1200,
	})
	if err != nil {
		t.Fatalf("unexpected error upserting supply 1: %v", err)
	}
	_, err = svc.UpsertSupply(ctx, adminActor, application.UpsertSupplyCommand{
		ClinicID:         "clinic-01",
		SKU:              "RESINA-A2-3M",
		Name:             "Resina 3M A2",
		Unit:             "syringe",
		CurrentStock:     4,
		MinStockQuantity: 3,
		UnitCostCents:    2000,
	})
	if err != nil {
		t.Fatalf("unexpected error upserting supply 2: %v", err)
	}

	// Register recipe for REST-RESINA-1S
	_, err = svc.SaveProcedureRecipe(ctx, dentistActor, application.SaveRecipeCommand{
		ProcedureCode: "REST-RESINA-1S",
		Items: []domain.RecipeItem{
			{SupplySKU: "ANES-LIDO-2", QuantityPerProcedure: 1},
			{SupplySKU: "RESINA-A2-3M", QuantityPerProcedure: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error saving recipe: %v", err)
	}

	// S3: Execute procedure deduction
	res, err := svc.DeductForCompletedProcedure(ctx, dentistActor, application.DeductProcedureCompletedCommand{
		CompletedProcedureID: "55667788-1122-4333-8445-667788990011",
		ClinicID:             "clinic-01",
		DentistID:            "dentist-01",
		ProcedureCode:        "REST-RESINA-1S",
	})
	if err != nil {
		t.Fatalf("unexpected error deducting procedure: %v", err)
	}
	if res.TotalMaterialCostCents != 3200 || len(res.DeductedItems) != 2 {
		t.Fatalf("unexpected deduction result: %+v", res)
	}
	if res.DeductedItems[0].RemainingStock != 48 || res.DeductedItems[0].LowStockAlert {
		t.Fatalf("unexpected item 0 state: %+v", res.DeductedItems[0])
	}
	if res.DeductedItems[1].RemainingStock != 3 || !res.DeductedItems[1].LowStockAlert {
		t.Fatalf("unexpected item 1 state (expected low stock alert): %+v", res.DeductedItems[1])
	}

	// S4: Idempotency — calling again with the same completed_procedure_id returns same result without deducting again
	resDup, err := svc.DeductForCompletedProcedure(ctx, dentistActor, application.DeductProcedureCompletedCommand{
		CompletedProcedureID: "55667788-1122-4333-8445-667788990011",
		ClinicID:             "clinic-01",
		DentistID:            "dentist-01",
		ProcedureCode:        "REST-RESINA-1S",
	})
	if err != nil {
		t.Fatalf("unexpected error on idempotent call: %v", err)
	}
	if resDup.DeductedItems[1].RemainingStock != 3 {
		t.Fatalf("idempotency failed! Remaining stock changed to %d", resDup.DeductedItems[1].RemainingStock)
	}

	// S4 Adversarial: Insufficient stock rolls back the entire transaction (ANES-LIDO-2 must remain at 48)
	_, err = svc.SaveProcedureRecipe(ctx, dentistActor, application.SaveRecipeCommand{
		ProcedureCode: "REHAB-COMPLETA",
		Items: []domain.RecipeItem{
			{SupplySKU: "ANES-LIDO-2", QuantityPerProcedure: 2},
			{SupplySKU: "RESINA-A2-3M", QuantityPerProcedure: 10}, // Only 3 remaining!
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.DeductForCompletedProcedure(ctx, dentistActor, application.DeductProcedureCompletedCommand{
		CompletedProcedureID: "proc-insufficient-01",
		ClinicID:             "clinic-01",
		DentistID:            "dentist-01",
		ProcedureCode:        "REHAB-COMPLETA",
	})
	if !errors.Is(err, domain.ErrInsufficientSupplyStock) {
		t.Fatalf("expected ErrInsufficientSupplyStock, got %v", err)
	}
	sup1After, _ := repo.GetSupplyBySKU(ctx, "clinic-01", "ANES-LIDO-2")
	if sup1After.CurrentStock != 48 {
		t.Fatalf("expected atomic rollback to keep ANES-LIDO-2 at 48, got %d", sup1After.CurrentStock)
	}

	// Adversarial MFA, Role, and validation checks
	noMFA := application.ActorContext{ActorID: "admin-01", ActorRole: "admin", MFAVerified: false}
	if _, err := svc.UpsertSupply(ctx, noMFA, application.UpsertSupplyCommand{}); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	if _, err := svc.SaveProcedureRecipe(ctx, noMFA, application.SaveRecipeCommand{}); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	if _, err := svc.DeductForCompletedProcedure(ctx, noMFA, application.DeductProcedureCompletedCommand{}); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	badRole := application.ActorContext{ActorID: "recep-01", ActorRole: "receptionist", MFAVerified: true}
	if _, err := svc.DeductForCompletedProcedure(ctx, badRole, application.DeductProcedureCompletedCommand{}); !errors.Is(err, application.ErrForbiddenRole) {
		t.Fatalf("expected ErrForbiddenRole, got %v", err)
	}
	if _, err := svc.DeductForCompletedProcedure(ctx, dentistActor, application.DeductProcedureCompletedCommand{}); !errors.Is(err, domain.ErrInvalidSupplyInput) {
		t.Fatalf("expected ErrInvalidSupplyInput, got %v", err)
	}
}

