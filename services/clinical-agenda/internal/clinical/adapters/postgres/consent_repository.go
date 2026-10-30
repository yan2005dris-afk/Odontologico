package postgres

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/application"
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

// ListConsents returns the informed consents matching the filter, ordered by
// creation date (descending) with the id as tiebreaker so that pagination
// never duplicates nor skips a consent between pages.
func (r *ConsentRepository) ListConsents(_ context.Context, filter application.ConsentFilter) ([]*domain.PatientConsent, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]*domain.PatientConsent, 0)
	for _, list := range r.byPatient {
		for _, c := range list {
			if filter.PatientID != "" && c.PatientID != filter.PatientID {
				continue
			}
			if filter.Status != "" && c.Status != filter.Status {
				continue
			}
			if filter.SignatureMethod != "" && !strings.EqualFold(c.SignatureMethod, filter.SignatureMethod) {
				continue
			}
			if filter.ProcedureCode != "" && !strings.EqualFold(c.ProcedureCode, filter.ProcedureCode) {
				continue
			}
			cp := *c
			matched = append(matched, &cp)
		}
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
		return []*domain.PatientConsent{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}
