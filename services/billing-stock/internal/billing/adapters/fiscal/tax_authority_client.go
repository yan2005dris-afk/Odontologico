package fiscal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/sigo-dental/billing-stock/internal/billing/application"
	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

type TaxAuthorityClient struct{}

func NewTaxAuthorityClient() *TaxAuthorityClient {
	return &TaxAuthorityClient{}
}

func (c *TaxAuthorityClient) StampInvoice(_ context.Context, invoice *domain.Invoice) (*application.FiscalAuthorization, error) {
	now := time.Now().UTC()
	payload := fmt.Sprintf("INV|%s|%s|%d|%s", invoice.ID, invoice.CustomerTaxID, invoice.TotalCents, now.Format(time.RFC3339Nano))
	sum := sha256.Sum256([]byte(payload))
	xmlHash := "sha256:" + hex.EncodeToString(sum[:])
	authCode := fmt.Sprintf("CAE-%s-%s", now.Format("20060102"), hex.EncodeToString(sum[:6]))
	return &application.FiscalAuthorization{
		AuthorizationCode: authCode,
		XMLHash:           xmlHash,
		StampedAt:         now,
	}, nil
}

type PaymentGatewayClient struct{}

func NewPaymentGatewayClient() *PaymentGatewayClient {
	return &PaymentGatewayClient{}
}

func (g *PaymentGatewayClient) AuthorizePayment(_ context.Context, method string, amountCents int64, externalRef string) (string, error) {
	payload := fmt.Sprintf("%s|%d|%s", method, amountCents, externalRef)
	sum := sha256.Sum256([]byte(payload))
	return "GW-AUTH-" + hex.EncodeToString(sum[:6]), nil
}
