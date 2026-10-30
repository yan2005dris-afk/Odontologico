package application

import (
	"context"

	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type PeriodontogramRepository interface {
	SaveChart(ctx context.Context, chart *domain.PeriodontogramChart) error
	ListByPatient(ctx context.Context, patientID string) ([]*domain.PeriodontogramChart, error)
}

type RecordPeriodontogramInput struct {
	Teeth []domain.PeriodontogramTooth `json:"teeth"`
}
