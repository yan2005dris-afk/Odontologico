package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sigo-dental/billing-stock/internal/billing/adapters/fiscal"
	"github.com/sigo-dental/billing-stock/internal/billing/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/billing/application"
	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

func TestBillingService_POSAndBlindCashClose_S1_S2_S3_S4(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewInMemoryBillingRepository()
	taxClient := fiscal.NewTaxAuthorityClient()
	gateway := fiscal.NewPaymentGatewayClient()
	svc := application.NewBillingService(repo, taxClient, gateway, nil)

	recepActor := application.ActorContext{
		ActorID:     "recep-01",
		ActorRole:   "receptionist",
		MFAVerified: true,
		SourceIP:    "10.0.0.15",
	}
	auditorActor := application.ActorContext{
		ActorID:     "auditor-01",
		ActorRole:   "auditor",
		MFAVerified: true,
		SourceIP:    "10.0.0.20",
	}

	// Open cash register with 50,000 opening float
	regView, err := svc.OpenCashRegister(ctx, recepActor, application.OpenCashRegisterCommand{
		ID:                "ca5b0001-1111-4222-8333-444455556666",
		ClinicID:          "sede-central-uuid",
		OpeningFloatCents: 50000,
	})
	if err != nil {
		t.Fatalf("unexpected error opening cash register: %v", err)
	}
	if regView.Status != domain.CashRegisterStatusOpen {
		t.Fatalf("expected OPEN status, got %s", regView.Status)
	}

	// S1: Issue mixed-payment invoice (CASH 60,000 + CREDIT_CARD 120,000 + DENTAL_INSURANCE 80,000 = 260,000)
	inv, err := svc.IssueInvoice(ctx, recepActor, application.IssueInvoiceCommand{
		PatientID:      "patient-uuid-01",
		CashRegisterID: regView.ID,
		CustomerTaxID:  "20-30405060-9",
		CustomerName:   "Maria Gomez",
		Items: []domain.InvoiceItemInput{
			{
				ProcedureCode:  "ENDO-MOLAR",
				Description:    "Endodoncia multirradicular",
				Quantity:       1,
				UnitPriceCents: 150000,
				TaxRateBPS:     0,
			},
			{
				ProcedureCode:  "BLANQ-AMB",
				Description:    "Blanqueamiento ambulatorio",
				Quantity:       2,
				UnitPriceCents: 50000,
				TaxRateBPS:     1000, // 10% tax -> 110,000
			},
		},
		Payments: []domain.InvoicePaymentInput{
			{Method: domain.PaymentMethodCash, AmountCents: 60000},
			{Method: domain.PaymentMethodCreditCard, AmountCents: 120000, ExternalReference: "POS-9001"},
			{Method: domain.PaymentMethodDentalInsurance, AmountCents: 80000, ExternalReference: "OSDE-771"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error issuing mixed payment invoice: %v", err)
	}
	if inv.FiscalAuthorizationCode == "" || inv.XMLHash == "" {
		t.Fatalf("expected fiscal authorization and xml hash, got %+v", inv)
	}
	if inv.TotalCents != 260000 {
		t.Fatalf("expected total 260000, got %d", inv.TotalCents)
	}

	// S1 Adversarial: Payment sum mismatch returns ErrPaymentSumMismatch
	_, err = svc.IssueInvoice(ctx, recepActor, application.IssueInvoiceCommand{
		PatientID:      "patient-uuid-01",
		CashRegisterID: regView.ID,
		CustomerTaxID:  "20-30405060-9",
		CustomerName:   "Maria Gomez",
		Items: []domain.InvoiceItemInput{
			{ProcedureCode: "CONS", Description: "Consulta", Quantity: 1, UnitPriceCents: 10000, TaxRateBPS: 0},
		},
		Payments: []domain.InvoicePaymentInput{
			{Method: domain.PaymentMethodCash, AmountCents: 5000},
		},
	})
	if !errors.Is(err, domain.ErrPaymentSumMismatch) {
		t.Fatalf("expected ErrPaymentSumMismatch, got %v", err)
	}

	// S2: GetCashRegisterStatus for receptionist NEVER exposes theoretical_cash_cents (50,000 + 60,000 = 110,000)
	statusView, err := svc.GetCashRegisterStatus(ctx, recepActor, regView.ID)
	if err != nil {
		t.Fatalf("unexpected error getting status: %v", err)
	}
	rawStatus, _ := json.Marshal(statusView)
	if strings.Contains(string(rawStatus), "110000") || strings.Contains(string(rawStatus), "theoretical") {
		t.Fatalf("RN-02 violation in GetCashRegisterStatus: %s", string(rawStatus))
	}

	// S2 Adversarial: Receptionist or Auditor calling GetAuditReport on OPEN cash register is rejected with ErrBlindCashCloseEnforced
	if _, err := svc.GetAuditReport(ctx, recepActor, regView.ID); !errors.Is(err, domain.ErrBlindCashCloseEnforced) {
		t.Fatalf("expected ErrBlindCashCloseEnforced for receptionist on open register, got %v", err)
	}
	if _, err := svc.GetAuditReport(ctx, auditorActor, regView.ID); !errors.Is(err, domain.ErrBlindCashCloseEnforced) {
		t.Fatalf("expected ErrBlindCashCloseEnforced for auditor on open register, got %v", err)
	}

	// S3: Execute Blind Cash Close (declared = 108,500; theoretical = 110,000; discrepancy = -1,500)
	receipt, err := svc.ExecuteBlindClose(ctx, recepActor, application.BlindCloseCommand{
		CashRegisterID:    regView.ID,
		DeclaredCashCents: 108500,
		Denominations:     map[string]int{"1000": 108, "500": 1},
		CashierNotes:      "Arqueo de turno tarde",
	})
	if err != nil {
		t.Fatalf("unexpected error on blind close: %v", err)
	}
	if receipt.Status != domain.CashRegisterStatusClosed {
		t.Fatalf("expected CLOSED status, got %s", receipt.Status)
	}

	// S3 Adversarial: Re-closing returns ErrCashRegisterAlreadyClosed
	if _, err := svc.ExecuteBlindClose(ctx, recepActor, application.BlindCloseCommand{
		CashRegisterID:    regView.ID,
		DeclaredCashCents: 110000,
	}); !errors.Is(err, domain.ErrCashRegisterAlreadyClosed) {
		t.Fatalf("expected ErrCashRegisterAlreadyClosed on second close, got %v", err)
	}

	// S3 Adversarial: Issuing a CASH invoice against a CLOSED cash register returns ErrCashRegisterAlreadyClosed
	_, err = svc.IssueInvoice(ctx, recepActor, application.IssueInvoiceCommand{
		PatientID:      "patient-uuid-02",
		CashRegisterID: regView.ID,
		CustomerTaxID:  "20-11223344-5",
		CustomerName:   "Carlos Ruiz",
		Items: []domain.InvoiceItemInput{
			{ProcedureCode: "CONS", Description: "Consulta", Quantity: 1, UnitPriceCents: 20000, TaxRateBPS: 0},
		},
		Payments: []domain.InvoicePaymentInput{
			{Method: domain.PaymentMethodCash, AmountCents: 20000},
		},
	})
	if !errors.Is(err, domain.ErrCashRegisterAlreadyClosed) {
		t.Fatalf("expected ErrCashRegisterAlreadyClosed when invoicing cash on closed register, got %v", err)
	}

	// S4: Receptionist is still forbidden from GetAuditReport after close, while Auditor/Admin succeeds
	if _, err := svc.GetAuditReport(ctx, recepActor, regView.ID); !errors.Is(err, application.ErrForbiddenRole) {
		t.Fatalf("expected ErrForbiddenRole for receptionist accessing audit report, got %v", err)
	}
	auditReport, err := svc.GetAuditReport(ctx, auditorActor, regView.ID)
	if err != nil {
		t.Fatalf("unexpected error for auditor getting audit report: %v", err)
	}
	if auditReport.TheoreticalCashCents != 110000 || auditReport.DeclaredCashCents != 108500 || auditReport.DiscrepancyCents != -1500 {
		t.Fatalf("unexpected audit report values: %+v", auditReport)
	}

	// Adversarial RBAC & MFA checks across methods
	noMFAActor := application.ActorContext{ActorID: "recep-01", ActorRole: "receptionist", MFAVerified: false}
	if _, err := svc.OpenCashRegister(ctx, noMFAActor, application.OpenCashRegisterCommand{ClinicID: "c1", OpeningFloatCents: 0}); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	if _, err := svc.IssueInvoice(ctx, noMFAActor, application.IssueInvoiceCommand{}); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	if _, err := svc.GetCashRegisterStatus(ctx, noMFAActor, regView.ID); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	if _, err := svc.ExecuteBlindClose(ctx, noMFAActor, application.BlindCloseCommand{CashRegisterID: regView.ID}); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}
	if _, err := svc.GetAuditReport(ctx, noMFAActor, regView.ID); !errors.Is(err, application.ErrMFARequired) {
		t.Fatalf("expected ErrMFARequired, got %v", err)
	}

	// Auto-generated IDs when omitted
	regView2, err := svc.OpenCashRegister(ctx, recepActor, application.OpenCashRegisterCommand{
		ClinicID:          "sede-norte-uuid",
		OpeningFloatCents: 10000,
	})
	if err != nil || regView2.ID == "" {
		t.Fatalf("expected auto-generated cash register ID, got err=%v", err)
	}
}
