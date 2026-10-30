package postgres

import (
	"context"
	"sort"
	"sync"

	"github.com/sigo-dental/billing-stock/internal/billing/application"
	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

type InMemoryBillingRepository struct {
	mu            sync.RWMutex
	cashRegisters map[string]*domain.CashRegister
	invoices      map[string]*domain.Invoice
	auditLogs     []application.AuditLogEntry
}

func NewInMemoryBillingRepository() *InMemoryBillingRepository {
	return &InMemoryBillingRepository{
		cashRegisters: make(map[string]*domain.CashRegister),
		invoices:      make(map[string]*domain.Invoice),
		auditLogs:     make([]application.AuditLogEntry, 0),
	}
}

func cloneCashRegister(src *domain.CashRegister) *domain.CashRegister {
	if src == nil {
		return nil
	}
	cp := *src
	if src.Movements != nil {
		cp.Movements = make([]domain.CashMovement, len(src.Movements))
		copy(cp.Movements, src.Movements)
	}
	if src.Denominations != nil {
		cp.Denominations = make(map[string]int, len(src.Denominations))
		for k, v := range src.Denominations {
			cp.Denominations[k] = v
		}
	}
	if src.DeclaredCashCents != nil {
		v := *src.DeclaredCashCents
		cp.DeclaredCashCents = &v
	}
	if src.TheoreticalCashCents != nil {
		v := *src.TheoreticalCashCents
		cp.TheoreticalCashCents = &v
	}
	if src.DiscrepancyCents != nil {
		v := *src.DiscrepancyCents
		cp.DiscrepancyCents = &v
	}
	if src.ClosedAt != nil {
		v := *src.ClosedAt
		cp.ClosedAt = &v
	}
	return &cp
}

func (r *InMemoryBillingRepository) SaveCashRegister(_ context.Context, reg *domain.CashRegister) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cashRegisters[reg.ID] = cloneCashRegister(reg)
	return nil
}

func (r *InMemoryBillingRepository) GetCashRegisterByID(_ context.Context, id string) (*domain.CashRegister, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.cashRegisters[id]
	if !ok {
		return nil, domain.ErrCashRegisterNotFound
	}
	return cloneCashRegister(reg), nil
}

func (r *InMemoryBillingRepository) UpdateCashRegister(_ context.Context, reg *domain.CashRegister) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.cashRegisters[reg.ID]; !ok {
		return domain.ErrCashRegisterNotFound
	}
	r.cashRegisters[reg.ID] = cloneCashRegister(reg)
	return nil
}

func (r *InMemoryBillingRepository) SaveInvoiceAndCashMovement(_ context.Context, inv *domain.Invoice, reg *domain.CashRegister) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.cashRegisters[reg.ID]; !ok {
		return domain.ErrCashRegisterNotFound
	}
	invCopy := *inv
	invCopy.Items = append([]domain.InvoiceItem(nil), inv.Items...)
	invCopy.Payments = append([]domain.InvoicePayment(nil), inv.Payments...)
	r.invoices[inv.ID] = &invCopy
	r.cashRegisters[reg.ID] = cloneCashRegister(reg)
	return nil
}

func (r *InMemoryBillingRepository) AppendAuditLog(_ context.Context, entry application.AuditLogEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditLogs = append(r.auditLogs, entry)
	return nil
}

func cloneInvoice(src *domain.Invoice) *domain.Invoice {
	if src == nil {
		return nil
	}
	cp := *src
	cp.Items = append([]domain.InvoiceItem(nil), src.Items...)
	cp.Payments = append([]domain.InvoicePayment(nil), src.Payments...)
	return &cp
}

// ListInvoices returns the invoices matching the filter ordered by creation date
// descending with the ID as tiebreaker, so pagination neither duplicates nor skips rows.
func (r *InMemoryBillingRepository) ListInvoices(_ context.Context, filter application.InvoiceFilter) ([]domain.Invoice, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]domain.Invoice, 0, len(r.invoices))
	for _, inv := range r.invoices {
		if filter.PatientID != "" && inv.PatientID != filter.PatientID {
			continue
		}
		if filter.FiscalStatus != "" && inv.FiscalStatus() != filter.FiscalStatus {
			continue
		}
		if filter.From != nil && inv.CreatedAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && inv.CreatedAt.After(*filter.To) {
			continue
		}
		matched = append(matched, *cloneInvoice(inv))
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID < matched[j].ID
	})

	total := len(matched)
	limit, offset := application.NormalizePage(filter.Limit, filter.Offset)
	return page(matched, limit, offset), total, nil
}

// ListCashRegisters returns the cash registers matching the filter ordered by opening
// date descending with the ID as tiebreaker.
func (r *InMemoryBillingRepository) ListCashRegisters(_ context.Context, filter application.CashRegisterFilter) ([]domain.CashRegister, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]domain.CashRegister, 0, len(r.cashRegisters))
	for _, reg := range r.cashRegisters {
		if filter.ClinicID != "" && reg.ClinicID != filter.ClinicID {
			continue
		}
		if filter.Status != "" && reg.Status != filter.Status {
			continue
		}
		matched = append(matched, *cloneCashRegister(reg))
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].OpenedAt.Equal(matched[j].OpenedAt) {
			return matched[i].OpenedAt.After(matched[j].OpenedAt)
		}
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
