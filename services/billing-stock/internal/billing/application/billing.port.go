package application

import (
	"context"
	"time"

	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

type ActorContext struct {
	ActorID     string
	ActorRole   string
	MFAVerified bool
	SourceIP    string
}

type FiscalAuthorization struct {
	AuthorizationCode string
	XMLHash           string
	StampedAt         time.Time
}

type FiscalTaxAuthorityPort interface {
	StampInvoice(ctx context.Context, invoice *domain.Invoice) (*FiscalAuthorization, error)
}

type PaymentGatewayPort interface {
	AuthorizePayment(ctx context.Context, method string, amountCents int64, externalRef string) (string, error)
}

type AuditLogEntry struct {
	ID          string
	ActorID     string
	ActorRole   string
	Action      string
	ResourceID  string
	Timestamp   time.Time
	SourceIP    string
	DetailsJSON string
}

type BillingRepositoryPort interface {
	SaveCashRegister(ctx context.Context, reg *domain.CashRegister) error
	GetCashRegisterByID(ctx context.Context, id string) (*domain.CashRegister, error)
	UpdateCashRegister(ctx context.Context, reg *domain.CashRegister) error
	SaveInvoiceAndCashMovement(ctx context.Context, inv *domain.Invoice, reg *domain.CashRegister) error
	AppendAuditLog(ctx context.Context, entry AuditLogEntry) error
}
