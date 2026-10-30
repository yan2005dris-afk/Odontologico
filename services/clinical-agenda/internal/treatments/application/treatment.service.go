package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

var (
	ErrTreatmentForbiddenRole = errors.New("actor role is not authorized for this treatment plan operation")
	ErrTreatmentMFARequired   = errors.New("MFA verification is required for treatment plan operations")
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

func (s *TreatmentService) recordAudit(ctx context.Context, actor ActorMeta, action auditdom.AuditAction, resType, resID string, meta map[string]string) {
	if s.audit == nil {
		return
	}
	_, _ = s.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      actor.ActorID,
		ActorRole:    actor.ActorRole,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		SourceIP:     actor.SourceIP,
		Metadata:     meta,
	})
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
