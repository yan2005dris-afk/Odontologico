package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

var (
	ErrMFARequired   = errors.New("MFA_REQUIRED")
	ErrForbiddenRole = errors.New("FORBIDDEN_ROLE")
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
	repo SettlementRepositoryPort
}

func NewSettlementService(repo SettlementRepositoryPort) *SettlementService {
	return &SettlementService{repo: repo}
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
