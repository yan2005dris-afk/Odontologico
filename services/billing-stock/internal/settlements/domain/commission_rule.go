package domain

import (
	"errors"
	"strings"
)

const (
	ModeFixedPercentage         = "FIXED_PERCENTAGE"
	ModeFixedAmount             = "FIXED_AMOUNT"
	ModeNetPercentageAfterCosts = "NET_PERCENTAGE_AFTER_COSTS"
)

var (
	ErrInvalidCommissionRule  = errors.New("INVALID_COMMISSION_RULE")
	ErrCommissionRuleNotFound = errors.New("COMMISSION_RULE_NOT_FOUND")
)

type CommissionRule struct {
	ID               string `json:"id"`
	DentistID        string `json:"dentist_id"`
	ProcedureCode    string `json:"procedure_code"`
	CalculationMode  string `json:"calculation_mode"`
	RateBPS          int    `json:"rate_bps"`
	FixedAmountCents int64  `json:"fixed_amount_cents"`
}

func NewCommissionRule(id, dentistID, procedureCode, mode string, rateBPS int, fixedAmountCents int64) (*CommissionRule, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(dentistID) == "" || strings.TrimSpace(procedureCode) == "" {
		return nil, ErrInvalidCommissionRule
	}
	switch mode {
	case ModeFixedPercentage, ModeNetPercentageAfterCosts:
		if rateBPS <= 0 || rateBPS > 10000 || fixedAmountCents < 0 {
			return nil, ErrInvalidCommissionRule
		}
	case ModeFixedAmount:
		if fixedAmountCents <= 0 || rateBPS < 0 || rateBPS > 10000 {
			return nil, ErrInvalidCommissionRule
		}
	default:
		return nil, ErrInvalidCommissionRule
	}

	return &CommissionRule{
		ID:               id,
		DentistID:        dentistID,
		ProcedureCode:    procedureCode,
		CalculationMode:  mode,
		RateBPS:          rateBPS,
		FixedAmountCents: fixedAmountCents,
	}, nil
}

func (r *CommissionRule) CalculateCommission(grossFeeCents, labCostCents, materialCostCents int64) int64 {
	if grossFeeCents < 0 {
		grossFeeCents = 0
	}
	if labCostCents < 0 {
		labCostCents = 0
	}
	if materialCostCents < 0 {
		materialCostCents = 0
	}

	switch r.CalculationMode {
	case ModeFixedPercentage:
		return (grossFeeCents * int64(r.RateBPS)) / 10000
	case ModeFixedAmount:
		return r.FixedAmountCents
	case ModeNetPercentageAfterCosts:
		netBase := grossFeeCents - labCostCents - materialCostCents
		if netBase <= 0 {
			return 0
		}
		return (netBase * int64(r.RateBPS)) / 10000
	default:
		return 0
	}
}
