package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/billing-stock/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/billing-stock/internal/audit/application"
	auditdom "github.com/sigo-dental/billing-stock/internal/audit/domain"
	inventoryhttp "github.com/sigo-dental/billing-stock/internal/inventory/adapters/http"
	"github.com/sigo-dental/billing-stock/internal/inventory/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/inventory/application"
	"github.com/sigo-dental/billing-stock/internal/inventory/domain"
)

func newInventoryServerWithAudit() (*http.ServeMux, *postgres.InMemoryInventoryRepository, *auditpg.AuditRepository) {
	repo := postgres.NewInMemoryInventoryRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, nil, nil)
	svc := application.NewInventoryService(repo, auditSvc)
	handler := inventoryhttp.NewInventoryHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, repo, auditRepo
}

func seedSupply(t *testing.T, repo *postgres.InMemoryInventoryRepository, id, clinicID, sku, name string, currentStock, minStock int) {
	t.Helper()
	supply, err := domain.NewSupply(id, clinicID, sku, name, "unit", "BATCH-01", "2027-01-01", currentStock, minStock, 1500)
	if err != nil {
		t.Fatalf("new supply: %v", err)
	}
	if err := repo.UpsertSupply(context.Background(), supply); err != nil {
		t.Fatalf("upsert supply: %v", err)
	}
}

func seedRecipe(t *testing.T, repo *postgres.InMemoryInventoryRepository, procedureCode string, items []domain.RecipeItem) {
	t.Helper()
	recipe, err := domain.NewProcedureRecipe(procedureCode, items)
	if err != nil {
		t.Fatalf("new recipe: %v", err)
	}
	if err := repo.SaveProcedureRecipe(context.Background(), recipe); err != nil {
		t.Fatalf("save recipe: %v", err)
	}
}

func decodeInventoryList(t *testing.T, body string) (map[string]any, []map[string]any) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode list payload: %v (body=%s)", err, body)
	}
	rawItems, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("expected items array in payload, got %s", body)
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected item object, got %T", raw)
		}
		items = append(items, item)
	}
	return payload, items
}

// T9 + S3: GET /api/v1/inventory/items answers {items,total} with the documented DTO.
func TestInventoryHandler_ListSupplies_ContractAndFilters(t *testing.T) {
	mux, repo, _ := newInventoryServerWithAudit()

	seedSupply(t, repo, "sup-001", "sede-central", "GLOVE-M", "Guante de nitrilo M", 10, 5)
	seedSupply(t, repo, "sup-002", "sede-central", "MASQ-C", "Mascarilla quirúrgica", 2, 5)
	seedSupply(t, repo, "sup-003", "sede-norte", "GLOVE-M", "Guante de nitrilo M", 20, 5)

	rec := doInventoryReq(mux, http.MethodGet, "/api/v1/inventory/items", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing supplies, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeInventoryList(t, rec.Body.String())
	if payload["total"] != float64(3) || len(items) != 3 {
		t.Fatalf("expected 3 supplies, got total=%v items=%d", payload["total"], len(items))
	}
	first := items[0]
	if first["sku"] != "GLOVE-M" || first["name"] != "Guante de nitrilo M" {
		t.Fatalf("unexpected supply item: %v", first)
	}
	if first["current_stock"] != float64(10) || first["min_stock"] != float64(5) {
		t.Fatalf("unexpected stock counters: %v", first)
	}
	if first["clinic_id"] != "sede-central" || first["unit"] != "unit" {
		t.Fatalf("unexpected clinic/unit: %v", first)
	}
	if first["low_stock"] != false {
		t.Fatalf("expected low_stock=false for a stocked item, got %v", first["low_stock"])
	}

	// clinic_id filter
	rec = doInventoryReq(mux, http.MethodGet, "/api/v1/inventory/items?clinic_id=sede-central", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ = decodeInventoryList(t, rec.Body.String())
	if payload["total"] != float64(2) {
		t.Fatalf("expected total=2 for sede-central, got %v", payload["total"])
	}

	// q filter (sku or name)
	rec = doInventoryReq(mux, http.MethodGet, "/api/v1/inventory/items?q=guante", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items = decodeInventoryList(t, rec.Body.String())
	if payload["total"] != float64(2) {
		t.Fatalf("expected total=2 for q=guante, got %v", payload["total"])
	}
	for _, item := range items {
		if item["name"] != "Guante de nitrilo M" {
			t.Fatalf("q filter not applied: %v", item)
		}
	}

	// low_stock filter
	rec = doInventoryReq(mux, http.MethodGet, "/api/v1/inventory/items?low_stock=true", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items = decodeInventoryList(t, rec.Body.String())
	if payload["total"] != float64(1) || len(items) != 1 {
		t.Fatalf("expected 1 low stock supply, got total=%v items=%d", payload["total"], len(items))
	}
	if items[0]["sku"] != "MASQ-C" || items[0]["low_stock"] != true {
		t.Fatalf("unexpected low stock item: %v", items[0])
	}
}

// T9 + S3: pagination is deterministic and the limit has a hard ceiling of 200.
func TestInventoryHandler_ListSupplies_PaginationAndHardLimitCap(t *testing.T) {
	mux, repo, _ := newInventoryServerWithAudit()
	for i := 0; i < 201; i++ {
		seedSupply(t, repo, fmt.Sprintf("sup-page-%03d", i), "sede-central", fmt.Sprintf("SKU-%03d", i), "Insumo", 10, 5)
	}

	rec := doInventoryReq(mux, http.MethodGet, "/api/v1/inventory/items?limit=1000", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeInventoryList(t, rec.Body.String())
	if payload["total"] != float64(201) {
		t.Fatalf("expected total=201, got %v", payload["total"])
	}
	if len(items) != 200 {
		t.Fatalf("expected the hard limit of 200 items, got %d", len(items))
	}

	rec = doInventoryReq(mux, http.MethodGet, "/api/v1/inventory/items?limit=2&offset=1", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items = decodeInventoryList(t, rec.Body.String())
	if payload["total"] != float64(201) {
		t.Fatalf("expected total=201 before pagination, got %v", payload["total"])
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items in the page, got %d", len(items))
	}
}

// T9 + S3: GET /api/v1/inventory/recipes lists the standard recipes per procedure.
func TestInventoryHandler_ListRecipes_ContractAndFilters(t *testing.T) {
	mux, repo, _ := newInventoryServerWithAudit()

	seedRecipe(t, repo, "CORONA-ZIRC", []domain.RecipeItem{{SupplySKU: "GLOVE-M", QuantityPerProcedure: 2}, {SupplySKU: "MASQ-C", QuantityPerProcedure: 1}})
	seedRecipe(t, repo, "IMPLANTE", []domain.RecipeItem{{SupplySKU: "GLOVE-M", QuantityPerProcedure: 4}})

	rec := doInventoryReq(mux, http.MethodGet, "/api/v1/inventory/recipes", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing recipes, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeInventoryList(t, rec.Body.String())
	if payload["total"] != float64(2) || len(items) != 2 {
		t.Fatalf("expected 2 recipes, got total=%v items=%d", payload["total"], len(items))
	}
	if items[0]["procedure_code"] != "CORONA-ZIRC" {
		t.Fatalf("unexpected procedure order: %v", items[0]["procedure_code"])
	}
	rawRecipeItems, ok := items[0]["items"].([]any)
	if !ok || len(rawRecipeItems) != 2 {
		t.Fatalf("expected 2 recipe items, got %v", items[0]["items"])
	}
	recipeItem, ok := rawRecipeItems[0].(map[string]any)
	if !ok {
		t.Fatalf("expected a recipe item object, got %T", rawRecipeItems[0])
	}
	if recipeItem["sku"] != "GLOVE-M" || recipeItem["quantity"] != float64(2) {
		t.Fatalf("unexpected recipe item: %v", recipeItem)
	}

	rec = doInventoryReq(mux, http.MethodGet, "/api/v1/inventory/recipes?procedure_code=IMPLANTE", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items = decodeInventoryList(t, rec.Body.String())
	if payload["total"] != float64(1) || len(items) != 1 || items[0]["procedure_code"] != "IMPLANTE" {
		t.Fatalf("expected only the IMPLANTE recipe, got total=%v items=%v", payload["total"], items)
	}
}

// T10 + RNF-06: the inventory listings leave an immutable EXPORT trail.
func TestInventoryHandler_Listings_RecordExportAuditTrail(t *testing.T) {
	mux, repo, auditRepo := newInventoryServerWithAudit()
	seedSupply(t, repo, "sup-audit-01", "sede-central", "GLOVE-M", "Guante de nitrilo M", 10, 5)
	seedRecipe(t, repo, "CORONA-ZIRC", []domain.RecipeItem{{SupplySKU: "GLOVE-M", QuantityPerProcedure: 2}})

	for _, path := range []string{"/api/v1/inventory/items", "/api/v1/inventory/recipes"} {
		rec := doInventoryReq(mux, http.MethodGet, path, nil, "admin-01", "admin", true)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on %s, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}

	entries, total, err := auditRepo.List(context.Background(), auditapp.AuditFilter{Action: string(auditdom.ActionExport)})
	if err != nil {
		t.Fatalf("list audit trail: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 EXPORT audit entries, got %d", total)
	}
	for _, entry := range entries {
		if entry.ActorID != "admin-01" || entry.ActorRole != "admin" {
			t.Fatalf("unexpected actor in audit entry: %+v", entry)
		}
		if entry.ResourceType == "" || entry.ResourceID == "" {
			t.Fatalf("audit entry requires resource_type and resource_id: %+v", entry)
		}
		if _, err := time.Parse(time.RFC3339, entry.TimestampISO8601); err != nil {
			t.Fatalf("expected an ISO-8601 timestamp, got %q", entry.TimestampISO8601)
		}
	}
}
