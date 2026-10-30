package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/patients/domain"
)

var (
	ErrForbiddenRole = errors.New("actor role is not authorized for this patient operation")
	ErrMFARequired   = errors.New("MFA verification is required for patient and anamnesis operations")
)

type stdClock struct{}

func (stdClock) Now() time.Time { return time.Now().UTC() }

type PatientService struct {
	repo  PatientRepository
	audit auditapp.Recorder
	clock Clock
}

func NewPatientService(repo PatientRepository, audit auditapp.Recorder, clock Clock) *PatientService {
	if clock == nil {
		clock = stdClock{}
	}
	return &PatientService{
		repo:  repo,
		audit: audit,
		clock: clock,
	}
}

func (s *PatientService) CreatePatient(ctx context.Context, actor ActorMeta, input UpsertPatientInput) (*domain.Patient, error) {
	if err := requirePatientWriteRole(actor); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	seq := s.repo.NextSequence(ctx)
	hcNumber := fmt.Sprintf("HC-%d-%06d", now.Year(), seq)

	patient, err := domain.NewPatient(domain.CreatePatientParams{
		ID:               genUUID(),
		HCNumber:         hcNumber,
		DocumentType:     input.DocumentType,
		DocumentNumber:   input.DocumentNumber,
		FirstName:        input.FirstName,
		LastName:         input.LastName,
		BirthDate:        input.BirthDate,
		Sex:              input.Sex,
		Phone:            input.Phone,
		Email:            input.Email,
		EmergencyContact: input.EmergencyContact,
		LegalGuardian:    input.LegalGuardian,
		InsurancePlan:    input.InsurancePlan,
	}, now)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, patient); err != nil {
		return nil, err
	}

	s.recordAudit(ctx, actor, auditdom.ActionCreate, "PATIENT", patient.ID, map[string]string{
		"hc_number": patient.HCNumber,
	})
	return patient, nil
}

func (s *PatientService) UpdatePatient(ctx context.Context, actor ActorMeta, patientID string, input UpsertPatientInput) (*domain.Patient, error) {
	if err := requirePatientWriteRole(actor); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByID(ctx, strings.TrimSpace(patientID))
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	updated, err := domain.NewPatient(domain.CreatePatientParams{
		ID:               existing.ID,
		HCNumber:         existing.HCNumber,
		DocumentType:     input.DocumentType,
		DocumentNumber:   input.DocumentNumber,
		FirstName:        input.FirstName,
		LastName:         input.LastName,
		BirthDate:        input.BirthDate,
		Sex:              input.Sex,
		Phone:            input.Phone,
		Email:            input.Email,
		EmergencyContact: input.EmergencyContact,
		LegalGuardian:    input.LegalGuardian,
		InsurancePlan:    input.InsurancePlan,
	}, now)
	if err != nil {
		return nil, err
	}
	updated.AnamnesisRecorded = existing.AnamnesisRecorded
	updated.CriticalAlerts = existing.CriticalAlerts
	updated.CreatedAt = existing.CreatedAt

	if err := s.repo.Update(ctx, updated); err != nil {
		return nil, err
	}

	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "PATIENT", updated.ID, nil)
	return updated, nil
}

func (s *PatientService) GetPatient(ctx context.Context, actor ActorMeta, patientID string) (*domain.Patient, error) {
	if err := requirePatientReadRole(actor); err != nil {
		return nil, err
	}
	patient, err := s.repo.GetByID(ctx, strings.TrimSpace(patientID))
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, actor, auditdom.ActionRead, "PATIENT", patient.ID, nil)
	return patient, nil
}

func (s *PatientService) GetPatientHeader(ctx context.Context, actor ActorMeta, patientID string) (*PatientHeaderDTO, error) {
	p, err := s.GetPatient(ctx, actor, patientID)
	if err != nil {
		return nil, err
	}
	return &PatientHeaderDTO{
		ID:                p.ID,
		HCNumber:          p.HCNumber,
		FullName:          p.FirstName + " " + p.LastName,
		DocumentType:      p.DocumentType,
		DocumentNumber:    p.DocumentNumber,
		Age:               p.Age,
		IsMinor:           p.IsMinor,
		AnamnesisRecorded: p.AnamnesisRecorded,
		CriticalAlerts:    p.CriticalAlerts,
	}, nil
}

func (s *PatientService) SearchPatients(ctx context.Context, actor ActorMeta, query string, limit, offset int) ([]*domain.Patient, int, error) {
	if err := requirePatientReadRole(actor); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 25
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.Search(ctx, strings.TrimSpace(query), limit, offset)
}

func (s *PatientService) UpsertAnamnesis(ctx context.Context, actor ActorMeta, patientID string, input UpsertAnamnesisInput) (*domain.Patient, error) {
	if !actor.MFAVerified {
		return nil, ErrMFARequired
	}
	role := strings.ToLower(strings.TrimSpace(actor.ActorRole))
	if role != "dentist" && role != "assistant" {
		return nil, ErrForbiddenRole
	}

	patient, err := s.repo.GetByID(ctx, strings.TrimSpace(patientID))
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	anamnesis, alerts, err := domain.NewAnamnesis(
		genUUID(),
		patient.ID,
		actor.ActorID,
		input.SystemicDiseases,
		input.DrugAllergies,
		input.SurgicalHistory,
		input.Coagulopathies,
		input.IsPregnant,
		input.Notes,
		now,
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.SaveAnamnesis(ctx, anamnesis, alerts); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(ctx, patient.ID)
	if err != nil {
		return nil, err
	}

	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "PATIENT_ANAMNESIS", patient.ID, map[string]string{
		"critical_alerts_count": fmt.Sprintf("%d", len(alerts)),
	})
	return updated, nil
}

func requirePatientWriteRole(actor ActorMeta) error {
	if !actor.MFAVerified {
		return ErrMFARequired
	}
	switch strings.ToLower(strings.TrimSpace(actor.ActorRole)) {
	case "admin", "receptionist", "dentist", "assistant":
		return nil
	default:
		return ErrForbiddenRole
	}
}

func requirePatientReadRole(actor ActorMeta) error {
	if !actor.MFAVerified {
		return ErrMFARequired
	}
	switch strings.ToLower(strings.TrimSpace(actor.ActorRole)) {
	case "admin", "receptionist", "dentist", "assistant", "auditor":
		return nil
	default:
		return ErrForbiddenRole
	}
}

func (s *PatientService) recordAudit(ctx context.Context, actor ActorMeta, action auditdom.AuditAction, resourceType, resourceID string, meta map[string]string) {
	if s.audit == nil {
		return
	}
	_, _ = s.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      actor.ActorID,
		ActorRole:    actor.ActorRole,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		SourceIP:     actor.SourceIP,
		Metadata:     meta,
	})
}

func genUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}
