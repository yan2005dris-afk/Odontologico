package application

import (
	"context"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/patients/domain"
)

type PatientRepository interface {
	Create(ctx context.Context, patient *domain.Patient) error
	Update(ctx context.Context, patient *domain.Patient) error
	GetByID(ctx context.Context, id string) (*domain.Patient, error)
	Search(ctx context.Context, query string, limit, offset int) ([]*domain.Patient, int, error)
	SaveAnamnesis(ctx context.Context, anamnesis *domain.Anamnesis, alerts []domain.CriticalAlert) error
	NextSequence(ctx context.Context) int
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

type UpsertPatientInput struct {
	DocumentType     string                  `json:"document_type"`
	DocumentNumber   string                  `json:"document_number"`
	FirstName        string                  `json:"first_name"`
	LastName         string                  `json:"last_name"`
	BirthDate        string                  `json:"birth_date"`
	Sex              string                  `json:"sex"`
	Phone            string                  `json:"phone"`
	Email            string                  `json:"email"`
	EmergencyContact domain.EmergencyContact `json:"emergency_contact"`
	LegalGuardian    *domain.LegalGuardian   `json:"legal_guardian"`
	InsurancePlan    *domain.InsurancePlan   `json:"insurance_plan"`
}

type UpsertAnamnesisInput struct {
	SystemicDiseases []domain.SystemicDisease `json:"systemic_diseases"`
	DrugAllergies    []domain.DrugAllergy     `json:"drug_allergies"`
	SurgicalHistory  []domain.SurgicalHistory `json:"surgical_history"`
	Coagulopathies   []domain.Coagulopathy    `json:"coagulopathies"`
	IsPregnant       bool                     `json:"is_pregnant"`
	Notes            string                   `json:"notes"`
}

type PatientHeaderDTO struct {
	ID                string                 `json:"id"`
	HCNumber          string                 `json:"hc_number"`
	FullName          string                 `json:"full_name"`
	DocumentType      string                 `json:"document_type"`
	DocumentNumber    string                 `json:"document_number"`
	Age               int                    `json:"age"`
	IsMinor           bool                   `json:"is_minor"`
	AnamnesisRecorded bool                   `json:"anamnesis_recorded"`
	CriticalAlerts    []domain.CriticalAlert `json:"critical_alerts"`
}
