package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	settlementhttp "github.com/sigo-dental/billing-stock/internal/settlements/adapters/http"
	"github.com/sigo-dental/billing-stock/internal/settlements/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/settlements/application"
)

func doSettlementReq(mux *http.ServeMux, method, path string, body any, actorID, role string, mfa bool) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Actor-ID", actorID)
	req.Header.Set("X-Actor-Role", role)
	if mfa {
		req.Header.Set("X-MFA-Verified", "true")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestSettlementHandler_EndToEnd_S1_S2(t *testing.T) {
	repo := postgres.NewInMemorySettlementRepository()
	svc := application.NewSettlementService(repo)
	handler := settlementhttp.NewSettlementHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Receptionist cannot configure rules (403 Forbidden)
	rec := doSettlementReq(mux, http.MethodPost, "/api/v1/settlements/rules", map[string]any{
		"dentist_id":       "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
		"procedure_code":   "*",
		"calculation_mode": "NET_PERCENTAGE_AFTER_COSTS",
		"rate_bps":         5000,
	}, "recep-01", "receptionist", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for receptionist configuring rules, got %d", rec.Code)
	}

	// Invalid rate_bps > 10000 returns 422
	rec = doSettlementReq(mux, http.MethodPost, "/api/v1/settlements/rules", map[string]any{
		"dentist_id":       "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
		"procedure_code":   "*",
		"calculation_mode": "NET_PERCENTAGE_AFTER_COSTS",
		"rate_bps":         12000,
	}, "admin-01", "admin", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for rate_bps > 10000, got %d", rec.Code)
	}

	// Admin configures NET_PERCENTAGE_AFTER_COSTS rule (50% = 5000 bps)
	rec = doSettlementReq(mux, http.MethodPost, "/api/v1/settlements/rules", map[string]any{
		"dentist_id":       "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
		"procedure_code":   "*",
		"calculation_mode": "NET_PERCENTAGE_AFTER_COSTS",
		"rate_bps":         5000,
	}, "admin-01", "admin", true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created configuring rule, got %d: %s", rec.Code, rec.Body.String())
	}

	// Generate settlement (gross 500,000 - lab 80,000 - mat 20,000 = 400,000 * 50% = 200,000)
	genPayload := map[string]any{
		"settlement_id": "77889900-aabb-4ccd-8eef-112233445566",
		"dentist_id":    "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
		"period_start":  "2026-10-01",
		"period_end":    "2026-10-31",
		"procedures": []map[string]any{
			{
				"completed_procedure_id": "proc-uuid-900",
				"procedure_code":         "PROTESIS-FIJA",
				"gross_fee_cents":        500000,
				"lab_cost_cents":         80000,
				"material_cost_cents":    20000,
			},
		},
	}
	rec = doSettlementReq(mux, http.MethodPost, "/api/v1/settlements/generate", genPayload, "auditor-01", "auditor", true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created generating settlement, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"net_payable_commission_cents":200000`) {
		t.Fatalf("expected net_payable_commission_cents 200000, got %s", rec.Body.String())
	}

	// Duplicate settlement of proc-uuid-900 returns 409 PROCEDURE_ALREADY_SETTLED
	rec = doSettlementReq(mux, http.MethodPost, "/api/v1/settlements/generate", genPayload, "auditor-01", "auditor", true)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "PROCEDURE_ALREADY_SETTLED") {
		t.Fatalf("expected 409 PROCEDURE_ALREADY_SETTLED, got %d: %s", rec.Code, rec.Body.String())
	}

	// Adversarial MFA & Not Found rule check
	rec = doSettlementReq(mux, http.MethodPost, "/api/v1/settlements/rules", map[string]any{}, "admin-01", "admin", false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when MFA missing, got %d", rec.Code)
	}
	rec = doSettlementReq(mux, http.MethodPost, "/api/v1/settlements/generate", map[string]any{
		"dentist_id":   "unknown-dentist-uuid",
		"period_start": "2026-10-01",
		"period_end":   "2026-10-31",
		"procedures": []map[string]any{
			{"completed_procedure_id": "proc-unruled", "procedure_code": "IMPLANTE", "gross_fee_cents": 100000},
		},
	}, "admin-01", "admin", true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when dentist has no commission rule, got %d", rec.Code)
	}
}

