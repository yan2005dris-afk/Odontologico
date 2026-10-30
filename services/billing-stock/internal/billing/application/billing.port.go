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

// InvoiceFilter holds the query filters of the invoice listing.
type InvoiceFilter struct {
	PatientID    string
	FiscalStatus string
	From         *time.Time
	To           *time.Time
	Limit        int
	Offset       int
}

// CashRegisterFilter holds the query filters of the cash register listing.
type CashRegisterFilter struct {
	ClinicID string
	Status   string
	Limit    int
	Offset   int
}

type BillingRepositoryPort interface {
	SaveCashRegister(ctx context.Context, reg *domain.CashRegister) error
	GetCashRegisterByID(ctx context.Context, id string) (*domain.CashRegister, error)
	UpdateCashRegister(ctx context.Context, reg *domain.CashRegister) error
	SaveInvoiceAndCashMovement(ctx context.Context, inv *domain.Invoice, reg *domain.CashRegister) error
	AppendAuditLog(ctx context.Context, entry AuditLogEntry) error
	ListInvoices(ctx context.Context, filter InvoiceFilter) ([]domain.Invoice, int, error)
	ListCashRegisters(ctx context.Context, filter CashRegisterFilter) ([]domain.CashRegister, int, error)
}
