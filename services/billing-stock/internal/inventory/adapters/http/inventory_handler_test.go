package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	inventoryhttp "github.com/sigo-dental/billing-stock/internal/inventory/adapters/http"
	"github.com/sigo-dental/billing-stock/internal/inventory/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/inventory/application"
)

func doInventoryReq(mux *http.ServeMux, method, path string, body any, actorID, role string, mfa bool) *httptest.ResponseRecorder {
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

func TestInventoryHandler_EndToEnd_S3_S4(t *testing.T) {
	repo := postgres.NewInMemoryInventoryRepository()
	svc := application.NewInventoryService(repo)
	handler := inventoryhttp.NewInventoryHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Upsert supplies
	rec := doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/items", map[string]any{
		"clinic_id":          "clinic-uuid-1",
		"sku":                "ANES-LIDO-2",
		"name":               "Anestesia Lidocaína 2%",
		"unit":               "cartridge",
		"current_stock":      49,
		"min_stock_quantity": 10,
		"unit_cost_cents":    1200,
	}, "admin-01", "admin", true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for supply 1, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/items", map[string]any{
		"clinic_id":          "clinic-uuid-1",
		"sku":                "RESINA-A2-3M",
		"name":               "Resina 3M Filtek A2",
		"unit":               "syringe",
		"current_stock":      4,
		"min_stock_quantity": 3,
		"unit_cost_cents":    2000,
	}, "admin-01", "admin", true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for supply 2, got %d: %s", rec.Code, rec.Body.String())
	}

	// Configure recipe
	rec = doInventoryReq(mux, http.MethodPut, "/api/v1/inventory/recipes/REST-RESINA-1S", map[string]any{
		"items": []map[string]any{
			{"supply_sku": "ANES-LIDO-2", "quantity_per_procedure": 1},
			{"supply_sku": "RESINA-A2-3M", "quantity_per_procedure": 1},
		},
	}, "dentist-01", "dentist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK saving recipe, got %d: %s", rec.Code, rec.Body.String())
	}

	// S3: Deduct procedure completed (200 OK)
	deductPayload := map[string]any{
		"completed_procedure_id": "55667788-1122-4333-8445-667788990011",
		"clinic_id":              "clinic-uuid-1",
		"dentist_id":             "dentist-01",
		"procedure_code":         "REST-RESINA-1S",
	}
	rec = doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/deductions/procedure-completed", deductPayload, "dentist-01", "dentist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deducting procedure, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"total_material_cost_cents":3200`) || !strings.Contains(rec.Body.String(), `"low_stock_alert":true`) {
		t.Fatalf("expected total_material_cost_cents 3200 and low_stock_alert true, got %s", rec.Body.String())
	}

	// S4: Idempotent retry returns 200 OK with same remaining stock (48 and 3)
	rec = doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/deductions/procedure-completed", deductPayload, "dentist-01", "dentist", true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"remaining_stock":3`) {
		t.Fatalf("expected idempotent 200 OK with remaining_stock 3, got %d: %s", rec.Code, rec.Body.String())
	}

	// S4 Adversarial: Deplete RESINA-A2-3M beyond remaining 3 units -> 409 INSUFFICIENT_SUPPLY_STOCK
	_ = doInventoryReq(mux, http.MethodPut, "/api/v1/inventory/recipes/GRAN-CIRUGIA", map[string]any{
		"items": []map[string]any{
			{"supply_sku": "RESINA-A2-3M", "quantity_per_procedure": 10},
		},
	}, "dentist-01", "dentist", true)

	rec = doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/deductions/procedure-completed", map[string]any{
		"completed_procedure_id": "proc-fail-stock",
		"clinic_id":              "clinic-uuid-1",
		"dentist_id":             "dentist-01",
		"procedure_code":         "GRAN-CIRUGIA",
	}, "dentist-01", "dentist", true)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "INSUFFICIENT_SUPPLY_STOCK") {
		t.Fatalf("expected 409 INSUFFICIENT_SUPPLY_STOCK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Adversarial MFA, Role, NotFound, and UnprocessableEntity checks
	rec = doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/items", map[string]any{}, "admin-01", "admin", false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without MFA, got %d", rec.Code)
	}
	rec = doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/items", map[string]any{}, "recep-01", "receptionist", true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for receptionist creating supply, got %d", rec.Code)
	}
	rec = doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/items", map[string]any{
		"clinic_id":     "clinic-uuid-1",
		"sku":           "",
		"current_stock": -1,
	}, "admin-01", "admin", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid supply, got %d", rec.Code)
	}
	rec = doInventoryReq(mux, http.MethodPut, "/api/v1/inventory/recipes/EMPTY-RECIPE", map[string]any{
		"items": []map[string]any{},
	}, "admin-01", "admin", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for empty recipe items, got %d", rec.Code)
	}
	rec = doInventoryReq(mux, http.MethodPost, "/api/v1/inventory/deductions/procedure-completed", map[string]any{
		"completed_procedure_id": "proc-unknown-recipe",
		"clinic_id":              "clinic-uuid-1",
		"dentist_id":             "dentist-01",
		"procedure_code":         "NON-EXISTENT-PROC",
	}, "dentist-01", "dentist", true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown recipe, got %d", rec.Code)
	}
}

