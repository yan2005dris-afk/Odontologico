package postgres

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/treatments/application"
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

// ListPlans returns the treatment plans matching the filter, ordered by
// creation date (descending) with the id as tiebreaker so that pagination
// never duplicates nor skips a plan between pages.
func (r *TreatmentRepository) ListPlans(_ context.Context, filter application.PlanFilter) ([]*domain.TreatmentPlan, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query := strings.ToLower(strings.TrimSpace(filter.Query))
	matched := make([]*domain.TreatmentPlan, 0, len(r.plansByID))
	for _, plan := range r.plansByID {
		if filter.PatientID != "" && plan.PatientID != filter.PatientID {
			continue
		}
		if filter.Status != "" && plan.Status != filter.Status {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(plan.Title), query) {
			continue
		}
		matched = append(matched, clonePlan(plan))
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID < matched[j].ID
	})

	total := len(matched)
	limit := filter.Limit
	if limit <= 0 {
		limit = application.DefaultListLimit
	} else if limit > application.MaxListLimit {
		limit = application.MaxListLimit
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []*domain.TreatmentPlan{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
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
