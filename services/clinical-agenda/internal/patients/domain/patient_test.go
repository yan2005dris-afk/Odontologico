package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/patients/domain"
)

func TestNewPatient_AdultAndMinorGuardianRule(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 0, 0, 0, time.UTC)

	// 1. Valid adult without guardian succeeds
	adult, err := domain.NewPatient(domain.CreatePatientParams{
		ID:             "p-1",
		HCNumber:       "HC-2026-000142",
		DocumentType:   "DNI",
		DocumentNumber: "40123987",
		FirstName:      "Martín",
		LastName:       "Echeverría",
		BirthDate:      "1982-04-15",
		Sex:            "M",
		Phone:          "+5491133445566",
		EmergencyContact: domain.EmergencyContact{
			FullName:     "Laura Echeverría",
			Phone:        "+5491144556677",
			Relationship: "Spouse",
		},
	}, now)
	if err != nil {
		t.Fatalf("expected valid adult patient, got %v", err)
	}
	if adult.IsMinor || adult.Age != 44 {
		t.Fatalf("expected age=44 is_minor=false, got age=%d is_minor=%v", adult.Age, adult.IsMinor)
	}

	// Validation branches
	if _, err := domain.NewPatient(domain.CreatePatientParams{DocumentType: "INVALID"}, now); !errors.Is(err, domain.ErrInvalidPatientDemographics) {
		t.Fatalf("expected ErrInvalidPatientDemographics")
	}
	if _, err := domain.NewPatient(domain.CreatePatientParams{DocumentType: "DNI", DocumentNumber: "123"}, now); !errors.Is(err, domain.ErrInvalidPatientDemographics) {
		t.Fatalf("expected ErrInvalidPatientDemographics for short doc")
	}
	if _, err := domain.NewPatient(domain.CreatePatientParams{DocumentType: "DNI", DocumentNumber: "123456", FirstName: "A", LastName: "B", Phone: "1", Sex: "Z"}, now); !errors.Is(err, domain.ErrInvalidPatientDemographics) {
		t.Fatalf("expected ErrInvalidPatientDemographics for invalid sex")
	}
	if _, err := domain.NewPatient(domain.CreatePatientParams{DocumentType: "DNI", DocumentNumber: "123456", FirstName: "A", LastName: "B", Phone: "1", Sex: "F", BirthDate: "2099-01-01"}, now); !errors.Is(err, domain.ErrInvalidBirthDate) {
		t.Fatalf("expected ErrInvalidBirthDate for future date")
	}
	if _, err := domain.NewPatient(domain.CreatePatientParams{DocumentType: "DNI", DocumentNumber: "123456", FirstName: "A", LastName: "B", Phone: "1", Sex: "F", BirthDate: "1990-01-01"}, now); !errors.Is(err, domain.ErrEmergencyContactRequired) {
		t.Fatalf("expected ErrEmergencyContactRequired")
	}

	// 2. Minor (<18) without legal_guardian fails with ErrLegalGuardianRequiredMinor (S1)
	_, err = domain.NewPatient(domain.CreatePatientParams{
		ID:             "p-2",
		HCNumber:       "HC-2026-000143",
		DocumentType:   "DNI",
		DocumentNumber: "55123987",
		FirstName:      "Sofía",
		LastName:       "Echeverría",
		BirthDate:      "2014-12-25",
		Sex:            "F",
		Phone:          "+5491133445566",
		EmergencyContact: domain.EmergencyContact{
			FullName:     "Laura Echeverría",
			Phone:        "+5491144556677",
			Relationship: "Mother",
		},
	}, now)
	if !errors.Is(err, domain.ErrLegalGuardianRequiredMinor) {
		t.Fatalf("expected ErrLegalGuardianRequiredMinor, got %v", err)
	}

	// 3. Minor (<18) with complete legal_guardian succeeds (S1)
	minor, err := domain.NewPatient(domain.CreatePatientParams{
		ID:             "p-2",
		HCNumber:       "HC-2026-000143",
		DocumentType:   "DNI",
		DocumentNumber: "55123987",
		FirstName:      "Sofía",
		LastName:       "Echeverría",
		BirthDate:      "2014-12-25",
		Sex:            "F",
		Phone:          "+5491133445566",
		EmergencyContact: domain.EmergencyContact{
			FullName:     "Laura Echeverría",
			Phone:        "+5491144556677",
			Relationship: "Mother",
		},
		LegalGuardian: &domain.LegalGuardian{
			FullName:       "Laura Echeverría",
			DocumentNumber: "30111222",
			Phone:          "+5491144556677",
			Relationship:   "Mother",
		},
	}, time.Time{})
	if err != nil || !minor.IsMinor {
		t.Fatalf("expected valid minor, got %+v err=%v", minor, err)
	}
}

func TestDeriveCriticalAlerts(t *testing.T) {
	if len(domain.DeriveCriticalAlerts(nil)) != 0 {
		t.Fatalf("expected empty alerts on nil anamnesis")
	}
	if _, _, err := domain.NewAnamnesis("a", "", "d", nil, nil, nil, nil, false, "", time.Time{}); !errors.Is(err, domain.ErrInvalidAnamnesis) {
		t.Fatalf("expected ErrInvalidAnamnesis")
	}
	if _, _, err := domain.NewAnamnesis("a", "p", "d", nil, []domain.DrugAllergy{{Substance: "X", Severity: "BAD"}}, nil, nil, false, "", time.Time{}); !errors.Is(err, domain.ErrInvalidAnamnesis) {
		t.Fatalf("expected ErrInvalidAnamnesis for bad severity")
	}

	_, alerts, err := domain.NewAnamnesis(
		"a-1",
		"p-1",
		"dentist-1",
		[]domain.SystemicDisease{
			{Code: "I10", Name: "Endocarditis infecciosa previa", Controlled: true},
		},
		[]domain.DrugAllergy{
			{Substance: "Penicilina", Reaction: "Anafilaxia", Severity: "CRITICAL"},
			{Substance: "Latex", Reaction: "Edema", Severity: "MODERATE"},
		},
		nil,
		[]domain.Coagulopathy{
			{Condition: "Fibrilación Auricular", Medication: "Warfarina", INRLastValue: 2.8, IsAnticoagulated: true},
			{Condition: "Hemofilia A", IsAnticoagulated: false},
		},
		true,
		"Control de INR previo a exodoncia",
		time.Time{},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(alerts) != 6 {
		t.Fatalf("expected 6 critical alerts, got %d: %+v", len(alerts), alerts)
	}
}
