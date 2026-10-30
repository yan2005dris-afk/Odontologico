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
	"time"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

const (
	// DefaultListLimit is the page size used when the caller does not ask
	// for one.
	DefaultListLimit = 50
	// MaxListLimit is the hard ceiling of a listing page (RNF-07).
	MaxListLimit = 200
)

var (
	ErrTreatmentForbiddenRole     = errors.New("actor role is not authorized for this treatment plan operation")
	ErrTreatmentReadForbiddenRole = errors.New("actor role is not authorized to read treatment plans")
	ErrTreatmentMFARequired       = errors.New("MFA verification is required for treatment plan operations")
	ErrInvalidPlanStatus          = errors.New("status must be one of: DRAFT, PARTIALLY_APPROVED, APPROVED, COMPLETED")
)

type stdClock struct{}

func (stdClock) Now() time.Time { return time.Now().UTC() }

type TreatmentService struct {
	repo  TreatmentRepository
	audit auditapp.Recorder
	clock Clock
}

func NewTreatmentService(repo TreatmentRepository, audit auditapp.Recorder, clock Clock) *TreatmentService {
	if clock == nil {
		clock = stdClock{}
	}
	return &TreatmentService{
		repo:  repo,
		audit: audit,
		clock: clock,
	}
}

func (s *TreatmentService) CreatePlan(ctx context.Context, actor ActorMeta, patientID string, input CreateTreatmentPlanInput) (*domain.TreatmentPlan, error) {
	if !actor.MFAVerified {
		return nil, ErrTreatmentMFARequired
	}
	role := strings.ToLower(strings.TrimSpace(actor.ActorRole))
	if role != "dentist" && role != "admin" {
		return nil, ErrTreatmentForbiddenRole
	}

	phases := make([]domain.TreatmentPhase, 0, len(input.Phases))
	for _, ph := range input.Phases {
		items := make([]domain.TreatmentItem, 0, len(ph.Items))
		for _, it := range ph.Items {
			itemID := strings.TrimSpace(it.ID)
			if itemID == "" {
				itemID = genUUID()
			}
			items = append(items, domain.TreatmentItem{
				ID:                       itemID,
				PhaseNumber:              ph.PhaseNumber,
				ProcedureCode:            it.ProcedureCode,
				ProcedureName:            it.ProcedureName,
				ToothFDI:                 it.ToothFDI,
				Surface:                  it.Surface,
				ProfessionalFeeCents:     it.ProfessionalFeeCents,
				LabCostCents:             it.LabCostCents,
				DiscountCents:            it.DiscountCents,
				EstimatedDurationMinutes: it.EstimatedDurationMinutes,
			})
		}
		phases = append(phases, domain.TreatmentPhase{
			PhaseNumber: ph.PhaseNumber,
			Name:        ph.Name,
			Items:       items,
		})
	}

	plan, err := domain.NewTreatmentPlan(genUUID(), patientID, input.Title, input.Currency, actor.ActorID, role, phases, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.SavePlan(ctx, plan); err != nil {
		return nil, err
	}

	s.recordAudit(ctx, actor, auditdom.ActionCreate, "TREATMENT_PLAN", plan.ID, map[string]string{
		"patient_id":      plan.PatientID,
		"total_net_cents": fmt.Sprintf("%d", plan.TotalNetCents),
	})
	return plan, nil
}

func (s *TreatmentService) ApprovePlan(ctx context.Context, actor ActorMeta, planID string, input ApprovePlanInput) (*ApprovePlanOutput, error) {
	if !actor.MFAVerified {
		return nil, ErrTreatmentMFARequired
	}
	role := strings.ToLower(strings.TrimSpace(actor.ActorRole))
	if role != "dentist" && role != "receptionist" && role != "admin" {
		return nil, ErrTreatmentForbiddenRole
	}

	plan, err := s.repo.GetPlanByID(ctx, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}

	tasks, err := plan.Approve(input.Mode, input.ApprovedItemIDs, input.PatientAcceptanceRef, genUUID, s.clock.Now())
	if err != nil {
		return nil, err
	}

	if err := s.repo.SavePlanApprovalTx(ctx, plan, tasks); err != nil {
		return nil, err
	}

	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "TREATMENT_PLAN", plan.ID, map[string]string{
		"status":          plan.Status,
		"generated_tasks": fmt.Sprintf("%d", len(tasks)),
	})

	return &ApprovePlanOutput{
		PlanID:                    plan.ID,
		PatientID:                 plan.PatientID,
		Status:                    plan.Status,
		ApprovedTotalCents:        plan.ApprovedTotalCents,
		PendingTotalCents:         plan.PendingTotalCents,
		GeneratedSchedulableTasks: tasks,
	}, nil
}

func (s *TreatmentService) CompleteItem(ctx context.Context, actor ActorMeta, planID, itemID string) (*domain.TreatmentItem, error) {
	if !actor.MFAVerified {
		return nil, ErrTreatmentMFARequired
	}
	role := strings.ToLower(strings.TrimSpace(actor.ActorRole))
	if role != "dentist" && role != "admin" {
		return nil, ErrTreatmentForbiddenRole
	}

	plan, err := s.repo.GetPlanByID(ctx, strings.TrimSpace(planID))
	if err != nil {
		return nil, err
	}

	item, err := plan.CompleteItem(strings.TrimSpace(itemID), actor.ActorID, s.clock.Now())
	if err != nil {
		return nil, err
	}

	if err := s.repo.SavePlan(ctx, plan); err != nil {
		return nil, err
	}

	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "TREATMENT_ITEM", item.ID, map[string]string{
		"plan_id": plan.ID,
		"status":  item.Status,
	})
	return item, nil
}

func (s *TreatmentService) DeleteItem(ctx context.Context, actor ActorMeta, planID, itemID string) error {
	if !actor.MFAVerified {
		return ErrTreatmentMFARequired
	}
	plan, err := s.repo.GetPlanByID(ctx, strings.TrimSpace(planID))
	if err != nil {
		return err
	}
	return plan.DeleteOrRevertItem(strings.TrimSpace(itemID))
}

func (s *TreatmentService) ListSchedulableTasks(ctx context.Context, actor ActorMeta, patientID string) ([]domain.SchedulableTask, error) {
	if !actor.MFAVerified {
		return nil, ErrTreatmentMFARequired
	}
	tasks, err := s.repo.ListSchedulableTasksByPatient(ctx, strings.TrimSpace(patientID))
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, actor, auditdom.ActionRead, "SCHEDULABLE_TASKS", patientID, nil)
	return tasks, nil
}

// ListPlans returns the treatment plans matching the filter (S1). The listing
// leaves an immutable audit trail with action EXPORT (RNF-06).
func (s *TreatmentService) ListPlans(ctx context.Context, actor ActorMeta, filter PlanFilter) ([]TreatmentPlanSummaryDTO, int, error) {
	if err := requireTreatmentReadRole(actor); err != nil {
		return nil, 0, err
	}
	normalized, err := normalizePlanFilter(filter)
	if err != nil {
		return nil, 0, err
	}

	plans, total, err := s.repo.ListPlans(ctx, normalized)
	if err != nil {
		return nil, 0, err
	}

	items := make([]TreatmentPlanSummaryDTO, 0, len(plans))
	for _, plan := range plans {
		if plan == nil {
			continue
		}
		items = append(items, summarizePlan(plan))
	}

	s.recordAudit(ctx, actor, auditdom.ActionExport, "TREATMENT_PLAN", "treatment-plans", map[string]string{
		"total": strconv.Itoa(total),
	})
	return items, total, nil
}

// ListPatientPlans returns the treatment plans of a single patient, listed
// with their items (S1).
func (s *TreatmentService) ListPatientPlans(ctx context.Context, actor ActorMeta, patientID string, limit, offset int) ([]PatientTreatmentPlanDTO, int, error) {
	if err := requireTreatmentReadRole(actor); err != nil {
		return nil, 0, err
	}
	patientID = strings.TrimSpace(patientID)
	filter, err := normalizePlanFilter(PlanFilter{PatientID: patientID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}

	plans, total, err := s.repo.ListPlans(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	items := make([]PatientTreatmentPlanDTO, 0, len(plans))
	for _, plan := range plans {
		if plan == nil {
			continue
		}
		items = append(items, expandPlan(plan))
	}

	s.recordAudit(ctx, actor, auditdom.ActionExport, "TREATMENT_PLAN", patientID, map[string]string{
		"total": strconv.Itoa(total),
	})
	return items, total, nil
}

// summarizePlan projects a plan for the global listing.
func summarizePlan(plan *domain.TreatmentPlan) TreatmentPlanSummaryDTO {
	return TreatmentPlanSummaryDTO{
		ID:                 plan.ID,
		PatientID:          plan.PatientID,
		Title:              plan.Title,
		Currency:           plan.Currency,
		Status:             plan.Status,
		TotalApprovedCents: plan.ApprovedTotalCents,
		TotalPlannedCents:  plan.TotalNetCents,
		CreatedAt:          plan.CreatedAt,
		UpdatedAt:          plan.UpdatedAt,
	}
}

// expandPlan projects a plan with its items for the per patient listing.
func expandPlan(plan *domain.TreatmentPlan) PatientTreatmentPlanDTO {
	items := make([]TreatmentPlanItemDTO, 0)
	for _, phase := range plan.Phases {
		for _, item := range phase.Items {
			items = append(items, TreatmentPlanItemDTO{
				ID:            item.ID,
				PhaseNumber:   item.PhaseNumber,
				ProcedureCode: item.ProcedureCode,
				ProcedureName: item.ProcedureName,
				ToothFDI:      item.ToothFDI,
				NetCents:      item.NetCents,
				Status:        item.Status,
			})
		}
	}
	return PatientTreatmentPlanDTO{
		ID:                 plan.ID,
		PatientID:          plan.PatientID,
		Title:              plan.Title,
		Currency:           plan.Currency,
		Status:             plan.Status,
		TotalApprovedCents: plan.ApprovedTotalCents,
		TotalPlannedCents:  plan.TotalNetCents,
		Items:              items,
		CreatedAt:          plan.CreatedAt,
		UpdatedAt:          plan.UpdatedAt,
	}
}

// normalizePlanFilter maps the public vocabulary of the listing API onto the
// domain statuses and applies the pagination contract: limit defaults to 50
// with a hard ceiling of 200, offset defaults to 0.
func normalizePlanFilter(filter PlanFilter) (PlanFilter, error) {
	filter.PatientID = strings.TrimSpace(filter.PatientID)
	filter.Query = strings.TrimSpace(filter.Query)

	status, err := normalizePlanStatus(filter.Status)
	if err != nil {
		return filter, err
	}
	filter.Status = status

	if filter.Limit <= 0 {
		filter.Limit = DefaultListLimit
	} else if filter.Limit > MaxListLimit {
		filter.Limit = MaxListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter, nil
}

// normalizePlanStatus maps the public status vocabulary of the listing API
// (DRAFT | PARTIALLY_APPROVED | APPROVED | COMPLETED) onto the domain
// statuses (DRAFT | APPROVED_PARTIAL | APPROVED_TOTAL | COMPLETED).
func normalizePlanStatus(status string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "":
		return "", nil
	case "DRAFT":
		return "DRAFT", nil
	case "PARTIALLY_APPROVED", "APPROVED_PARTIAL":
		return "APPROVED_PARTIAL", nil
	case "APPROVED", "APPROVED_TOTAL":
		return "APPROVED_TOTAL", nil
	case "COMPLETED":
		return "COMPLETED", nil
	default:
		return "", ErrInvalidPlanStatus
	}
}

// requireTreatmentReadRole restricts the plan listings to the roles allowed to
// read treatment and budget data (SRS §3). The auditor role has no access.
func requireTreatmentReadRole(actor ActorMeta) error {
	switch strings.ToLower(strings.TrimSpace(actor.ActorRole)) {
	case "admin", "dentist", "receptionist", "assistant":
		return nil
	default:
		return ErrTreatmentReadForbiddenRole
	}
}

// recordAudit appends the immutable audit trail of the operation. A failing
// recorder must never invalidate the business response, but the failure is
// logged instead of being silently discarded (RNF-06).
func (s *TreatmentService) recordAudit(ctx context.Context, actor ActorMeta, action auditdom.AuditAction, resType, resID string, meta map[string]string) {
	if s.audit == nil {
		return
	}
	if _, err := s.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      actor.ActorID,
		ActorRole:    actor.ActorRole,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		SourceIP:     actor.SourceIP,
		Metadata:     meta,
	}); err != nil {
		log.Printf("audit trail could not be recorded (action=%s resource=%s/%s): %v", action, resType, resID, err)
	}
}

func genUUID() string {
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
