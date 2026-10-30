package application

import (
	"context"
	"time"

	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

type ActorContext struct {
	ActorID     string
	ActorRole   string
	MFAVerified bool
	SourceIP    string
}

// SettlementFilter holds the query filters of the settlement listing.
type SettlementFilter struct {
	DentistID string
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

// CommissionRuleFilter holds the query filters of the commission rule listing.
type CommissionRuleFilter struct {
	DentistID     string
	ProcedureCode string
	Limit         int
	Offset        int
}

type SettlementRepositoryPort interface {
	UpsertCommissionRule(ctx context.Context, rule *domain.CommissionRule) error
	GetCommissionRule(ctx context.Context, dentistID, procedureCode string) (*domain.CommissionRule, error)
	IsProcedureSettled(ctx context.Context, completedProcedureID string) (bool, error)
	SaveSettlement(ctx context.Context, settlement *domain.DentistSettlement) error
	ListSettlements(ctx context.Context, filter SettlementFilter) ([]domain.DentistSettlement, int, error)
	ListCommissionRules(ctx context.Context, filter CommissionRuleFilter) ([]domain.CommissionRule, int, error)
}
