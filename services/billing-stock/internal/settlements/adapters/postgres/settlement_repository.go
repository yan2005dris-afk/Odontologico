package postgres

import (
	"context"
	"fmt"
	"sync"

	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

type InMemorySettlementRepository struct {
	mu                sync.RWMutex
	rules             map[string]*domain.CommissionRule
	settlements       map[string]*domain.DentistSettlement
	settledProcedures map[string]string
}

func NewInMemorySettlementRepository() *InMemorySettlementRepository {
	return &InMemorySettlementRepository{
		rules:             make(map[string]*domain.CommissionRule),
		settlements:       make(map[string]*domain.DentistSettlement),
		settledProcedures: make(map[string]string),
	}
}

func ruleKey(dentistID, procCode string) string {
	return fmt.Sprintf("%s::%s", dentistID, procCode)
}

func (r *InMemorySettlementRepository) UpsertCommissionRule(_ context.Context, rule *domain.CommissionRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *rule
	r.rules[ruleKey(rule.DentistID, rule.ProcedureCode)] = &cp
	return nil
}

func (r *InMemorySettlementRepository) GetCommissionRule(_ context.Context, dentistID, procedureCode string) (*domain.CommissionRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if rule, ok := r.rules[ruleKey(dentistID, procedureCode)]; ok {
		cp := *rule
		return &cp, nil
	}
	if wildcard, ok := r.rules[ruleKey(dentistID, "*")]; ok {
		cp := *wildcard
		return &cp, nil
	}
	return nil, domain.ErrCommissionRuleNotFound
}

func (r *InMemorySettlementRepository) IsProcedureSettled(_ context.Context, completedProcedureID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.settledProcedures[completedProcedureID]
	return ok, nil
}

func (r *InMemorySettlementRepository) SaveSettlement(_ context.Context, settlement *domain.DentistSettlement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, it := range settlement.Items {
		if _, exists := r.settledProcedures[it.CompletedProcedureID]; exists {
			return domain.ErrProcedureAlreadySettled
		}
	}
	for _, it := range settlement.Items {
		r.settledProcedures[it.CompletedProcedureID] = settlement.SettlementID
	}
	cp := *settlement
	cp.Items = append([]domain.SettlementItem(nil), settlement.Items...)
	r.settlements[settlement.SettlementID] = &cp
	return nil
}
