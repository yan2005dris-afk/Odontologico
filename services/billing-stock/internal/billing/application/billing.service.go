package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

var (
	ErrMFARequired   = errors.New("MFA_REQUIRED")
	ErrForbiddenRole = errors.New("FORBIDDEN_ROLE")
)

type OpenCashRegisterCommand struct {
	ID                string `json:"id"`
	ClinicID          string `json:"clinic_id"`
	OpeningFloatCents int64  `json:"opening_float_cents"`
}

type IssueInvoiceCommand struct {
	ID             string                       `json:"id"`
	PatientID      string                       `json:"patient_id"`
	CashRegisterID string                       `json:"cash_register_id"`
	CustomerTaxID  string                       `json:"customer_tax_id"`
	CustomerName   string                       `json:"customer_name"`
	Items          []domain.InvoiceItemInput    `json:"items"`
	Payments       []domain.InvoicePaymentInput `json:"payments"`
}

type BlindCloseCommand struct {
	CashRegisterID    string         `json:"cash_register_id"`
	DeclaredCashCents int64          `json:"declared_cash_cents"`
	Denominations     map[string]int `json:"denominations,omitempty"`
	CashierNotes      string         `json:"cashier_notes,omitempty"`
}

type BillingService struct {
	repo       BillingRepositoryPort
	taxClient  FiscalTaxAuthorityPort
	payGateway PaymentGatewayPort
}

func NewBillingService(repo BillingRepositoryPort, taxClient FiscalTaxAuthorityPort, payGateway PaymentGatewayPort) *BillingService {
	return &BillingService{
		repo:       repo,
		taxClient:  taxClient,
		payGateway: payGateway,
	}
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

func validateActor(actor ActorContext, allowedRoles ...string) error {
	if !actor.MFAVerified {
		return ErrMFARequired
	}
	if strings.TrimSpace(actor.ActorID) == "" {
		return ErrForbiddenRole
	}
	for _, r := range allowedRoles {
		if actor.ActorRole == r {
			return nil
		}
	}
	return ErrForbiddenRole
}

func (s *BillingService) OpenCashRegister(ctx context.Context, actor ActorContext, cmd OpenCashRegisterCommand) (*domain.PublicCashRegisterView, error) {
	if err := validateActor(actor, "receptionist", "admin"); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		id = newUUID()
	}
	reg, err := domain.OpenCashRegister(id, cmd.ClinicID, actor.ActorID, cmd.OpeningFloatCents, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveCashRegister(ctx, reg); err != nil {
		return nil, err
	}
	_ = s.repo.AppendAuditLog(ctx, AuditLogEntry{
		ID:          newUUID(),
		ActorID:     actor.ActorID,
		ActorRole:   actor.ActorRole,
		Action:      "CASH_REGISTER_OPENED",
		ResourceID:  reg.ID,
		Timestamp:   time.Now().UTC(),
		SourceIP:    actor.SourceIP,
		DetailsJSON: fmt.Sprintf(`{"opening_float_cents":%d}`, reg.OpeningFloatCents),
	})
	view := reg.ToReceptionistStatusView()
	return &view, nil
}

func (s *BillingService) IssueInvoice(ctx context.Context, actor ActorContext, cmd IssueInvoiceCommand) (*domain.Invoice, error) {
	if err := validateActor(actor, "receptionist", "admin"); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		id = newUUID()
	}

	reg, err := s.repo.GetCashRegisterByID(ctx, cmd.CashRegisterID)
	if err != nil {
		return nil, err
	}
	if reg.Status != domain.CashRegisterStatusOpen {
		return nil, domain.ErrCashRegisterAlreadyClosed
	}

	inv, err := domain.NewInvoice(domain.CreateInvoiceParams{
		ID:             id,
		PatientID:      cmd.PatientID,
		CashRegisterID: cmd.CashRegisterID,
		CustomerTaxID:  cmd.CustomerTaxID,
		CustomerName:   cmd.CustomerName,
		Items:          cmd.Items,
		Payments:       cmd.Payments,
		IssuedBy:       actor.ActorID,
		SourceIP:       actor.SourceIP,
		CreatedAt:      time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}

	// Authorize non-cash payments through payment gateway port
	for idx := range inv.Payments {
		p := &inv.Payments[idx]
		if p.Method != domain.PaymentMethodCash {
			authCode, err := s.payGateway.AuthorizePayment(ctx, p.Method, p.AmountCents, p.ExternalReference)
			if err != nil {
				return nil, err
			}
			p.GatewayAuthorization = authCode
		}
	}

	// Stamp electronic fiscal invoice
	fiscalAuth, err := s.taxClient.StampInvoice(ctx, inv)
	if err != nil {
		return nil, err
	}
	if err := inv.AuthorizeFiscal(fiscalAuth.AuthorizationCode, fiscalAuth.XMLHash, fiscalAuth.StampedAt); err != nil {
		return nil, err
	}

	// Record cash movement if invoice includes CASH payment(s)
	cashCents := inv.TotalCashPaymentCents()
	if cashCents > 0 {
		if err := reg.RecordCashMovement(
			newUUID(),
			inv.ID,
			domain.MovementDirectionIn,
			cashCents,
			fmt.Sprintf("Cash payment for invoice %s", inv.ID),
			actor.ActorID,
			inv.CreatedAt,
		); err != nil {
			return nil, err
		}
	}

	if err := s.repo.SaveInvoiceAndCashMovement(ctx, inv, reg); err != nil {
		return nil, err
	}

	_ = s.repo.AppendAuditLog(ctx, AuditLogEntry{
		ID:          newUUID(),
		ActorID:     actor.ActorID,
		ActorRole:   actor.ActorRole,
		Action:      "INVOICE_ISSUED",
		ResourceID:  inv.ID,
		Timestamp:   time.Now().UTC(),
		SourceIP:    actor.SourceIP,
		DetailsJSON: fmt.Sprintf(`{"total_cents":%d,"fiscal_code":"%s"}`, inv.TotalCents, inv.FiscalAuthorizationCode),
	})

	return inv, nil
}

func (s *BillingService) GetCashRegisterStatus(ctx context.Context, actor ActorContext, cashRegisterID string) (*domain.PublicCashRegisterView, error) {
	if err := validateActor(actor, "receptionist", "admin", "auditor"); err != nil {
		return nil, err
	}
	reg, err := s.repo.GetCashRegisterByID(ctx, cashRegisterID)
	if err != nil {
		return nil, err
	}
	view := reg.ToReceptionistStatusView()
	return &view, nil
}

func (s *BillingService) ExecuteBlindClose(ctx context.Context, actor ActorContext, cmd BlindCloseCommand) (*domain.BlindCloseReceiptView, error) {
	if err := validateActor(actor, "receptionist", "admin"); err != nil {
		return nil, err
	}
	reg, err := s.repo.GetCashRegisterByID(ctx, cmd.CashRegisterID)
	if err != nil {
		return nil, err
	}

	receipt, err := reg.ExecuteBlindClose(cmd.DeclaredCashCents, cmd.Denominations, cmd.CashierNotes, actor.ActorID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateCashRegister(ctx, reg); err != nil {
		return nil, err
	}

	_ = s.repo.AppendAuditLog(ctx, AuditLogEntry{
		ID:          newUUID(),
		ActorID:     actor.ActorID,
		ActorRole:   actor.ActorRole,
		Action:      "CASH_REGISTER_BLIND_CLOSED",
		ResourceID:  reg.ID,
		Timestamp:   time.Now().UTC(),
		SourceIP:    actor.SourceIP,
		DetailsJSON: fmt.Sprintf(`{"declared_cash_cents":%d,"receipt_hash":"%s"}`, cmd.DeclaredCashCents, receipt.ReceiptHash),
	})

	return receipt, nil
}

func (s *BillingService) GetAuditReport(ctx context.Context, actor ActorContext, cashRegisterID string) (*domain.CashRegisterAuditReportView, error) {
	if !actor.MFAVerified {
		return nil, ErrMFARequired
	}
	reg, err := s.repo.GetCashRegisterByID(ctx, cashRegisterID)
	if err != nil {
		return nil, err
	}
	// RN-02: If register is still OPEN, querying audit/theoretical cash is strictly forbidden for ALL roles (including receptionist)
	if reg.Status == domain.CashRegisterStatusOpen {
		return nil, domain.ErrBlindCashCloseEnforced
	}
	// S4: Once closed, only auditor and admin can view the reconciliation report with theoretical & discrepancy
	if err := validateActor(actor, "auditor", "admin"); err != nil {
		return nil, err
	}
	return reg.ToAuditReportView()
}
