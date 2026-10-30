package domain_test

import (
	"testing"
	"time"

	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

func TestNewInvoice_MixedPaymentValidation_RF12(t *testing.T) {
	now := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)

	items := []domain.InvoiceItemInput{
		{
			ProcedureCode:  "ENDO-MOLAR",
			Description:    "Endodoncia birradicular/multirradicular",
			Quantity:       1,
			UnitPriceCents: 150000,
			TaxRateBPS:     0,
		},
		{
			ProcedureCode:  "KIT-BLANQ",
			Description:    "Kit blanqueamiento ambulatorio",
			Quantity:       2,
			UnitPriceCents: 50000,
			TaxRateBPS:     1000, // 10% tax -> subtotal 100,000 + tax 10,000 = 110,000
		},
	}

	// Expected total: 150,000 + 110,000 = 260,000
	validPayments := []domain.InvoicePaymentInput{
		{Method: domain.PaymentMethodCash, AmountCents: 60000},
		{Method: domain.PaymentMethodCreditCard, AmountCents: 120000, ExternalReference: "POS-TX-991"},
		{Method: domain.PaymentMethodDentalInsurance, AmountCents: 80000, ExternalReference: "OSDE-AUTH-44"},
	}

	inv, err := domain.NewInvoice(domain.CreateInvoiceParams{
		ID:             "inv-001",
		PatientID:      "pat-001",
		CashRegisterID: "reg-001",
		CustomerTaxID:  "20-30405060-9",
		CustomerName:   "Juan Perez",
		Items:          items,
		Payments:       validPayments,
		IssuedBy:       "recep-01",
		SourceIP:       "192.168.1.10",
		CreatedAt:      now,
	})
	if err != nil {
		t.Fatalf("expected valid invoice with mixed payments, got error: %v", err)
	}

	if inv.SubtotalCents != 250000 {
		t.Fatalf("expected subtotal 250000, got %d", inv.SubtotalCents)
	}
	if inv.TaxCents != 10000 {
		t.Fatalf("expected tax 10000, got %d", inv.TaxCents)
	}
	if inv.TotalCents != 260000 {
		t.Fatalf("expected total 260000, got %d", inv.TotalCents)
	}
	if inv.TotalCashPaymentCents() != 60000 {
		t.Fatalf("expected cash total 60000, got %d", inv.TotalCashPaymentCents())
	}

	// Stamp fiscal authorization
	if err := inv.AuthorizeFiscal("CAE-20261030-0001", "sha256:xmlhash", now.Add(time.Second)); err != nil {
		t.Fatalf("unexpected error stamping invoice: %v", err)
	}
	if err := inv.AuthorizeFiscal("", "sha256:xmlhash", now); err == nil {
		t.Fatalf("expected error when fiscal authorization code is empty")
	}

	// Adversarial S1: Payment sum mismatch returns ErrPaymentSumMismatch
	mismatchPayments := []domain.InvoicePaymentInput{
		{Method: domain.PaymentMethodCash, AmountCents: 60000},
		{Method: domain.PaymentMethodCreditCard, AmountCents: 100000},
	}
	_, err = domain.NewInvoice(domain.CreateInvoiceParams{
		ID:             "inv-002",
		PatientID:      "pat-001",
		CashRegisterID: "reg-001",
		CustomerTaxID:  "20-30405060-9",
		CustomerName:   "Juan Perez",
		Items:          items,
		Payments:       mismatchPayments,
		IssuedBy:       "recep-01",
		SourceIP:       "192.168.1.10",
		CreatedAt:      now,
	})
	if err != domain.ErrPaymentSumMismatch {
		t.Fatalf("expected ErrPaymentSumMismatch, got %v", err)
	}

	// Adversarial validation cases
	invalidCases := []struct {
		name   string
		mutate func(p *domain.CreateInvoiceParams)
	}{
		{
			name:   "missing patient id",
			mutate: func(p *domain.CreateInvoiceParams) { p.PatientID = "" },
		},
		{
			name:   "empty items",
			mutate: func(p *domain.CreateInvoiceParams) { p.Items = nil },
		},
		{
			name:   "invalid quantity",
			mutate: func(p *domain.CreateInvoiceParams) { p.Items[0].Quantity = 0 },
		},
		{
			name:   "invalid tax rate",
			mutate: func(p *domain.CreateInvoiceParams) { p.Items[0].TaxRateBPS = 20000 },
		},
		{
			name:   "unsupported payment method",
			mutate: func(p *domain.CreateInvoiceParams) { p.Payments[0].Method = "CRYPTO" },
		},
		{
			name:   "non-positive payment amount",
			mutate: func(p *domain.CreateInvoiceParams) { p.Payments[0].AmountCents = 0 },
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			params := domain.CreateInvoiceParams{
				ID:             "inv-err",
				PatientID:      "pat-001",
				CashRegisterID: "reg-001",
				CustomerTaxID:  "20-30405060-9",
				CustomerName:   "Juan Perez",
				Items: []domain.InvoiceItemInput{
					{ProcedureCode: "CONS", Description: "Consulta", Quantity: 1, UnitPriceCents: 10000, TaxRateBPS: 0},
				},
				Payments: []domain.InvoicePaymentInput{
					{Method: domain.PaymentMethodCash, AmountCents: 10000},
				},
				IssuedBy:  "recep-01",
				SourceIP:  "127.0.0.1",
				CreatedAt: now,
			}
			tc.mutate(&params)
			if _, err := domain.NewInvoice(params); err == nil {
				t.Fatalf("expected error for case %s, got nil", tc.name)
			}
		})
	}
}
