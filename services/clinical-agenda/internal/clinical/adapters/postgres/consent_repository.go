package postgres

import (
	"context"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

const (
	InsertConsentSQL = `INSERT INTO patient_consents (id, patient_id, procedure_code, title, rendered_text, status, signature_method, signed_by_name, signed_by_document, document_sha256, storage_object_key, signed_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
)

type ConsentRepository struct {
	mu        sync.RWMutex
	byPatient map[string][]*domain.PatientConsent
}

func NewConsentRepository() *ConsentRepository {
	return &ConsentRepository{
		byPatient: make(map[string][]*domain.PatientConsent),
	}
}

func (r *ConsentRepository) SaveConsent(_ context.Context, consent *domain.PatientConsent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *consent
	list := r.byPatient[consent.PatientID]
	for i, existing := range list {
		if existing.ID == consent.ID {
			list[i] = &copied
			r.byPatient[consent.PatientID] = list
			return nil
		}
	}
	r.byPatient[consent.PatientID] = append(list, &copied)
	return nil
}

func (r *ConsentRepository) GetConsentByID(_ context.Context, patientID, consentID string) (*domain.PatientConsent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.byPatient[patientID] {
		if c.ID == consentID {
			cp := *c
			return &cp, nil
		}
	}
	return nil, domain.ErrConsentNotFound
}

func (r *ConsentRepository) ListConsentsByPatient(_ context.Context, patientID string) ([]*domain.PatientConsent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byPatient[patientID]
	out := make([]*domain.PatientConsent, len(list))
	for i, c := range list {
		cp := *c
		out[i] = &cp
	}
	return out, nil
}
