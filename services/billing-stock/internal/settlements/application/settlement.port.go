package application

import (
	"context"

	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

type ActorContext struct {
	ActorID     string
	ActorRole   string
	MFAVerified bool
	SourceIP    string
}

type SettlementRepositoryPort interface {
	UpsertCommissionRule(ctx context.Context, rule *domain.CommissionRule) error
	GetCommissionRule(ctx context.Context, dentistID, procedureCode string) (*domain.CommissionRule, error)
	IsProcedureSettled(ctx context.Context, completedProcedureID string) (bool, error)
	SaveSettlement(ctx context.Context, settlement *domain.DentistSettlement) error
}
