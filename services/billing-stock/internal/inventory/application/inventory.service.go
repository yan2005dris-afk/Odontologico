package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	auditapp "github.com/sigo-dental/billing-stock/internal/audit/application"
	auditdom "github.com/sigo-dental/billing-stock/internal/audit/domain"
	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

var (
	ErrMFARequired   = errors.New("MFA_REQUIRED")
	ErrForbiddenRole = errors.New("FORBIDDEN_ROLE")
)

const (
	// DefaultListLimit is the page size used when the client does not request one.
	DefaultListLimit = 50
	// MaxListLimit is the hard ceiling of the page size (plan S3).
	MaxListLimit = 200
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
	repo  InventoryRepositoryPort
	audit auditapp.Recorder
}

func NewInventoryService(repo InventoryRepositoryPort, audit auditapp.Recorder) *InventoryService {
	return &InventoryService{repo: repo, audit: audit}
}

// NormalizePage applies the canonical pagination rules: limit defaults to 50, is capped
// at 200 and offset never goes below zero.
func NormalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = DefaultListLimit
	}
	if limit > MaxListLimit {
		limit = MaxListLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// requireListingActor checks the minimum identity a listing needs to leave an RNF-06
// trail. Role authorization is resolved at the gateway (plan S4).
func requireListingActor(actor ActorContext) error {
	if strings.TrimSpace(actor.ActorID) == "" || strings.TrimSpace(actor.ActorRole) == "" {
		return ErrForbiddenRole
	}
	return nil
}

// recordExportAudit appends the immutable RNF-06 trail of a bulk read. A failing
// recorder never invalidates the listing: the error is only logged.
func (s *InventoryService) recordExportAudit(ctx context.Context, actor ActorContext, resourceType, resourceID string, meta map[string]string) {
	if s.audit == nil {
		return
	}
	if _, err := s.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      actor.ActorID,
		ActorRole:    actor.ActorRole,
		Action:       auditdom.ActionExport,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		SourceIP:     actor.SourceIP,
		Metadata:     meta,
	}); err != nil {
		log.Printf("billing-stock: audit trail write failed for %s export: %v", resourceType, err)
	}
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

// ListSupplies returns the supplies matching the filter together with the total amount
// of records before pagination.
func (s *InventoryService) ListSupplies(ctx context.Context, actor ActorContext, filter SupplyFilter) ([]domain.SupplyListItemView, int, error) {
	if err := requireListingActor(actor); err != nil {
		return nil, 0, err
	}
	filter.Limit, filter.Offset = NormalizePage(filter.Limit, filter.Offset)
	supplies, total, err := s.repo.ListSupplies(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.SupplyListItemView, 0, len(supplies))
	for idx := range supplies {
		items = append(items, supplies[idx].ToListItemView())
	}
	s.recordExportAudit(ctx, actor, "SUPPLY", "supplies", map[string]string{
		"total":     strconv.Itoa(total),
		"limit":     strconv.Itoa(filter.Limit),
		"offset":    strconv.Itoa(filter.Offset),
		"clinic_id": filter.ClinicID,
		"q":         filter.Query,
		"low_stock": strconv.FormatBool(filter.LowStockOnly),
	})
	return items, total, nil
}

// ListRecipes returns the standard recipes matching the filter together with the total
// amount of records before pagination.
func (s *InventoryService) ListRecipes(ctx context.Context, actor ActorContext, filter RecipeFilter) ([]domain.RecipeListItemView, int, error) {
	if err := requireListingActor(actor); err != nil {
		return nil, 0, err
	}
	filter.Limit, filter.Offset = NormalizePage(filter.Limit, filter.Offset)
	recipes, total, err := s.repo.ListRecipes(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.RecipeListItemView, 0, len(recipes))
	for idx := range recipes {
		items = append(items, recipes[idx].ToListItemView())
	}
	s.recordExportAudit(ctx, actor, "PROCEDURE_RECIPE", "procedure-recipes", map[string]string{
		"total":          strconv.Itoa(total),
		"limit":          strconv.Itoa(filter.Limit),
		"offset":         strconv.Itoa(filter.Offset),
		"procedure_code": filter.ProcedureCode,
	})
	return items, total, nil
}
