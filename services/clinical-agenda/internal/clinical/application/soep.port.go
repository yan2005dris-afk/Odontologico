package application

import (
	"context"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type SOEPRepository interface {
	SaveNote(ctx context.Context, note *domain.SOEPNote) error
	GetNoteByID(ctx context.Context, patientID, noteID string) (*domain.SOEPNote, error)
	ListNotesByPatient(ctx context.Context, patientID string) ([]*domain.SOEPNote, error)
}

type CreateSOEPNoteInput struct {
	AppointmentID string `json:"appointment_id"`
	ProcedureCode string `json:"procedure_code"`
	Subjective    string `json:"subjective"`
	Objective     string `json:"objective"`
	Assessment    string `json:"assessment"`
	Plan          string `json:"plan"`
}

type UpdateSOEPNoteInput struct {
	Subjective string `json:"subjective"`
	Objective  string `json:"objective"`
	Assessment string `json:"assessment"`
	Plan       string `json:"plan"`
}

type CreateSOEPAddendumInput struct {
	Reason         string `json:"reason"`
	CorrectionText string `json:"correction_text"`
}
