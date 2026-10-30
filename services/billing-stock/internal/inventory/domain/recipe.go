package domain

import (
	"errors"
	"strings"
)

var (
	ErrInvalidRecipeInput = errors.New("INVALID_RECIPE_INPUT")
	ErrRecipeNotFound     = errors.New("RECIPE_NOT_FOUND")
)

type RecipeItem struct {
	SupplySKU            string `json:"supply_sku"`
	QuantityPerProcedure int    `json:"quantity_per_procedure"`
}

type ProcedureRecipe struct {
	ProcedureCode string       `json:"procedure_code"`
	Items         []RecipeItem `json:"items"`
}

type DeductedSupplyItem struct {
	SupplySKU        string `json:"supply_sku"`
	QuantityDeducted int    `json:"quantity_deducted"`
	RemainingStock   int    `json:"remaining_stock"`
	LowStockAlert    bool   `json:"low_stock_alert"`
}

type ProcedureDeductionResult struct {
	CompletedProcedureID   string               `json:"completed_procedure_id"`
	ProcedureCode          string               `json:"procedure_code"`
	TotalMaterialCostCents int64                `json:"total_material_cost_cents"`
	DeductedItems          []DeductedSupplyItem `json:"deducted_items"`
}

func NewProcedureRecipe(procedureCode string, items []RecipeItem) (*ProcedureRecipe, error) {
	if strings.TrimSpace(procedureCode) == "" || len(items) == 0 {
		return nil, ErrInvalidRecipeInput
	}
	cleaned := make([]RecipeItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, it := range items {
		sku := strings.TrimSpace(it.SupplySKU)
		if sku == "" || it.QuantityPerProcedure <= 0 {
			return nil, ErrInvalidRecipeInput
		}
		if _, dup := seen[sku]; dup {
			return nil, ErrInvalidRecipeInput
		}
		seen[sku] = struct{}{}
		cleaned = append(cleaned, RecipeItem{
			SupplySKU:            sku,
			QuantityPerProcedure: it.QuantityPerProcedure,
		})
	}
	return &ProcedureRecipe{
		ProcedureCode: procedureCode,
		Items:         cleaned,
	}, nil
}
