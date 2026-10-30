package application

import (
	"context"
	"strings"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type ConsentService struct {
	repo  ConsentRepository
	blobs BlobStoragePort
	audit auditapp.Recorder
	clock Clock
}

func NewConsentService(repo ConsentRepository, blobs BlobStoragePort, audit auditapp.Recorder, clock Clock) *ConsentService {
	if clock == nil {
		clock = stdClock{}
	}
	return &ConsentService{
		repo:  repo,
		blobs: blobs,
		audit: audit,
		clock: clock,
	}
}

func (s *ConsentService) CreateConsent(ctx context.Context, actor ActorMeta, patientID string, input CreateConsentInput) (*domain.PatientConsent, error) {
	if err := requireConsentStaff(actor); err != nil {
		return nil, err
	}
	consent, err := domain.NewPatientConsent(
		newID(),
		patientID,
		input.ProcedureCode,
		input.Title,
		input.RenderedText,
		input.ExpectedOTP,
		s.clock.Now(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveConsent(ctx, consent); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_, _ = s.audit.Record(ctx, auditapp.RecordInput{
			ActorID:      actor.ActorID,
			ActorRole:    actor.ActorRole,
			Action:       auditdom.ActionCreate,
			ResourceType: "INFORMED_CONSENT",
			ResourceID:   consent.ID,
			SourceIP:     actor.SourceIP,
		})
	}
	return consent, nil
}

func (s *ConsentService) SignConsent(ctx context.Context, actor ActorMeta, patientID, consentID string, input SignConsentInput) (*domain.PatientConsent, error) {
	if err := requireConsentStaff(actor); err != nil {
		return nil, err
	}
	consent, err := s.repo.GetConsentByID(ctx, patientID, consentID)
	if err != nil {
		return nil, err
	}
	pdfBytes, err := consent.Sign(domain.SignConsentParams{
		Method:              input.SignatureMethod,
		BiometricSVGStrokes: input.BiometricSVGStrokes,
		OTPCode:             input.OTPCode,
		SignedByName:        input.SignedByName,
		SignedByDocument:    input.SignedByDocument,
		DeviceMetadata:      input.DeviceMetadata,
	}, s.clock.Now())
	if err != nil {
		return nil, err
	}

	if s.blobs != nil {
		if err := s.blobs.PutEncryptedPDF(ctx, consent.StorageObjectKey, pdfBytes, consent.DocumentSHA256); err != nil {
			return nil, err
		}
	}
	if err := s.repo.SaveConsent(ctx, consent); err != nil {
		return nil, err
	}

	if s.audit != nil {
		_, _ = s.audit.Record(ctx, auditapp.RecordInput{
			ActorID:      actor.ActorID,
			ActorRole:    actor.ActorRole,
			Action:       auditdom.ActionUpdate,
			ResourceType: "INFORMED_CONSENT",
			ResourceID:   consent.ID,
			SourceIP:     actor.SourceIP,
			Metadata: map[string]string{
				"document_sha256":    consent.DocumentSHA256,
				"storage_object_key": consent.StorageObjectKey,
				"signature_method":   consent.SignatureMethod,
			},
		})
	}
	return consent, nil
}

func requireConsentStaff(actor ActorMeta) error {
	if !actor.MFAVerified {
		return ErrClinicalMFARequired
	}
	switch strings.ToLower(strings.TrimSpace(actor.ActorRole)) {
	case "dentist", "assistant", "receptionist":
		return nil
	default:
		return ErrClinicalForbiddenRole
	}
}
