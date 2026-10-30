package application

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

const (
	// DefaultListLimit is the page size used when the caller does not ask
	// for one.
	DefaultListLimit = 50
	// MaxListLimit is the hard ceiling of a listing page (RNF-07).
	MaxListLimit = 200
)

var (
	ErrInvalidConsentStatus = errors.New("status must be one of: PENDING, SIGNED_BIOMETRIC, SIGNED_OTP")
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
	s.recordAudit(ctx, actor, auditdom.ActionCreate, "INFORMED_CONSENT", consent.ID, nil)
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

	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "INFORMED_CONSENT", consent.ID, map[string]string{
		"document_sha256":    consent.DocumentSHA256,
		"storage_object_key": consent.StorageObjectKey,
		"signature_method":   consent.SignatureMethod,
	})
	return consent, nil
}

// ListConsents returns the informed consents matching the filter (S1). The
// listing leaves an immutable audit trail with action EXPORT (RNF-06).
func (s *ConsentService) ListConsents(ctx context.Context, actor ActorMeta, filter ConsentFilter) ([]*domain.PatientConsent, int, error) {
	if err := requireConsentReadRole(actor); err != nil {
		return nil, 0, err
	}
	normalized, err := normalizeConsentFilter(filter)
	if err != nil {
		return nil, 0, err
	}

	consents, total, err := s.repo.ListConsents(ctx, normalized)
	if err != nil {
		return nil, 0, err
	}

	s.recordAudit(ctx, actor, auditdom.ActionExport, "INFORMED_CONSENT", "consents", map[string]string{
		"total": strconv.Itoa(total),
	})
	return consents, total, nil
}

// ListConsentsByPatient returns the informed consents of a single patient
// (S1). It reuses the paginated repository read with the patient filter, so
// the response honours the same {items,total} contract as the global
// listing.
func (s *ConsentService) ListConsentsByPatient(ctx context.Context, actor ActorMeta, patientID string, limit, offset int) ([]*domain.PatientConsent, int, error) {
	if err := requireConsentReadRole(actor); err != nil {
		return nil, 0, err
	}
	filter, err := normalizeConsentFilter(ConsentFilter{
		PatientID: strings.TrimSpace(patientID),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, 0, err
	}

	consents, total, err := s.repo.ListConsents(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	s.recordAudit(ctx, actor, auditdom.ActionExport, "INFORMED_CONSENT", filter.PatientID, map[string]string{
		"total": strconv.Itoa(total),
	})
	return consents, total, nil
}

// normalizeConsentFilter maps the public vocabulary of the listing API
// (PENDING | SIGNED_BIOMETRIC | SIGNED_OTP) onto the domain statuses and
// applies the pagination contract: limit defaults to 50 with a hard ceiling
// of 200, offset defaults to 0.
func normalizeConsentFilter(filter ConsentFilter) (ConsentFilter, error) {
	filter.PatientID = strings.TrimSpace(filter.PatientID)
	filter.ProcedureCode = strings.TrimSpace(filter.ProcedureCode)

	switch strings.ToUpper(strings.TrimSpace(filter.Status)) {
	case "":
		filter.Status = ""
		filter.SignatureMethod = ""
	case "PENDING":
		filter.Status = "PENDING_SIGNATURE"
		filter.SignatureMethod = ""
	case "SIGNED_BIOMETRIC":
		filter.Status = "SIGNED"
		filter.SignatureMethod = "BIOMETRIC"
	case "SIGNED_OTP":
		filter.Status = "SIGNED"
		filter.SignatureMethod = "OTP"
	default:
		return filter, ErrInvalidConsentStatus
	}

	if filter.Limit <= 0 {
		filter.Limit = DefaultListLimit
	} else if filter.Limit > MaxListLimit {
		filter.Limit = MaxListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter, nil
}

// requireConsentReadRole restricts the consent listings to the roles allowed
// to read clinical documents (SRS §3). The auditor role has no access.
func requireConsentReadRole(actor ActorMeta) error {
	switch strings.ToLower(strings.TrimSpace(actor.ActorRole)) {
	case "admin", "dentist", "receptionist", "assistant":
		return nil
	default:
		return ErrClinicalReadForbiddenRole
	}
}

// recordAudit appends the immutable audit trail of the operation. A failing
// recorder must never invalidate the business response, but the failure is
// logged instead of being silently discarded (RNF-06).
func (s *ConsentService) recordAudit(ctx context.Context, actor ActorMeta, action auditdom.AuditAction, resType, resID string, meta map[string]string) {
	if s.audit == nil {
		return
	}
	if _, err := s.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      actor.ActorID,
		ActorRole:    actor.ActorRole,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		SourceIP:     actor.SourceIP,
		Metadata:     meta,
	}); err != nil {
		log.Printf("audit trail could not be recorded (action=%s resource=%s/%s): %v", action, resType, resID, err)
	}
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
