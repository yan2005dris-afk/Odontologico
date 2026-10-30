package postgres

import (
	"context"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

const (
	InsertFindingSQL = `INSERT INTO odontogram_findings (id, patient_id, tooth_fdi, tooth_universal, dentition_type, surface, finding_type, color_hex, status, notes, recorded_by, recorded_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
)

type OdontogramRepository struct {
	mu        sync.RWMutex
	byPatient map[string][]*domain.OdontogramFinding
}

func NewOdontogramRepository() *OdontogramRepository {
	return &OdontogramRepository{
		byPatient: make(map[string][]*domain.OdontogramFinding),
	}
}

func (r *OdontogramRepository) SaveFinding(_ context.Context, finding *domain.OdontogramFinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := cloneFinding(finding)
	list := r.byPatient[finding.PatientID]
	for i, existing := range list {
		if existing.ID == finding.ID {
			list[i] = copied
			r.byPatient[finding.PatientID] = list
			return nil
		}
	}
	r.byPatient[finding.PatientID] = append(list, copied)
	return nil
}

func (r *OdontogramRepository) GetFindingByID(_ context.Context, patientID, findingID string) (*domain.OdontogramFinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, f := range r.byPatient[patientID] {
		if f.ID == findingID {
			return cloneFinding(f), nil
		}
	}
	return nil, domain.ErrFindingNotFound
}

func (r *OdontogramRepository) DeleteFinding(_ context.Context, patientID, findingID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.byPatient[patientID]
	for i, f := range list {
		if f.ID == findingID {
			if f.Status == "REALIZADO" || f.Immutable {
				return domain.ErrClinicalRecordImmutable
			}
			r.byPatient[patientID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return domain.ErrFindingNotFound
}

func (r *OdontogramRepository) ListFindingsByPatient(_ context.Context, patientID string) ([]*domain.OdontogramFinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byPatient[patientID]
	out := make([]*domain.OdontogramFinding, len(list))
	for i, f := range list {
		out[i] = cloneFinding(f)
	}
	return out, nil
}

func cloneFinding(f *domain.OdontogramFinding) *domain.OdontogramFinding {
	c := *f
	if f.Addenda != nil {
		c.Addenda = make([]domain.FindingAddendum, len(f.Addenda))
		copy(c.Addenda, f.Addenda)
	}
	return &c
}
