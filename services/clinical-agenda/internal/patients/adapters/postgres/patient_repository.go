package postgres

import (
	"context"
	"strings"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/patients/domain"
)

const (
	InsertPatientSQL   = `INSERT INTO patients (id, hc_number, document_type, document_number, first_name, last_name, birth_date, sex, phone, email, emergency_contact_json, legal_guardian_json, insurance_plan_json, anamnesis_recorded, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`
	InsertAnamnesisSQL = `INSERT INTO patient_anamnesis (id, patient_id, systemic_diseases_json, drug_allergies_json, surgical_history_json, coagulopathies_json, is_pregnant, notes, recorded_by, recorded_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
)

type PatientRepository struct {
	mu         sync.RWMutex
	seq        int
	byID       map[string]*domain.Patient
	byDocument map[string]string
	anamnesis  map[string][]*domain.Anamnesis
	order      []string
}

func NewPatientRepository() *PatientRepository {
	return &PatientRepository{
		seq:        141,
		byID:       make(map[string]*domain.Patient),
		byDocument: make(map[string]string),
		anamnesis:  make(map[string][]*domain.Anamnesis),
		order:      make([]string, 0),
	}
}

func (r *PatientRepository) NextSequence(_ context.Context) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return r.seq
}

func (r *PatientRepository) Create(_ context.Context, patient *domain.Patient) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	docKey := strings.ToUpper(strings.TrimSpace(patient.DocumentNumber))
	if _, exists := r.byDocument[docKey]; exists {
		return domain.ErrDuplicatePatientDocument
	}
	copied := clonePatient(patient)
	r.byID[copied.ID] = copied
	r.byDocument[docKey] = copied.ID
	r.order = append(r.order, copied.ID)
	return nil
}

func (r *PatientRepository) Update(_ context.Context, patient *domain.Patient) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.byID[patient.ID]
	if !ok {
		return domain.ErrPatientNotFound
	}
	newDocKey := strings.ToUpper(strings.TrimSpace(patient.DocumentNumber))
	if ownerID, exists := r.byDocument[newDocKey]; exists && ownerID != patient.ID {
		return domain.ErrDuplicatePatientDocument
	}
	delete(r.byDocument, strings.ToUpper(strings.TrimSpace(existing.DocumentNumber)))
	copied := clonePatient(patient)
	r.byID[copied.ID] = copied
	r.byDocument[newDocKey] = copied.ID
	return nil
}

func (r *PatientRepository) GetByID(_ context.Context, id string) (*domain.Patient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrPatientNotFound
	}
	return clonePatient(p), nil
}

func (r *PatientRepository) Search(_ context.Context, query string, limit, offset int) ([]*domain.Patient, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	var matched []*domain.Patient
	for _, id := range r.order {
		p := r.byID[id]
		if q == "" ||
			strings.Contains(strings.ToLower(p.DocumentNumber), q) ||
			strings.Contains(strings.ToLower(p.HCNumber), q) ||
			strings.Contains(strings.ToLower(p.FirstName+" "+p.LastName), q) {
			matched = append(matched, clonePatient(p))
		}
	}

	total := len(matched)
	if offset >= total {
		return []*domain.Patient{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}

func (r *PatientRepository) SaveAnamnesis(_ context.Context, anamnesis *domain.Anamnesis, alerts []domain.CriticalAlert) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[anamnesis.PatientID]
	if !ok {
		return domain.ErrPatientNotFound
	}
	copiedAnam := *anamnesis
	r.anamnesis[anamnesis.PatientID] = append(r.anamnesis[anamnesis.PatientID], &copiedAnam)
	p.AnamnesisRecorded = true
	p.CriticalAlerts = make([]domain.CriticalAlert, len(alerts))
	copy(p.CriticalAlerts, alerts)
	p.UpdatedAt = anamnesis.RecordedAt
	return nil
}

func clonePatient(p *domain.Patient) *domain.Patient {
	c := *p
	if p.LegalGuardian != nil {
		lg := *p.LegalGuardian
		c.LegalGuardian = &lg
	}
	if p.InsurancePlan != nil {
		ip := *p.InsurancePlan
		c.InsurancePlan = &ip
	}
	if p.CriticalAlerts != nil {
		c.CriticalAlerts = make([]domain.CriticalAlert, len(p.CriticalAlerts))
		copy(c.CriticalAlerts, p.CriticalAlerts)
	} else {
		c.CriticalAlerts = []domain.CriticalAlert{}
	}
	return &c
}
