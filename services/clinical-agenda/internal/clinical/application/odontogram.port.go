package application

import (
	"context"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type OdontogramRepository interface {
	SaveFinding(ctx context.Context, finding *domain.OdontogramFinding) error
	GetFindingByID(ctx context.Context, patientID, findingID string) (*domain.OdontogramFinding, error)
	DeleteFinding(ctx context.Context, patientID, findingID string) error
	ListFindingsByPatient(ctx context.Context, patientID string) ([]*domain.OdontogramFinding, error)
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

type RecordFindingInput struct {
	ToothCode      string `json:"tooth_code"`
	NotationSystem string `json:"notation_system"`
	Surface        string `json:"surface"`
	FindingType    string `json:"finding_type"`
	Status         string `json:"status"`
	Notes          string `json:"notes"`
}

type UpdateFindingInput struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type FindingAddendumInput struct {
	Note string `json:"note"`
}

type OdontogramSnapshotDTO struct {
	PatientID     string                      `json:"patient_id"`
	DentitionMode string                      `json:"dentition_mode"`
	Findings      []*domain.OdontogramFinding `json:"findings"`
}
