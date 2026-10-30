package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/patients/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/patients/application"
	"github.com/sigo-dental/clinical-agenda/internal/patients/domain"
)

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time { return f.now }

func TestPatientService_CreateSearchAnamnesisAndAudit(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 0, 0, 0, time.UTC)
	repo := postgres.NewPatientRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, fixedClock{now: now}, nil)
	svc := application.NewPatientService(repo, auditSvc, fixedClock{now: now})
	_ = application.NewPatientService(repo, nil, nil)

	receptionist := application.ActorMeta{
		ActorID:     "rec-1",
		ActorRole:   "receptionist",
		MFAVerified: true,
		SourceIP:    "10.0.0.10",
	}

	// MFA & role checks across methods
	for _, fn := range []func() error{
		func() error {
			_, err := svc.CreatePatient(context.Background(), application.ActorMeta{MFAVerified: false}, application.UpsertPatientInput{})
			return err
		},
		func() error {
			_, err := svc.UpdatePatient(context.Background(), application.ActorMeta{MFAVerified: false}, "p", application.UpsertPatientInput{})
			return err
		},
		func() error {
			_, err := svc.GetPatient(context.Background(), application.ActorMeta{MFAVerified: false}, "p")
			return err
		},
		func() error {
			_, _, err := svc.SearchPatients(context.Background(), application.ActorMeta{MFAVerified: false}, "", 10, 0)
			return err
		},
		func() error {
			_, err := svc.UpsertAnamnesis(context.Background(), application.ActorMeta{MFAVerified: false}, "p", application.UpsertAnamnesisInput{})
			return err
		},
	} {
		if !errors.Is(fn(), application.ErrMFARequired) {
			t.Fatalf("expected ErrMFARequired")
		}
	}
	if _, err := svc.CreatePatient(context.Background(), application.ActorMeta{MFAVerified: true, ActorRole: "auditor"}, application.UpsertPatientInput{}); !errors.Is(err, application.ErrForbiddenRole) {
		t.Fatalf("expected ErrForbiddenRole for auditor on CreatePatient")
	}
	if _, err := svc.GetPatient(context.Background(), application.ActorMeta{MFAVerified: true, ActorRole: "unknown"}, "p"); !errors.Is(err, application.ErrForbiddenRole) {
		t.Fatalf("expected ErrForbiddenRole for unknown role on GetPatient")
	}

	// S1: Create adult patient
	input := application.UpsertPatientInput{
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
	}
	created, err := svc.CreatePatient(context.Background(), receptionist, input)
	if err != nil {
		t.Fatalf("expected patient created, got %v", err)
	}

	// UpdatePatient
	input.Phone = "+5491199998888"
	updated, err := svc.UpdatePatient(context.Background(), receptionist, created.ID, input)
	if err != nil || updated.Phone != "+5491199998888" {
		t.Fatalf("expected patient updated, got %+v err=%v", updated, err)
	}

	// GetPatient & GetPatientHeader
	header, err := svc.GetPatientHeader(context.Background(), receptionist, created.ID)
	if err != nil || header.HCNumber != created.HCNumber {
		t.Fatalf("expected patient header, got %+v err=%v", header, err)
	}

	// S1: Duplicate document number returns ErrDuplicatePatientDocument
	_, err = svc.CreatePatient(context.Background(), receptionist, input)
	if !errors.Is(err, domain.ErrDuplicatePatientDocument) {
		t.Fatalf("expected ErrDuplicatePatientDocument, got %v", err)
	}

	// S3: Receptionist cannot record clinical anamnesis -> ErrForbiddenRole
	_, err = svc.UpsertAnamnesis(context.Background(), receptionist, created.ID, application.UpsertAnamnesisInput{})
	if !errors.Is(err, application.ErrForbiddenRole) {
		t.Fatalf("expected ErrForbiddenRole for receptionist on anamnesis, got %v", err)
	}

	// S3 & S4: Dentist records anamnesis and derives critical alerts
	dentist := application.ActorMeta{
		ActorID:     "dent-1",
		ActorRole:   "dentist",
		MFAVerified: true,
		SourceIP:    "10.0.0.20",
	}
	withAnam, err := svc.UpsertAnamnesis(context.Background(), dentist, created.ID, application.UpsertAnamnesisInput{
		DrugAllergies: []domain.DrugAllergy{
			{Substance: "Penicilina", Reaction: "Anafilaxia", Severity: "CRITICAL"},
		},
		Coagulopathies: []domain.Coagulopathy{
			{Condition: "FA", Medication: "Warfarina", INRLastValue: 2.8, IsAnticoagulated: true},
		},
	})
	if err != nil {
		t.Fatalf("expected anamnesis success, got %v", err)
	}
	if !withAnam.AnamnesisRecorded || len(withAnam.CriticalAlerts) != 2 {
		t.Fatalf("expected 2 critical alerts, got %+v", withAnam.CriticalAlerts)
	}

	// S2: Fast search
	results, total, err := svc.SearchPatients(context.Background(), dentist, "40123987", 500, -1)
	if err != nil || total != 1 || len(results) != 1 {
		t.Fatalf("expected 1 search result, got total=%d err=%v", total, err)
	}

	// S5: Audit trail recorded
	logs, logTotal, err := auditSvc.ListLogs(context.Background(), "auditor", auditapp.AuditFilter{})
	if err != nil || logTotal < 3 || len(logs) < 3 {
		t.Fatalf("expected >= 3 audit logs, got %d err=%v", logTotal, err)
	}
}
