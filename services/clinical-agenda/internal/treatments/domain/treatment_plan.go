package domain

import (
	"errors"
	"strings"
	"time"
)

const MaxDentistDiscountPercent = 20 // Dentists can discount up to 20%; higher requires admin role

var (
	ErrInvalidTreatmentPlan      = errors.New("treatment plan requires patient_id, title, currency, and at least one phase with items")
	ErrInvalidMonetaryAmount     = errors.New("monetary amounts must be non-negative and discount cannot exceed gross fee + lab cost")
	ErrInvalidDurationMinutes    = errors.New("estimated_duration_minutes must be between 15 and 240")
	ErrDiscountLimitExceeded     = errors.New("discount exceeds clinician authorization limit (20%); admin authorization required")
	ErrInvalidApprovalRequest    = errors.New("approval mode must be TOTAL or PARTIAL with non-empty approved_item_ids and patient_acceptance_ref")
	ErrTreatmentPlanNotFound     = errors.New("treatment plan not found")
	ErrTreatmentItemNotFound     = errors.New("treatment item not found in plan")
	ErrClinicalRecordImmutable   = errors.New("RN-01: completed treatment items (REALIZADO) are strictly immutable")
)

type TreatmentItem struct {
	ID                       string     `json:"id"`
	PhaseNumber              int        `json:"phase_number"`
	ProcedureCode            string     `json:"procedure_code"`
	ProcedureName            string     `json:"procedure_name"`
	ToothFDI                 string     `json:"tooth_fdi,omitempty"`
	Surface                  string     `json:"surface,omitempty"`
	ProfessionalFeeCents     int64      `json:"professional_fee_cents"`
	LabCostCents             int64      `json:"lab_cost_cents"`
	DiscountCents            int64      `json:"discount_cents"`
	NetCents                 int64      `json:"net_cents"`
	EstimatedDurationMinutes int        `json:"estimated_duration_minutes"`
	Status                   string     `json:"status"` // PENDING_APPROVAL | APPROVED | REALIZADO
	CompletedBy              string     `json:"completed_by,omitempty"`
	CompletedAt              *time.Time `json:"completed_at,omitempty"`
}

type TreatmentPhase struct {
	PhaseNumber      int             `json:"phase_number"`
	Name             string          `json:"name"`
	SubtotalNetCents int64           `json:"subtotal_net_cents"`
	Items            []TreatmentItem `json:"items"`
}

type TreatmentPlan struct {
	ID                   string           `json:"id"`
	PatientID            string           `json:"patient_id"`
	Title                string           `json:"title"`
	Currency             string           `json:"currency"`
	Status               string           `json:"status"` // DRAFT | APPROVED_PARTIAL | APPROVED_TOTAL | COMPLETED
	Phases               []TreatmentPhase `json:"phases"`
	TotalGrossCents      int64            `json:"total_gross_cents"`
	TotalDiscountCents   int64            `json:"total_discount_cents"`
	TotalNetCents        int64            `json:"total_net_cents"`
	ApprovedTotalCents   int64            `json:"approved_total_cents"`
	PendingTotalCents    int64            `json:"pending_total_cents"`
	PatientAcceptanceRef string           `json:"patient_acceptance_ref,omitempty"`
	CreatedBy            string           `json:"created_by"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

func NewTreatmentPlan(id, patientID, title, currency, actorID, actorRole string, phases []TreatmentPhase, now time.Time) (*TreatmentPlan, error) {
	patientID = strings.TrimSpace(patientID)
	title = strings.TrimSpace(title)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if patientID == "" || title == "" || len(currency) != 3 || len(phases) == 0 {
		return nil, ErrInvalidTreatmentPlan
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	isAdmin := strings.EqualFold(strings.TrimSpace(actorRole), "admin")

	var totalGross, totalDiscount, totalNet int64
	builtPhases := make([]TreatmentPhase, 0, len(phases))

	for _, ph := range phases {
		if ph.PhaseNumber < 1 || strings.TrimSpace(ph.Name) == "" || len(ph.Items) == 0 {
			return nil, ErrInvalidTreatmentPlan
		}
		var phaseNet int64
		builtItems := make([]TreatmentItem, 0, len(ph.Items))

		for _, it := range ph.Items {
			if strings.TrimSpace(it.ProcedureCode) == "" || strings.TrimSpace(it.ProcedureName) == "" {
				return nil, ErrInvalidTreatmentPlan
			}
			if it.ProfessionalFeeCents < 0 || it.LabCostCents < 0 || it.DiscountCents < 0 {
				return nil, ErrInvalidMonetaryAmount
			}
			gross := it.ProfessionalFeeCents + it.LabCostCents
			if it.DiscountCents > gross {
				return nil, ErrInvalidMonetaryAmount
			}
			if !isAdmin && gross > 0 && it.DiscountCents*100 > gross*MaxDentistDiscountPercent {
				return nil, ErrDiscountLimitExceeded
			}
			if it.EstimatedDurationMinutes < 15 || it.EstimatedDurationMinutes > 240 {
				return nil, ErrInvalidDurationMinutes
			}

			net := gross - it.DiscountCents
			totalGross += gross
			totalDiscount += it.DiscountCents
			totalNet += net
			phaseNet += net

			builtItems = append(builtItems, TreatmentItem{
				ID:                       it.ID,
				PhaseNumber:              ph.PhaseNumber,
				ProcedureCode:            strings.TrimSpace(it.ProcedureCode),
				ProcedureName:            strings.TrimSpace(it.ProcedureName),
				ToothFDI:                 strings.TrimSpace(it.ToothFDI),
				Surface:                  strings.TrimSpace(it.Surface),
				ProfessionalFeeCents:     it.ProfessionalFeeCents,
				LabCostCents:             it.LabCostCents,
				DiscountCents:            it.DiscountCents,
				NetCents:                 net,
				EstimatedDurationMinutes: it.EstimatedDurationMinutes,
				Status:                   "PENDING_APPROVAL",
			})
		}

		builtPhases = append(builtPhases, TreatmentPhase{
			PhaseNumber:      ph.PhaseNumber,
			Name:             strings.TrimSpace(ph.Name),
			SubtotalNetCents: phaseNet,
			Items:            builtItems,
		})
	}

	return &TreatmentPlan{
		ID:                 id,
		PatientID:          patientID,
		Title:              title,
		Currency:           currency,
		Status:             "DRAFT",
		Phases:             builtPhases,
		TotalGrossCents:    totalGross,
		TotalDiscountCents: totalDiscount,
		TotalNetCents:      totalNet,
		ApprovedTotalCents: 0,
		PendingTotalCents:  totalNet,
		CreatedBy:          actorID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

func (p *TreatmentPlan) Approve(mode string, approvedItemIDs []string, acceptanceRef string, taskIDGen func() string, now time.Time) ([]SchedulableTask, error) {
	mode = strings.ToUpper(strings.TrimSpace(mode))
	acceptanceRef = strings.TrimSpace(acceptanceRef)
	if acceptanceRef == "" || (mode != "TOTAL" && mode != "PARTIAL") {
		return nil, ErrInvalidApprovalRequest
	}
	if mode == "PARTIAL" && len(approvedItemIDs) == 0 {
		return nil, ErrInvalidApprovalRequest
	}

	selectedSet := make(map[string]bool, len(approvedItemIDs))
	for _, id := range approvedItemIDs {
		selectedSet[strings.TrimSpace(id)] = true
	}

	var newlyGenerated []SchedulableTask
	matchedAny := false

	for pIdx := range p.Phases {
		for iIdx := range p.Phases[pIdx].Items {
			item := &p.Phases[pIdx].Items[iIdx]
			shouldApprove := mode == "TOTAL" || selectedSet[item.ID]
			if !shouldApprove {
				continue
			}
			matchedAny = true
			if item.Status == "PENDING_APPROVAL" {
				item.Status = "APPROVED"
				newlyGenerated = append(newlyGenerated, SchedulableTask{
					TaskID:                   taskIDGen(),
					PatientID:                p.PatientID,
					PlanID:                   p.ID,
					TreatmentItemID:          item.ID,
					PhaseNumber:              item.PhaseNumber,
					ProcedureCode:            item.ProcedureCode,
					ProcedureName:            item.ProcedureName,
					ToothFDI:                 item.ToothFDI,
					EstimatedDurationMinutes: item.EstimatedDurationMinutes,
					Status:                   "READY_TO_SCHEDULE",
				})
			}
		}
	}

	if !matchedAny {
		return nil, ErrTreatmentItemNotFound
	}

	p.recomputeTotals()
	p.PatientAcceptanceRef = acceptanceRef
	p.UpdatedAt = now.UTC()
	return newlyGenerated, nil
}

func (p *TreatmentPlan) CompleteItem(itemID, dentistID string, now time.Time) (*TreatmentItem, error) {
	now = now.UTC()
	for pIdx := range p.Phases {
		for iIdx := range p.Phases[pIdx].Items {
			item := &p.Phases[pIdx].Items[iIdx]
			if item.ID == itemID {
				if item.Status == "REALIZADO" {
					return nil, ErrClinicalRecordImmutable
				}
				item.Status = "REALIZADO"
				item.CompletedBy = dentistID
				item.CompletedAt = &now
				p.recomputeTotals()
				p.UpdatedAt = now
				cp := *item
				return &cp, nil
			}
		}
	}
	return nil, ErrTreatmentItemNotFound
}

func (p *TreatmentPlan) DeleteOrRevertItem(itemID string) error {
	for pIdx := range p.Phases {
		for iIdx := range p.Phases[pIdx].Items {
			item := &p.Phases[pIdx].Items[iIdx]
			if item.ID == itemID {
				if item.Status == "REALIZADO" {
					return ErrClinicalRecordImmutable
				}
				return nil
			}
		}
	}
	return ErrTreatmentItemNotFound
}

func (p *TreatmentPlan) recomputeTotals() {
	var approved, pending int64
	allApprovedOrDone := true
	allDone := true
	for _, ph := range p.Phases {
		for _, it := range ph.Items {
			if it.Status == "APPROVED" || it.Status == "REALIZADO" {
				approved += it.NetCents
			} else {
				pending += it.NetCents
				allApprovedOrDone = false
			}
			if it.Status != "REALIZADO" {
				allDone = false
			}
		}
	}
	p.ApprovedTotalCents = approved
	p.PendingTotalCents = pending
	if allDone {
		p.Status = "COMPLETED"
	} else if allApprovedOrDone {
		p.Status = "APPROVED_TOTAL"
	} else if approved > 0 {
		p.Status = "APPROVED_PARTIAL"
	}
}
