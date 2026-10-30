package postgres

import (
	"context"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

const (
	InsertTreatmentPlanSQL   = `INSERT INTO treatment_plans (id, patient_id, title, currency, status, total_gross_cents, total_discount_cents, total_net_cents, approved_total_cents, pending_total_cents, patient_acceptance_ref, created_by, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
	InsertSchedulableTaskSQL = `INSERT INTO schedulable_tasks (task_id, patient_id, plan_id, treatment_item_id, phase_number, procedure_code, procedure_name, tooth_fdi, estimated_duration_minutes, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())`
)

type TreatmentRepository struct {
	mu             sync.RWMutex
	plansByID      map[string]*domain.TreatmentPlan
	tasksByPatient map[string][]domain.SchedulableTask
}

func NewTreatmentRepository() *TreatmentRepository {
	return &TreatmentRepository{
		plansByID:      make(map[string]*domain.TreatmentPlan),
		tasksByPatient: make(map[string][]domain.SchedulableTask),
	}
}

func (r *TreatmentRepository) SavePlan(_ context.Context, plan *domain.TreatmentPlan) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plansByID[plan.ID] = clonePlan(plan)
	return nil
}

func (r *TreatmentRepository) GetPlanByID(_ context.Context, planID string) (*domain.TreatmentPlan, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	plan, ok := r.plansByID[planID]
	if !ok {
		return nil, domain.ErrTreatmentPlanNotFound
	}
	return clonePlan(plan), nil
}

func (r *TreatmentRepository) SavePlanApprovalTx(_ context.Context, plan *domain.TreatmentPlan, newTasks []domain.SchedulableTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plansByID[plan.ID] = clonePlan(plan)
	if len(newTasks) > 0 {
		r.tasksByPatient[plan.PatientID] = append(r.tasksByPatient[plan.PatientID], newTasks...)
	}
	return nil
}

func (r *TreatmentRepository) ListSchedulableTasksByPatient(_ context.Context, patientID string) ([]domain.SchedulableTask, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.tasksByPatient[patientID]
	out := make([]domain.SchedulableTask, len(list))
	copy(out, list)
	return out, nil
}

func clonePlan(p *domain.TreatmentPlan) *domain.TreatmentPlan {
	cp := *p
	cp.Phases = make([]domain.TreatmentPhase, len(p.Phases))
	for i, ph := range p.Phases {
		cpPh := ph
		cpPh.Items = make([]domain.TreatmentItem, len(ph.Items))
		copy(cpPh.Items, ph.Items)
		cp.Phases[i] = cpPh
	}
	return &cp
}
