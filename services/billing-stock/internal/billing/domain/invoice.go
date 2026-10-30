package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	PaymentMethodCash            = "CASH"
	PaymentMethodCreditCard      = "CREDIT_CARD"
	PaymentMethodDebitCard       = "DEBIT_CARD"
	PaymentMethodBankTransfer    = "BANK_TRANSFER"
	PaymentMethodDentalInsurance = "DENTAL_INSURANCE"
)

var (
	ErrPaymentSumMismatch  = errors.New("PAYMENT_SUM_MISMATCH")
	ErrInvalidInvoiceInput = errors.New("invalid invoice input")
	ErrInvoiceNotFound     = errors.New("invoice not found")
)

type InvoiceItemInput struct {
	ProcedureCode  string `json:"procedure_code"`
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	TaxRateBPS     int    `json:"tax_rate_bps"`
}

type InvoiceItem struct {
	ID                string `json:"id"`
	ProcedureCode     string `json:"procedure_code"`
	Description       string `json:"description"`
	Quantity          int    `json:"quantity"`
	UnitPriceCents    int64  `json:"unit_price_cents"`
	TaxRateBPS        int    `json:"tax_rate_bps"`
	LineSubtotalCents int64  `json:"line_subtotal_cents"`
	LineTaxCents      int64  `json:"line_tax_cents"`
	LineTotalCents    int64  `json:"line_total_cents"`
}

type InvoicePaymentInput struct {
	Method            string `json:"method"`
	AmountCents       int64  `json:"amount_cents"`
	ExternalReference string `json:"external_reference"`
}

type InvoicePayment struct {
	ID                   string `json:"id"`
	Method               string `json:"method"`
	AmountCents          int64  `json:"amount_cents"`
	ExternalReference    string `json:"external_reference"`
	GatewayAuthorization string `json:"gateway_authorization,omitempty"`
}

type Invoice struct {
	ID                      string           `json:"id"`
	PatientID               string           `json:"patient_id"`
	CashRegisterID          string           `json:"cash_register_id"`
	CustomerTaxID           string           `json:"customer_tax_id"`
	CustomerName            string           `json:"customer_name"`
	SubtotalCents           int64            `json:"subtotal_cents"`
	TaxCents                int64            `json:"tax_cents"`
	TotalCents              int64            `json:"total_cents"`
	Items                   []InvoiceItem    `json:"items"`
	Payments                []InvoicePayment `json:"payments"`
	FiscalAuthorizationCode string           `json:"fiscal_authorization_code"`
	XMLHash                 string           `json:"xml_hash"`
	StampedAt               time.Time        `json:"stamped_at"`
	IssuedBy                string           `json:"issued_by"`
	SourceIP                string           `json:"source_ip"`
	CreatedAt               time.Time        `json:"created_at"`
}

type CreateInvoiceParams struct {
	ID             string
	PatientID      string
	CashRegisterID string
	CustomerTaxID  string
	CustomerName   string
	Items          []InvoiceItemInput
	Payments       []InvoicePaymentInput
	IssuedBy       string
	SourceIP       string
	CreatedAt      time.Time
}

func isValidPaymentMethod(m string) bool {
	switch m {
	case PaymentMethodCash, PaymentMethodCreditCard, PaymentMethodDebitCard, PaymentMethodBankTransfer, PaymentMethodDentalInsurance:
		return true
	default:
		return false
	}
}

func NewInvoice(p CreateInvoiceParams) (*Invoice, error) {
	if strings.TrimSpace(p.ID) == "" ||
		strings.TrimSpace(p.PatientID) == "" ||
		strings.TrimSpace(p.CashRegisterID) == "" ||
		strings.TrimSpace(p.CustomerTaxID) == "" ||
		strings.TrimSpace(p.CustomerName) == "" ||
		strings.TrimSpace(p.IssuedBy) == "" {
		return nil, ErrInvalidInvoiceInput
	}
	if len(p.Items) == 0 || len(p.Payments) == 0 {
		return nil, ErrInvalidInvoiceInput
	}

	var subtotalCents int64
	var taxCents int64
	items := make([]InvoiceItem, 0, len(p.Items))

	for idx, in := range p.Items {
		if strings.TrimSpace(in.ProcedureCode) == "" || strings.TrimSpace(in.Description) == "" {
			return nil, ErrInvalidInvoiceInput
		}
		if in.Quantity <= 0 || in.UnitPriceCents <= 0 || in.TaxRateBPS < 0 || in.TaxRateBPS > 10000 {
			return nil, ErrInvalidInvoiceInput
		}
		lineSub := int64(in.Quantity) * in.UnitPriceCents
		lineTax := (lineSub * int64(in.TaxRateBPS)) / 10000
		lineTotal := lineSub + lineTax

		subtotalCents += lineSub
		taxCents += lineTax

		items = append(items, InvoiceItem{
			ID:                fmt.Sprintf("%s-item-%d", p.ID, idx+1),
			ProcedureCode:     in.ProcedureCode,
			Description:       in.Description,
			Quantity:          in.Quantity,
			UnitPriceCents:    in.UnitPriceCents,
			TaxRateBPS:        in.TaxRateBPS,
			LineSubtotalCents: lineSub,
			LineTaxCents:      lineTax,
			LineTotalCents:    lineTotal,
		})
	}

	totalCents := subtotalCents + taxCents
	if totalCents <= 0 {
		return nil, ErrInvalidInvoiceInput
	}

	var paymentSumCents int64
	payments := make([]InvoicePayment, 0, len(p.Payments))
	for idx, pay := range p.Payments {
		if !isValidPaymentMethod(pay.Method) || pay.AmountCents <= 0 {
			return nil, ErrInvalidInvoiceInput
		}
		paymentSumCents += pay.AmountCents
		payments = append(payments, InvoicePayment{
			ID:                fmt.Sprintf("%s-pay-%d", p.ID, idx+1),
			Method:            pay.Method,
			AmountCents:       pay.AmountCents,
			ExternalReference: pay.ExternalReference,
		})
	}

	if paymentSumCents != totalCents {
		return nil, ErrPaymentSumMismatch
	}

	createdAt := p.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	sourceIP := strings.TrimSpace(p.SourceIP)
	if sourceIP == "" {
		sourceIP = "0.0.0.0"
	}

	return &Invoice{
		ID:             p.ID,
		PatientID:      p.PatientID,
		CashRegisterID: p.CashRegisterID,
		CustomerTaxID:  p.CustomerTaxID,
		CustomerName:   p.CustomerName,
		SubtotalCents:  subtotalCents,
		TaxCents:       taxCents,
		TotalCents:     totalCents,
		Items:          items,
		Payments:       payments,
		IssuedBy:       p.IssuedBy,
		SourceIP:       sourceIP,
		CreatedAt:      createdAt.UTC(),
	}, nil
}

func (inv *Invoice) AuthorizeFiscal(code, xmlHash string, stampedAt time.Time) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(xmlHash) == "" {
		return ErrInvalidInvoiceInput
	}
	if stampedAt.IsZero() {
		stampedAt = time.Now().UTC()
	}
	inv.FiscalAuthorizationCode = code
	inv.XMLHash = xmlHash
	inv.StampedAt = stampedAt.UTC()
	return nil
}

func (inv *Invoice) TotalCashPaymentCents() int64 {
	var cash int64
	for _, p := range inv.Payments {
		if p.Method == PaymentMethodCash {
			cash += p.AmountCents
		}
	}
	return cash
}
