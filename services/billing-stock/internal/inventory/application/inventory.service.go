package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

var (
	ErrMFARequired   = errors.New("MFA_REQUIRED")
	ErrForbiddenRole = errors.New("FORBIDDEN_ROLE")
)

type UpsertSupplyCommand struct {
	ID               string `json:"id"`
	ClinicID         string `json:"clinic_id"`
	SKU              string `json:"sku"`
	Name             string `json:"name"`
	Unit             string `json:"unit"`
	BatchNumber      string `json:"batch_number"`
	ExpiryDate       string `json:"expiry_date"`
	CurrentStock     int    `json:"current_stock"`
	MinStockQuantity int    `json:"min_stock_quantity"`
	UnitCostCents    int64  `json:"unit_cost_cents"`
}

type SaveRecipeCommand struct {
	ProcedureCode string              `json:"procedure_code"`
	Items         []domain.RecipeItem `json:"items"`
}

type DeductProcedureCompletedCommand struct {
	CompletedProcedureID string `json:"completed_procedure_id"`
	ClinicID             string `json:"clinic_id"`
	DentistID            string `json:"dentist_id"`
	ProcedureCode        string `json:"procedure_code"`
}

type InventoryService struct {
	repo InventoryRepositoryPort
}

func NewInventoryService(repo InventoryRepositoryPort) *InventoryService {
	return &InventoryService{repo: repo}
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

func validateActor(actor ActorContext, allowedRoles ...string) error {
	if !actor.MFAVerified {
		return ErrMFARequired
	}
	if strings.TrimSpace(actor.ActorID) == "" {
		return ErrForbiddenRole
	}
	for _, r := range allowedRoles {
		if actor.ActorRole == r {
			return nil
		}
	}
	return ErrForbiddenRole
}

func (s *InventoryService) UpsertSupply(ctx context.Context, actor ActorContext, cmd UpsertSupplyCommand) (*domain.Supply, error) {
	if err := validateActor(actor, "admin", "assistant"); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		id = newUUID()
	}
	sup, err := domain.NewSupply(
		id,
		cmd.ClinicID,
		cmd.SKU,
		cmd.Name,
		cmd.Unit,
		cmd.BatchNumber,
		cmd.ExpiryDate,
		cmd.CurrentStock,
		cmd.MinStockQuantity,
		cmd.UnitCostCents,
	)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpsertSupply(ctx, sup); err != nil {
		return nil, err
	}
	return sup, nil
}

func (s *InventoryService) SaveProcedureRecipe(ctx context.Context, actor ActorContext, cmd SaveRecipeCommand) (*domain.ProcedureRecipe, error) {
	if err := validateActor(actor, "admin", "dentist"); err != nil {
		return nil, err
	}
	recipe, err := domain.NewProcedureRecipe(cmd.ProcedureCode, cmd.Items)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveProcedureRecipe(ctx, recipe); err != nil {
		return nil, err
	}
	return recipe, nil
}

func (s *InventoryService) DeductForCompletedProcedure(ctx context.Context, actor ActorContext, cmd DeductProcedureCompletedCommand) (*domain.ProcedureDeductionResult, error) {
	if err := validateActor(actor, "dentist", "assistant", "admin"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.CompletedProcedureID) == "" ||
		strings.TrimSpace(cmd.ClinicID) == "" ||
		strings.TrimSpace(cmd.DentistID) == "" ||
		strings.TrimSpace(cmd.ProcedureCode) == "" {
		return nil, domain.ErrInvalidSupplyInput
	}
	return s.repo.ExecuteAtomicProcedureDeduction(
		ctx,
		cmd.CompletedProcedureID,
		cmd.ClinicID,
		cmd.DentistID,
		cmd.ProcedureCode,
		actor.ActorID,
	)
}
