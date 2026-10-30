package application

import (
	"context"
	"strings"

	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type PeriodontogramService struct {
	repo  PeriodontogramRepository
	audit auditapp.Recorder
	clock Clock
}

func NewPeriodontogramService(repo PeriodontogramRepository, audit auditapp.Recorder, clock Clock) *PeriodontogramService {
	if clock == nil {
		clock = stdClock{}
	}
	return &PeriodontogramService{
		repo:  repo,
		audit: audit,
		clock: clock,
	}
}

func (s *PeriodontogramService) RecordChart(ctx context.Context, actor ActorMeta, patientID string, input RecordPeriodontogramInput) (*domain.PeriodontogramChart, error) {
	if err := requireClinicianWrite(actor); err != nil {
		return nil, err
	}
	chart, err := domain.NewPeriodontogramChart(newID(), patientID, actor.ActorID, input.Teeth, s.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveChart(ctx, chart); err != nil {
		return nil, err
	}
	if s.audit != nil {
		_, _ = s.audit.Record(ctx, auditapp.RecordInput{
			ActorID:      actor.ActorID,
			ActorRole:    actor.ActorRole,
			Action:       auditdom.ActionCreate,
			ResourceType: "PERIODONTOGRAM",
			ResourceID:   chart.ID,
			SourceIP:     actor.SourceIP,
		})
	}
	return chart, nil
}

func (s *PeriodontogramService) ListCharts(ctx context.Context, actor ActorMeta, patientID string) ([]*domain.PeriodontogramChart, error) {
	if !actor.MFAVerified {
		return nil, ErrClinicalMFARequired
	}
	charts, err := s.repo.ListByPatient(ctx, strings.TrimSpace(patientID))
	if err != nil {
		return nil, err
	}
	if s.audit != nil {
		_, _ = s.audit.Record(ctx, auditapp.RecordInput{
			ActorID:      actor.ActorID,
			ActorRole:    actor.ActorRole,
			Action:       auditdom.ActionRead,
			ResourceType: "PERIODONTOGRAM",
			ResourceID:   patientID,
			SourceIP:     actor.SourceIP,
		})
	}
	return charts, nil
}
