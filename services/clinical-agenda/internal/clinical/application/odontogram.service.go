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
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

var (
	ErrClinicalForbiddenRole = errors.New("actor role is not authorized to mutate clinical records")
	ErrClinicalMFARequired   = errors.New("MFA verification is required for clinical operations")
)

type stdClock struct{}

func (stdClock) Now() time.Time { return time.Now().UTC() }

type OdontogramService struct {
	repo  OdontogramRepository
	audit auditapp.Recorder
	clock Clock
}

func NewOdontogramService(repo OdontogramRepository, audit auditapp.Recorder, clock Clock) *OdontogramService {
	if clock == nil {
		clock = stdClock{}
	}
	return &OdontogramService{
		repo:  repo,
		audit: audit,
		clock: clock,
	}
}

func (s *OdontogramService) RecordFinding(ctx context.Context, actor ActorMeta, patientID string, input RecordFindingInput) (*domain.OdontogramFinding, error) {
	if err := requireClinicianWrite(actor); err != nil {
		return nil, err
	}
	finding, err := domain.NewOdontogramFinding(
		newID(),
		patientID,
		input.ToothCode,
		input.NotationSystem,
		input.Surface,
		input.FindingType,
		input.Status,
		input.Notes,
		actor.ActorID,
		s.clock.Now(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveFinding(ctx, finding); err != nil {
		return nil, err
	}
	s.recordAudit(ctx, actor, auditdom.ActionCreate, "ODONTOGRAM", finding.ID, map[string]string{
		"patient_id": finding.PatientID,
		"tooth_fdi":  finding.ToothFDI,
		"status":     finding.Status,
	})
	return finding, nil
}

func (s *OdontogramService) UpdateFinding(ctx context.Context, actor ActorMeta, patientID, findingID string, input UpdateFindingInput) (*domain.OdontogramFinding, error) {
	if err := requireClinicianWrite(actor); err != nil {
		return nil, err
	}
	finding, err := s.repo.GetFindingByID(ctx, patientID, findingID)
	if err != nil {
		return nil, err
	}
	if err := finding.UpdateStatus(input.Status, input.Notes); err != nil {
		return nil, err
	}
	if err := s.repo.SaveFinding(ctx, finding); err != nil {
		return nil, err
	}
	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "ODONTOGRAM", finding.ID, map[string]string{
		"patient_id": finding.PatientID,
		"status":     finding.Status,
	})
	return finding, nil
}

func (s *OdontogramService) DeleteFinding(ctx context.Context, actor ActorMeta, patientID, findingID string) error {
	if err := requireClinicianWrite(actor); err != nil {
		return err
	}
	finding, err := s.repo.GetFindingByID(ctx, patientID, findingID)
	if err != nil {
		return err
	}
	if err := finding.CanDelete(); err != nil {
		return err
	}
	if err := s.repo.DeleteFinding(ctx, patientID, findingID); err != nil {
		return err
	}
	s.recordAudit(ctx, actor, auditdom.ActionUpdate, "ODONTOGRAM", finding.ID, map[string]string{
		"deleted": "true",
	})
	return nil
}

func (s *OdontogramService) AddFindingAddendum(ctx context.Context, actor ActorMeta, patientID, findingID string, input FindingAddendumInput) (*domain.FindingAddendum, error) {
	if err := requireClinicianWrite(actor); err != nil {
		return nil, err
	}
	finding, err := s.repo.GetFindingByID(ctx, patientID, findingID)
	if err != nil {
		return nil, err
	}
	add, err := finding.AddAddendum(newID(), input.Note, actor.ActorID, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveFinding(ctx, finding); err != nil {
		return nil, err
	}
	s.recordAudit(ctx, actor, auditdom.ActionCreate, "ODONTOGRAM_ADDENDUM", add.ID, map[string]string{
		"finding_id": finding.ID,
	})
	return add, nil
}

func (s *OdontogramService) GetOdontogram(ctx context.Context, actor ActorMeta, patientID string) (*OdontogramSnapshotDTO, error) {
	if !actor.MFAVerified {
		return nil, ErrClinicalMFARequired
	}
	findings, err := s.repo.ListFindingsByPatient(ctx, strings.TrimSpace(patientID))
	if err != nil {
		return nil, err
	}
	s.recordAudit(ctx, actor, auditdom.ActionRead, "ODONTOGRAM", patientID, nil)
	return &OdontogramSnapshotDTO{
		PatientID:     patientID,
		DentitionMode: "MIXED",
		Findings:      findings,
	}, nil
}

func requireClinicianWrite(actor ActorMeta) error {
	if !actor.MFAVerified {
		return ErrClinicalMFARequired
	}
	role := strings.ToLower(strings.TrimSpace(actor.ActorRole))
	if role != "dentist" && role != "assistant" {
		return ErrClinicalForbiddenRole
	}
	return nil
}

func (s *OdontogramService) recordAudit(ctx context.Context, actor ActorMeta, action auditdom.AuditAction, resType, resID string, meta map[string]string) {
	if s.audit == nil {
		return
	}
	_, _ = s.audit.Record(ctx, auditapp.RecordInput{
		ActorID:      actor.ActorID,
		ActorRole:    actor.ActorRole,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		SourceIP:     actor.SourceIP,
		Metadata:     meta,
	})
}

func newID() string {
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
