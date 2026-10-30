package application

import (
	"context"
	"strings"
	"time"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type SOEPService struct {
	repo       SOEPRepository
	audit      auditapp.Recorder
	clock      Clock
	lockWindow time.Duration
}

func NewSOEPService(repo SOEPRepository, audit auditapp.Recorder, clock Clock, lockWindow time.Duration) *SOEPService {
	if clock == nil {
		clock = stdClock{}
	}
	if lockWindow <= 0 {
		lockWindow = domain.DefaultSOEPLockWindow
	}
	return &SOEPService{
		repo:       repo,
		audit:      audit,
		clock:      clock,
		lockWindow: lockWindow,
	}
}

func (s *SOEPService) CreateNote(ctx context.Context, actor ActorMeta, patientID string, input CreateSOEPNoteInput) (*domain.SOEPNote, error) {
	if err := requireDentistOnly(actor); err != nil {
		return nil, err
	}
	note, err := domain.NewSOEPNote(
		newID(),
		patientID,
		input.AppointmentID,
		input.ProcedureCode,
		actor.ActorID,
		input.Subjective,
		input.Objective,
		input.Assessment,
		input.Plan,
		s.lockWindow,
		s.clock.Now(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveNote(ctx, note); err != nil {
		return nil, err
	}
	s.auditEvent(ctx, actor, auditdom.ActionCreate, "SOEP_NOTE", note.ID, map[string]string{
		"patient_id":     note.PatientID,
		"signature_hash": note.SignatureHash,
	})
	return note, nil
}

func (s *SOEPService) UpdateNote(ctx context.Context, actor ActorMeta, patientID, noteID string, input UpdateSOEPNoteInput) (*domain.SOEPNote, error) {
	if err := requireDentistOnly(actor); err != nil {
		return nil, err
	}
	note, err := s.repo.GetNoteByID(ctx, patientID, noteID)
	if err != nil {
		return nil, err
	}
	if err := note.UpdateWithinWindow(actor.ActorID, input.Subjective, input.Objective, input.Assessment, input.Plan, s.clock.Now()); err != nil {
		return nil, err
	}
	if err := s.repo.SaveNote(ctx, note); err != nil {
		return nil, err
	}
	s.auditEvent(ctx, actor, auditdom.ActionUpdate, "SOEP_NOTE", note.ID, map[string]string{
		"signature_hash": note.SignatureHash,
	})
	return note, nil
}

func (s *SOEPService) DeleteNote(_ context.Context, actor ActorMeta, _, _ string) error {
	if err := requireDentistOnly(actor); err != nil {
		return err
	}
	return domain.ErrClinicalRecordImmutable
}

func (s *SOEPService) AddAddendum(ctx context.Context, actor ActorMeta, patientID, noteID string, input CreateSOEPAddendumInput) (*domain.SOEPAddendum, error) {
	if err := requireDentistOnly(actor); err != nil {
		return nil, err
	}
	note, err := s.repo.GetNoteByID(ctx, patientID, noteID)
	if err != nil {
		return nil, err
	}
	note.RefreshLockState(s.clock.Now())
	add, err := note.AppendAddendum(newID(), actor.ActorID, input.Reason, input.CorrectionText, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveNote(ctx, note); err != nil {
		return nil, err
	}
	s.auditEvent(ctx, actor, auditdom.ActionCreate, "SOEP_ADDENDUM", add.ID, map[string]string{
		"parent_note_id": note.ID,
		"signature_hash": add.SignatureHash,
	})
	return add, nil
}

func (s *SOEPService) ListNotes(ctx context.Context, actor ActorMeta, patientID string) ([]*domain.SOEPNote, error) {
	if !actor.MFAVerified {
		return nil, ErrClinicalMFARequired
	}
	notes, err := s.repo.ListNotesByPatient(ctx, strings.TrimSpace(patientID))
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	for _, n := range notes {
		n.RefreshLockState(now)
	}
	s.auditEvent(ctx, actor, auditdom.ActionRead, "SOEP_NOTE", patientID, nil)
	return notes, nil
}

func requireDentistOnly(actor ActorMeta) error {
	if !actor.MFAVerified {
		return ErrClinicalMFARequired
	}
	if strings.ToLower(strings.TrimSpace(actor.ActorRole)) != "dentist" {
		return ErrClinicalForbiddenRole
	}
	return nil
}

func (s *SOEPService) auditEvent(ctx context.Context, actor ActorMeta, action auditdom.AuditAction, resType, resID string, meta map[string]string) {
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
