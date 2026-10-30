package domain

import (
	"errors"
	"strings"
)

const (
	MovementTypeProcedureConsumption = "PROCEDURE_CONSUMPTION"
	MovementTypeRestock              = "RESTOCK"
	MovementTypeAdjustment           = "ADJUSTMENT"
)

var (
	ErrInvalidSupplyInput      = errors.New("INVALID_SUPPLY_INPUT")
	ErrSupplyNotFound          = errors.New("SUPPLY_NOT_FOUND")
	ErrInsufficientSupplyStock = errors.New("INSUFFICIENT_SUPPLY_STOCK")
)

type Supply struct {
	ID               string `json:"id"`
	ClinicID         string `json:"clinic_id"`
	SKU              string `json:"sku"`
	Name             string `json:"name"`
	Unit             string `json:"unit"`
	BatchNumber      string `json:"batch_number,omitempty"`
	ExpiryDate       string `json:"expiry_date,omitempty"`
	CurrentStock     int    `json:"current_stock"`
	MinStockQuantity int    `json:"min_stock_quantity"`
	UnitCostCents    int64  `json:"unit_cost_cents"`
}

func NewSupply(id, clinicID, sku, name, unit, batchNumber, expiryDate string, currentStock, minStock int, unitCostCents int64) (*Supply, error) {
	if strings.TrimSpace(id) == "" ||
		strings.TrimSpace(clinicID) == "" ||
		strings.TrimSpace(sku) == "" ||
		strings.TrimSpace(name) == "" ||
		strings.TrimSpace(unit) == "" {
		return nil, ErrInvalidSupplyInput
	}
	if currentStock < 0 || minStock < 0 || unitCostCents < 0 {
		return nil, ErrInvalidSupplyInput
	}
	return &Supply{
		ID:               id,
		ClinicID:         clinicID,
		SKU:              sku,
		Name:             name,
		Unit:             unit,
		BatchNumber:      batchNumber,
		ExpiryDate:       expiryDate,
		CurrentStock:     currentStock,
		MinStockQuantity: minStock,
		UnitCostCents:    unitCostCents,
	}, nil
}

func (s *Supply) Deduct(quantity int) (lineMaterialCostCents int64, lowStockAlert bool, err error) {
	if quantity <= 0 {
		return 0, false, ErrInvalidSupplyInput
	}
	if s.CurrentStock < quantity {
		return 0, false, ErrInsufficientSupplyStock
	}
	s.CurrentStock -= quantity
	cost := int64(quantity) * s.UnitCostCents
	alert := s.CurrentStock <= s.MinStockQuantity
	return cost, alert, nil
}

// SupplyListItemView is the DTO of the supply listing.
type SupplyListItemView struct {
	SKU          string `json:"sku"`
	Name         string `json:"name"`
	CurrentStock int    `json:"current_stock"`
	MinStock     int    `json:"min_stock"`
	ClinicID     string `json:"clinic_id"`
	Unit         string `json:"unit"`
	LowStock     bool   `json:"low_stock"`
}

// ToListItemView maps the supply to the listing DTO. low_stock is derived: the supply
// is at (or below) its minimum stock.
func (s *Supply) ToListItemView() SupplyListItemView {
	return SupplyListItemView{
		SKU:          s.SKU,
		Name:         s.Name,
		CurrentStock: s.CurrentStock,
		MinStock:     s.MinStockQuantity,
		ClinicID:     s.ClinicID,
		Unit:         s.Unit,
		LowStock:     s.CurrentStock <= s.MinStockQuantity,
	}
}
