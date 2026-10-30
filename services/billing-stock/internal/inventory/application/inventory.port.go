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

type InventoryRepositoryPort interface {
	UpsertSupply(ctx context.Context, supply *domain.Supply) error
	GetSupplyBySKU(ctx context.Context, clinicID, sku string) (*domain.Supply, error)
	SaveProcedureRecipe(ctx context.Context, recipe *domain.ProcedureRecipe) error
	GetProcedureRecipe(ctx context.Context, procedureCode string) (*domain.ProcedureRecipe, error)
	ExecuteAtomicProcedureDeduction(
		ctx context.Context,
		completedProcedureID, clinicID, dentistID, procedureCode, actorID string,
	) (*domain.ProcedureDeductionResult, error)
}
