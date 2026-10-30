package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/sigo-dental/billing-stock/internal/inventory/application"
	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

type InMemoryInventoryRepository struct {
	mu               sync.RWMutex
	supplies         map[string]*domain.Supply
	recipes          map[string]*domain.ProcedureRecipe
	deductionsByProc map[string]*domain.ProcedureDeductionResult
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

// ListSupplies returns the supplies matching the filter. The supply model has no
// creation timestamp, so the deterministic order falls back to the supply ID.
func (r *InMemoryInventoryRepository) ListSupplies(_ context.Context, filter application.SupplyFilter) ([]domain.Supply, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	matched := make([]domain.Supply, 0, len(r.supplies))
	for _, supply := range r.supplies {
		if filter.ClinicID != "" && supply.ClinicID != filter.ClinicID {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(supply.SKU), query) &&
			!strings.Contains(strings.ToLower(supply.Name), query) {
			continue
		}
		if filter.LowStockOnly && supply.CurrentStock > supply.MinStockQuantity {
			continue
		}
		matched = append(matched, *supply)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ID < matched[j].ID
	})

	total := len(matched)
	limit, offset := application.NormalizePage(filter.Limit, filter.Offset)
	return page(matched, limit, offset), total, nil
}

// ListRecipes returns the recipes matching the filter ordered by procedure code.
func (r *InMemoryInventoryRepository) ListRecipes(_ context.Context, filter application.RecipeFilter) ([]domain.ProcedureRecipe, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]domain.ProcedureRecipe, 0, len(r.recipes))
	for _, recipe := range r.recipes {
		if filter.ProcedureCode != "" && recipe.ProcedureCode != filter.ProcedureCode {
			continue
		}
		cp := *recipe
		cp.Items = append([]domain.RecipeItem(nil), recipe.Items...)
		matched = append(matched, cp)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ProcedureCode < matched[j].ProcedureCode
	})

	total := len(matched)
	limit, offset := application.NormalizePage(filter.Limit, filter.Offset)
	return page(matched, limit, offset), total, nil
}

// page returns the requested window of an already ordered slice.
func page[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
