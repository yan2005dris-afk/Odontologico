package postgres

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/sigo-dental/billing-stock/internal/settlements/application"
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

// ListSettlements returns the settlements matching the filter ordered by creation date
// descending with the ID as tiebreaker, so pagination neither duplicates nor skips rows.
func (r *InMemorySettlementRepository) ListSettlements(_ context.Context, filter application.SettlementFilter) ([]domain.DentistSettlement, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]domain.DentistSettlement, 0, len(r.settlements))
	for _, settlement := range r.settlements {
		if filter.DentistID != "" && settlement.DentistID != filter.DentistID {
			continue
		}
		if filter.From != nil && settlement.CreatedAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && settlement.CreatedAt.After(*filter.To) {
			continue
		}
		cp := *settlement
		cp.Items = append([]domain.SettlementItem(nil), settlement.Items...)
		matched = append(matched, cp)
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].SettlementID < matched[j].SettlementID
	})

	total := len(matched)
	limit, offset := application.NormalizePage(filter.Limit, filter.Offset)
	return page(matched, limit, offset), total, nil
}

// ListCommissionRules returns the commission rules matching the filter. The rule model
// has no creation timestamp, so the deterministic order falls back to the rule ID.
func (r *InMemorySettlementRepository) ListCommissionRules(_ context.Context, filter application.CommissionRuleFilter) ([]domain.CommissionRule, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]domain.CommissionRule, 0, len(r.rules))
	for _, rule := range r.rules {
		if filter.DentistID != "" && rule.DentistID != filter.DentistID {
			continue
		}
		if filter.ProcedureCode != "" && rule.ProcedureCode != filter.ProcedureCode {
			continue
		}
		matched = append(matched, *rule)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ID < matched[j].ID
	})

	total := len(matched)
	limit, offset := application.NormalizePage(filter.Limit, filter.Offset)
	return page(matched, limit, offset), total, nil
}

// page returns the requested window of an already ordered slice.
func page[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
