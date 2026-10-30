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
	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
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

type ConfigureRuleCommand struct {
	ID               string `json:"id"`
	DentistID        string `json:"dentist_id"`
	ProcedureCode    string `json:"procedure_code"`
	CalculationMode  string `json:"calculation_mode"`
	RateBPS          int    `json:"rate_bps"`
	FixedAmountCents int64  `json:"fixed_amount_cents"`
}

type GenerateSettlementCommand struct {
	SettlementID string                           `json:"settlement_id"`
	DentistID    string                           `json:"dentist_id"`
	PeriodStart  string                           `json:"period_start"`
	PeriodEnd    string                           `json:"period_end"`
	Procedures   []domain.CompletedProcedureInput `json:"procedures"`
}

type SettlementService struct {
	repo  SettlementRepositoryPort
	audit auditapp.Recorder
}

func NewSettlementService(repo SettlementRepositoryPort, audit auditapp.Recorder) *SettlementService {
	return &SettlementService{repo: repo, audit: audit}
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
func (s *SettlementService) recordExportAudit(ctx context.Context, actor ActorContext, resourceType, resourceID string, meta map[string]string) {
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

func (s *SettlementService) ConfigureCommissionRule(ctx context.Context, actor ActorContext, cmd ConfigureRuleCommand) (*domain.CommissionRule, error) {
	if err := validateActor(actor, "admin", "auditor"); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		id = newUUID()
	}
	rule, err := domain.NewCommissionRule(id, cmd.DentistID, cmd.ProcedureCode, cmd.CalculationMode, cmd.RateBPS, cmd.FixedAmountCents)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpsertCommissionRule(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *SettlementService) GenerateSettlement(ctx context.Context, actor ActorContext, cmd GenerateSettlementCommand) (*domain.DentistSettlement, error) {
	if err := validateActor(actor, "admin", "auditor"); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(cmd.SettlementID)
	if id == "" {
		id = newUUID()
	}

	// Check if any procedure is already settled in previous settlements
	for _, p := range cmd.Procedures {
		settled, err := s.repo.IsProcedureSettled(ctx, p.CompletedProcedureID)
		if err != nil {
			return nil, err
		}
		if settled {
			return nil, domain.ErrProcedureAlreadySettled
		}
	}

	settlement, err := domain.BuildDentistSettlement(
		id,
		cmd.DentistID,
		cmd.PeriodStart,
		cmd.PeriodEnd,
		actor.ActorID,
		cmd.Procedures,
		func(procCode string) (*domain.CommissionRule, error) {
			return s.repo.GetCommissionRule(ctx, cmd.DentistID, procCode)
		},
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.SaveSettlement(ctx, settlement); err != nil {
		return nil, err
	}
	return settlement, nil
}

// ListSettlements returns the settlements matching the filter, newest first, together
// with the total amount of records before pagination.
func (s *SettlementService) ListSettlements(ctx context.Context, actor ActorContext, filter SettlementFilter) ([]domain.SettlementListItemView, int, error) {
	if err := requireListingActor(actor); err != nil {
		return nil, 0, err
	}
	filter.Limit, filter.Offset = NormalizePage(filter.Limit, filter.Offset)
	settlements, total, err := s.repo.ListSettlements(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.SettlementListItemView, 0, len(settlements))
	for idx := range settlements {
		items = append(items, settlements[idx].ToListItemView())
	}
	s.recordExportAudit(ctx, actor, "SETTLEMENT", "settlements", map[string]string{
		"total":      strconv.Itoa(total),
		"limit":      strconv.Itoa(filter.Limit),
		"offset":     strconv.Itoa(filter.Offset),
		"dentist_id": filter.DentistID,
	})
	return items, total, nil
}

// ListCommissionRules returns the commission rules matching the filter.
func (s *SettlementService) ListCommissionRules(ctx context.Context, actor ActorContext, filter CommissionRuleFilter) ([]domain.CommissionRule, int, error) {
	if err := requireListingActor(actor); err != nil {
		return nil, 0, err
	}
	filter.Limit, filter.Offset = NormalizePage(filter.Limit, filter.Offset)
	rules, total, err := s.repo.ListCommissionRules(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.CommissionRule, len(rules))
	copy(items, rules)
	s.recordExportAudit(ctx, actor, "COMMISSION_RULE", "commission-rules", map[string]string{
		"total":          strconv.Itoa(total),
		"limit":          strconv.Itoa(filter.Limit),
		"offset":         strconv.Itoa(filter.Offset),
		"dentist_id":     filter.DentistID,
		"procedure_code": filter.ProcedureCode,
	})
	return items, total, nil
}
