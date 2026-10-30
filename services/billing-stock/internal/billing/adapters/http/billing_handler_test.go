package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sigo-dental/billing-stock/internal/billing/adapters/fiscal"
	billinghttp "github.com/sigo-dental/billing-stock/internal/billing/adapters/http"
	"github.com/sigo-dental/billing-stock/internal/billing/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/billing/application"
)

func newTestBillingServer() *http.ServeMux {
	repo := postgres.NewInMemoryBillingRepository()
	taxClient := fiscal.NewTaxAuthorityClient()
	gateway := fiscal.NewPaymentGatewayClient()
	svc := application.NewBillingService(repo, taxClient, gateway, nil)
	handler := billinghttp.NewBillingHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func doRequest(mux *http.ServeMux, method, path string, body any, actorID, role string, mfa bool) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if actorID != "" {
		req.Header.Set("X-Actor-ID", actorID)
	}
	if role != "" {
		req.Header.Set("X-Actor-Role", role)
	}
	if mfa {
		req.Header.Set("X-MFA-Verified", "true")
	}
	req.Header.Set("X-Source-IP", "192.168.10.25")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestBillingHandler_EndToEndSpecs_S1_S2_S3_S4(t *testing.T) {
	mux := newTestBillingServer()

	// MFA check
	rec := doRequest(mux, http.MethodPost, "/api/v1/billing/cash-registers", map[string]any{
		"id":                  "ca5b0001-1111-4222-8333-444455556666",
		"clinic_id":           "sede-central-uuid",
		"opening_float_cents": 50000,
	}, "recepcionista-01-uuid", "receptionist", false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when MFA missing, got %d", rec.Code)
	}

	// Open cash register (201 Created)
	rec = doRequest(mux, http.MethodPost, "/api/v1/billing/cash-registers", map[string]any{
		"id":                  "ca5b0001-1111-4222-8333-444455556666",
		"clinic_id":           "sede-central-uuid",
		"opening_float_cents": 50000,
	}, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created opening cash register, got %d: %s", rec.Code, rec.Body.String())
	}

	// S1: Issue mixed-payment invoice (201 Created)
	invoicePayload := map[string]any{
		"patient_id":       "pat-101",
		"cash_register_id": "ca5b0001-1111-4222-8333-444455556666",
		"customer_tax_id":  "20-30405060-9",
		"customer_name":    "Ana Lopez",
		"items": []map[string]any{
			{
				"procedure_code":   "CORONA-ZIRC",
				"description":      "Corona de circonio",
				"quantity":         1,
				"unit_price_cents": 200000,
				"tax_rate_bps":     0,
			},
		},
		"payments": []map[string]any{
			{"method": "CASH", "amount_cents": 135000},
			{"method": "DEBIT_CARD", "amount_cents": 65000, "external_reference": "DEB-552"},
		},
	}
	rec = doRequest(mux, http.MethodPost, "/api/v1/billing/invoices", invoicePayload, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on mixed-payment invoice, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "fiscal_authorization_code") {
		t.Fatalf("expected fiscal_authorization_code in response, got %s", rec.Body.String())
	}

	// S1 Adversarial: Payment sum mismatch returns 422 PAYMENT_SUM_MISMATCH
	invoicePayload["payments"] = []map[string]any{
		{"method": "CASH", "amount_cents": 100000},
	}
	rec = doRequest(mux, http.MethodPost, "/api/v1/billing/invoices", invoicePayload, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "PAYMENT_SUM_MISMATCH") {
		t.Fatalf("expected 422 PAYMENT_SUM_MISMATCH, got %d: %s", rec.Code, rec.Body.String())
	}

	// S2: GET /api/v1/billing/cash-registers/{id} while OPEN never leaks theoretical_cash_cents (50000 + 135000 = 185000)
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers/ca5b0001-1111-4222-8333-444455556666", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK getting cash register, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "theoretical_cash_cents") || strings.Contains(rec.Body.String(), "185000") {
		t.Fatalf("RN-02 violation! Leaked theoretical cash in GET cash-register: %s", rec.Body.String())
	}

	// S2 Adversarial: Receptionist calling /audit-report on OPEN register gets 403 BLIND_CASH_CLOSE_ENFORCED
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers/ca5b0001-1111-4222-8333-444455556666/audit-report", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "BLIND_CASH_CLOSE_ENFORCED") {
		t.Fatalf("expected 403 BLIND_CASH_CLOSE_ENFORCED, got %d: %s", rec.Code, rec.Body.String())
	}

	// S3 Adversarial: Negative declared_cash_cents returns 422
	rec = doRequest(mux, http.MethodPost, "/api/v1/billing/cash-registers/ca5b0001-1111-4222-8333-444455556666/blind-close", map[string]any{
		"declared_cash_cents": -10,
	}, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for negative declared cash, got %d", rec.Code)
	}

	// S3: Execute Blind Cash Close (200 OK)
	rec = doRequest(mux, http.MethodPost, "/api/v1/billing/cash-registers/ca5b0001-1111-4222-8333-444455556666/blind-close", map[string]any{
		"declared_cash_cents": 185000,
		"denominations":       map[string]int{"1000": 185},
		"cashier_notes":       "Cierre exacto",
	}, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on blind close, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "theoretical_cash_cents") || strings.Contains(rec.Body.String(), "discrepancy_cents") {
		t.Fatalf("RN-02 violation in blind-close receipt: %s", rec.Body.String())
	}

	// S3 Adversarial: Second blind-close attempt returns 409 CASH_REGISTER_ALREADY_CLOSED
	rec = doRequest(mux, http.MethodPost, "/api/v1/billing/cash-registers/ca5b0001-1111-4222-8333-444455556666/blind-close", map[string]any{
		"declared_cash_cents": 185000,
	}, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "CASH_REGISTER_ALREADY_CLOSED") {
		t.Fatalf("expected 409 CASH_REGISTER_ALREADY_CLOSED, got %d: %s", rec.Code, rec.Body.String())
	}

	// S4: Auditor queries /audit-report (200 OK) and inspects theoretical_cash_cents & discrepancy_cents
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers/ca5b0001-1111-4222-8333-444455556666/audit-report", nil, "auditor-01-uuid", "auditor", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for auditor on audit-report, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"theoretical_cash_cents":185000`) || !strings.Contains(rec.Body.String(), `"discrepancy_cents":0`) {
		t.Fatalf("expected theoretical_cash_cents and discrepancy_cents in audit report, got %s", rec.Body.String())
	}

	// S4 Adversarial: Receptionist cannot view /audit-report even after close (403 Forbidden)
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers/ca5b0001-1111-4222-8333-444455556666/audit-report", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for receptionist on audit-report, got %d", rec.Code)
	}
}
