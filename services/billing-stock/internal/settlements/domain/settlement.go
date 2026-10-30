package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	SettlementStatusDraft    = "DRAFT"
	SettlementStatusApproved = "APPROVED"
	SettlementStatusPaid     = "PAID"
)

var (
	ErrProcedureAlreadySettled = errors.New("PROCEDURE_ALREADY_SETTLED")
	ErrInvalidSettlementInput  = errors.New("INVALID_SETTLEMENT_INPUT")
)

type CompletedProcedureInput struct {
	CompletedProcedureID string `json:"completed_procedure_id"`
	ProcedureCode        string `json:"procedure_code"`
	GrossFeeCents        int64  `json:"gross_fee_cents"`
	LabCostCents         int64  `json:"lab_cost_cents"`
	MaterialCostCents    int64  `json:"material_cost_cents"`
}

type SettlementItem struct {
	CompletedProcedureID string `json:"completed_procedure_id"`
	ProcedureCode        string `json:"procedure_code"`
	GrossFeeCents        int64  `json:"gross_fee_cents"`
	LabCostCents         int64  `json:"lab_cost_cents"`
	MaterialCostCents    int64  `json:"material_cost_cents"`
	CommissionCents      int64  `json:"commission_cents"`
}

type DentistSettlement struct {
	SettlementID                 string           `json:"settlement_id"`
	DentistID                    string           `json:"dentist_id"`
	PeriodStart                  string           `json:"period_start"`
	PeriodEnd                    string           `json:"period_end"`
	GrossFeesCents               int64            `json:"gross_fees_cents"`
	TotalLabDeductionsCents      int64            `json:"total_lab_deductions_cents"`
	TotalMaterialDeductionsCents int64            `json:"total_material_deductions_cents"`
	NetPayableCommissionCents    int64            `json:"net_payable_commission_cents"`
	Status                       string           `json:"status"`
	Items                        []SettlementItem `json:"items,omitempty"`
	GeneratedBy                  string           `json:"generated_by"`
	CreatedAt                    time.Time        `json:"created_at"`
}

// SettlementListItemView is the DTO of the settlement listing.
type SettlementListItemView struct {
	DentistID       string `json:"dentist_id"`
	PeriodFrom      string `json:"period_from"`
	PeriodTo        string `json:"period_to"`
	ProcedureCount  int    `json:"procedure_count"`
	GrossCents      int64  `json:"gross_cents"`
	CommissionCents int64  `json:"commission_cents"`
	NetCents        int64  `json:"net_cents"`
}

// ToListItemView maps the settlement to the listing DTO. The net amount is what the
// professional nets after the lab and material deductions and the clinic commission.
func (s *DentistSettlement) ToListItemView() SettlementListItemView {
	commission := s.NetPayableCommissionCents
	net := s.GrossFeesCents - s.TotalLabDeductionsCents - s.TotalMaterialDeductionsCents - commission
	return SettlementListItemView{
		DentistID:       s.DentistID,
		PeriodFrom:      s.PeriodStart,
		PeriodTo:        s.PeriodEnd,
		ProcedureCount:  len(s.Items),
		GrossCents:      s.GrossFeesCents,
		CommissionCents: commission,
		NetCents:        net,
	}
}

func BuildDentistSettlement(
	id, dentistID, periodStart, periodEnd, generatedBy string,
	procedures []CompletedProcedureInput,
	ruleResolver func(procedureCode string) (*CommissionRule, error),
) (*DentistSettlement, error) {
	if strings.TrimSpace(id) == "" ||
		strings.TrimSpace(dentistID) == "" ||
		strings.TrimSpace(periodStart) == "" ||
		strings.TrimSpace(periodEnd) == "" ||
		strings.TrimSpace(generatedBy) == "" ||
		len(procedures) == 0 {
		return nil, ErrInvalidSettlementInput
	}

	seen := make(map[string]struct{}, len(procedures))
	items := make([]SettlementItem, 0, len(procedures))

	var grossTotal, labTotal, matTotal, netCommissionTotal int64

	for _, p := range procedures {
		if strings.TrimSpace(p.CompletedProcedureID) == "" || strings.TrimSpace(p.ProcedureCode) == "" || p.GrossFeeCents < 0 || p.LabCostCents < 0 || p.MaterialCostCents < 0 {
			return nil, ErrInvalidSettlementInput
		}
		if _, dup := seen[p.CompletedProcedureID]; dup {
			return nil, ErrProcedureAlreadySettled
		}
		seen[p.CompletedProcedureID] = struct{}{}

		rule, err := ruleResolver(p.ProcedureCode)
		if err != nil {
			return nil, err
		}
		comm := rule.CalculateCommission(p.GrossFeeCents, p.LabCostCents, p.MaterialCostCents)

		grossTotal += p.GrossFeeCents
		labTotal += p.LabCostCents
		matTotal += p.MaterialCostCents
		netCommissionTotal += comm

		items = append(items, SettlementItem{
			CompletedProcedureID: p.CompletedProcedureID,
			ProcedureCode:        p.ProcedureCode,
			GrossFeeCents:        p.GrossFeeCents,
			LabCostCents:         p.LabCostCents,
			MaterialCostCents:    p.MaterialCostCents,
			CommissionCents:      comm,
		})
	}

	return &DentistSettlement{
		SettlementID:                 id,
		DentistID:                    dentistID,
		PeriodStart:                  periodStart,
		PeriodEnd:                    periodEnd,
		GrossFeesCents:               grossTotal,
		TotalLabDeductionsCents:      labTotal,
		TotalMaterialDeductionsCents: matTotal,
		NetPayableCommissionCents:    netCommissionTotal,
		Status:                       SettlementStatusDraft,
		Items:                        items,
		GeneratedBy:                  generatedBy,
		CreatedAt:                    time.Now().UTC(),
	}, nil
}
