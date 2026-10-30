package postgres

import (
	"context"
	"sync"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

const (
	InsertSOEPNoteSQL     = `INSERT INTO soep_notes (id, patient_id, appointment_id, procedure_code, author_dentist_id, subjective, objective, assessment, plan, signature_hash, created_at, locks_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	InsertSOEPAddendumSQL = `INSERT INTO soep_addenda (id, parent_note_id, author_dentist_id, reason, correction_text, signature_hash, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
)

type SOEPRepository struct {
	mu        sync.RWMutex
	byPatient map[string][]*domain.SOEPNote
}

func NewSOEPRepository() *SOEPRepository {
	return &SOEPRepository{
		byPatient: make(map[string][]*domain.SOEPNote),
	}
}

func (r *SOEPRepository) SaveNote(_ context.Context, note *domain.SOEPNote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := cloneSOEPNote(note)
	list := r.byPatient[note.PatientID]
	for i, existing := range list {
		if existing.ID == note.ID {
			list[i] = copied
			r.byPatient[note.PatientID] = list
			return nil
		}
	}
	r.byPatient[note.PatientID] = append(list, copied)
	return nil
}

func (r *SOEPRepository) GetNoteByID(_ context.Context, patientID, noteID string) (*domain.SOEPNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, n := range r.byPatient[patientID] {
		if n.ID == noteID {
			return cloneSOEPNote(n), nil
		}
	}
	return nil, domain.ErrSOEPNotFound
}

func (r *SOEPRepository) ListNotesByPatient(_ context.Context, patientID string) ([]*domain.SOEPNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byPatient[patientID]
	out := make([]*domain.SOEPNote, len(list))
	for i, n := range list {
		out[i] = cloneSOEPNote(n)
	}
	return out, nil
}

func cloneSOEPNote(n *domain.SOEPNote) *domain.SOEPNote {
	c := *n
	c.Addenda = make([]domain.SOEPAddendum, len(n.Addenda))
	copy(c.Addenda, n.Addenda)
	return &c
}
