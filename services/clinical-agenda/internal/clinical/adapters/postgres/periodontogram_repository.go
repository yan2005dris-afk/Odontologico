package postgres

import (
	"context"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

const (
	InsertPerioChartSQL = `INSERT INTO periodontogram_charts (id, patient_id, bop_percentage, mean_nic_mm, recorded_by, recorded_at) VALUES ($1,$2,$3,$4,$5,$6)`
)

type PeriodontogramRepository struct {
	mu        sync.RWMutex
	byPatient map[string][]*domain.PeriodontogramChart
}

func NewPeriodontogramRepository() *PeriodontogramRepository {
	return &PeriodontogramRepository{
		byPatient: make(map[string][]*domain.PeriodontogramChart),
	}
}

func (r *PeriodontogramRepository) SaveChart(_ context.Context, chart *domain.PeriodontogramChart) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *chart
	copied.Teeth = make([]domain.PeriodontogramTooth, len(chart.Teeth))
	copy(copied.Teeth, chart.Teeth)
	r.byPatient[chart.PatientID] = append(r.byPatient[chart.PatientID], &copied)
	return nil
}

func (r *PeriodontogramRepository) ListByPatient(_ context.Context, patientID string) ([]*domain.PeriodontogramChart, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byPatient[patientID]
	out := make([]*domain.PeriodontogramChart, len(list))
	for i, c := range list {
		cp := *c
		cp.Teeth = make([]domain.PeriodontogramTooth, len(c.Teeth))
		copy(cp.Teeth, c.Teeth)
		out[i] = &cp
	}
	return out, nil
}
