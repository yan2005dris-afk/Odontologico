package application

import (
	"context"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

type TreatmentRepository interface {
	SavePlan(ctx context.Context, plan *domain.TreatmentPlan) error
	GetPlanByID(ctx context.Context, planID string) (*domain.TreatmentPlan, error)
	SavePlanApprovalTx(ctx context.Context, plan *domain.TreatmentPlan, newTasks []domain.SchedulableTask) error
	ListSchedulableTasksByPatient(ctx context.Context, patientID string) ([]domain.SchedulableTask, error)
}

type Clock interface {
	Now() time.Time
}

type ActorMeta struct {
	ActorID     string
	ActorRole   string
	MFAVerified bool
	SourceIP    string
}

type CreateTreatmentItemInput struct {
	ID                       string `json:"id"`
	ProcedureCode            string `json:"procedure_code"`
	ProcedureName            string `json:"procedure_name"`
	ToothFDI                 string `json:"tooth_fdi"`
	Surface                  string `json:"surface"`
	ProfessionalFeeCents     int64  `json:"professional_fee_cents"`
	LabCostCents             int64  `json:"lab_cost_cents"`
	DiscountCents            int64  `json:"discount_cents"`
	EstimatedDurationMinutes int    `json:"estimated_duration_minutes"`
}

type CreateTreatmentPhaseInput struct {
	PhaseNumber int                        `json:"phase_number"`
	Name        string                     `json:"name"`
	Items       []CreateTreatmentItemInput `json:"items"`
}

type CreateTreatmentPlanInput struct {
	Title    string                      `json:"title"`
	Currency string                      `json:"currency"`
	Phases   []CreateTreatmentPhaseInput `json:"phases"`
}

type ApprovePlanInput struct {
	Mode                 string   `json:"mode"`
	ApprovedItemIDs      []string `json:"approved_item_ids"`
	PatientAcceptanceRef string   `json:"patient_acceptance_ref"`
}

type ApprovePlanOutput struct {
	PlanID                    string                   `json:"plan_id"`
	PatientID                 string                   `json:"patient_id"`
	Status                    string                   `json:"status"`
	ApprovedTotalCents        int64                    `json:"approved_total_cents"`
	PendingTotalCents         int64                    `json:"pending_total_cents"`
	GeneratedSchedulableTasks []domain.SchedulableTask `json:"generated_schedulable_tasks"`
}
