package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

type InMemoryInventoryRepository struct {
	mu                sync.RWMutex
	supplies          map[string]*domain.Supply
	recipes           map[string]*domain.ProcedureRecipe
	deductionsByProc  map[string]*domain.ProcedureDeductionResult
}

func NewInMemoryInventoryRepository() *InMemoryInventoryRepository {
	return &InMemoryInventoryRepository{
		supplies:         make(map[string]*domain.Supply),
		recipes:          make(map[string]*domain.ProcedureRecipe),
		deductionsByProc: make(map[string]*domain.ProcedureDeductionResult),
	}
}

func supplyKey(clinicID, sku string) string {
	return fmt.Sprintf("%s::%s", clinicID, sku)
}

func (r *InMemoryInventoryRepository) UpsertSupply(_ context.Context, supply *domain.Supply) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *supply
	r.supplies[supplyKey(supply.ClinicID, supply.SKU)] = &cp
	return nil
}

func (r *InMemoryInventoryRepository) GetSupplyBySKU(_ context.Context, clinicID, sku string) (*domain.Supply, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sup, ok := r.supplies[supplyKey(clinicID, sku)]
	if !ok {
		return nil, domain.ErrSupplyNotFound
	}
	cp := *sup
	return &cp, nil
}

func (r *InMemoryInventoryRepository) SaveProcedureRecipe(_ context.Context, recipe *domain.ProcedureRecipe) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *recipe
	cp.Items = append([]domain.RecipeItem(nil), recipe.Items...)
	r.recipes[recipe.ProcedureCode] = &cp
	return nil
}

func (r *InMemoryInventoryRepository) GetProcedureRecipe(_ context.Context, procedureCode string) (*domain.ProcedureRecipe, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.recipes[procedureCode]
	if !ok {
		return nil, domain.ErrRecipeNotFound
	}
	cp := *rec
	cp.Items = append([]domain.RecipeItem(nil), rec.Items...)
	return &cp, nil
}

func (r *InMemoryInventoryRepository) ExecuteAtomicProcedureDeduction(
	_ context.Context,
	completedProcedureID, clinicID, _, procedureCode, _ string,
) (*domain.ProcedureDeductionResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// S4: Idempotency check — if completed_procedure_id was already processed, return previous deduction without double-deducting
	if existing, ok := r.deductionsByProc[completedProcedureID]; ok {
		cp := *existing
		cp.DeductedItems = append([]domain.DeductedSupplyItem(nil), existing.DeductedItems...)
		return &cp, nil
	}

	rec, ok := r.recipes[procedureCode]
	if !ok {
		return nil, domain.ErrRecipeNotFound
	}

	// Stage copies for full transactional rollback on any insufficient stock (S4)
	stagedSupplies := make([]*domain.Supply, 0, len(rec.Items))
	for _, item := range rec.Items {
		sup, exists := r.supplies[supplyKey(clinicID, item.SupplySKU)]
		if !exists {
			return nil, domain.ErrSupplyNotFound
		}
		cp := *sup
		stagedSupplies = append(stagedSupplies, &cp)
	}

	var totalCost int64
	deductedItems := make([]domain.DeductedSupplyItem, 0, len(rec.Items))
	for idx, item := range rec.Items {
		sup := stagedSupplies[idx]
		lineCost, alert, err := sup.Deduct(item.QuantityPerProcedure)
		if err != nil {
			// Atomic rollback: nothing in r.supplies was modified
			return nil, err
		}
		totalCost += lineCost
		deductedItems = append(deductedItems, domain.DeductedSupplyItem{
			SupplySKU:        sup.SKU,
			QuantityDeducted: item.QuantityPerProcedure,
			RemainingStock:   sup.CurrentStock,
			LowStockAlert:    alert,
		})
	}

	// Commit staged supplies
	for _, sup := range stagedSupplies {
		r.supplies[supplyKey(clinicID, sup.SKU)] = sup
	}

	result := &domain.ProcedureDeductionResult{
		CompletedProcedureID:   completedProcedureID,
		ProcedureCode:          procedureCode,
		TotalMaterialCostCents: totalCost,
		DeductedItems:          deductedItems,
	}
	r.deductionsByProc[completedProcedureID] = result

	cp := *result
	cp.DeductedItems = append([]domain.DeductedSupplyItem(nil), result.DeductedItems...)
	return &cp, nil
}
