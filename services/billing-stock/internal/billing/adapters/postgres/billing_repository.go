package postgres

import (
	"context"
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
