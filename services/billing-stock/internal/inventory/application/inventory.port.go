package application

import (
	"context"

	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

type ActorContext struct {
	ActorID     string
	ActorRole   string
	MFAVerified bool
	SourceIP    string
}

// SupplyFilter holds the query filters of the supply listing.
type SupplyFilter struct {
	ClinicID string
	Query    string
	// LowStockOnly keeps only the supplies at (or below) their minimum stock.
	LowStockOnly bool
	Limit        int
	Offset       int
}

// RecipeFilter holds the query filters of the recipe listing.
type RecipeFilter struct {
	ProcedureCode string
	Limit         int
	Offset        int
}

type InventoryRepositoryPort interface {
	UpsertSupply(ctx context.Context, supply *domain.Supply) error
	GetSupplyBySKU(ctx context.Context, clinicID, sku string) (*domain.Supply, error)
	SaveProcedureRecipe(ctx context.Context, recipe *domain.ProcedureRecipe) error
	GetProcedureRecipe(ctx context.Context, procedureCode string) (*domain.ProcedureRecipe, error)
	ExecuteAtomicProcedureDeduction(
		ctx context.Context,
		completedProcedureID, clinicID, dentistID, procedureCode, actorID string,
	) (*domain.ProcedureDeductionResult, error)
	ListSupplies(ctx context.Context, filter SupplyFilter) ([]domain.Supply, int, error)
	ListRecipes(ctx context.Context, filter RecipeFilter) ([]domain.ProcedureRecipe, int, error)
}
