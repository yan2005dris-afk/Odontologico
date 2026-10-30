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
	settlementhttp "github.com/sigo-dental/billing-stock/internal/settlements/adapters/http"
	"github.com/sigo-dental/billing-stock/internal/settlements/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/settlements/application"
	"github.com/sigo-dental/billing-stock/internal/settlements/domain"
)

func newSettlementServerWithAudit() (*http.ServeMux, *postgres.InMemorySettlementRepository, *auditpg.AuditRepository) {
	repo := postgres.NewInMemorySettlementRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, nil, nil)
	svc := application.NewSettlementService(repo, auditSvc)
	handler := settlementhttp.NewSettlementHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, repo, auditRepo
}

func seedSettlement(t *testing.T, repo *postgres.InMemorySettlementRepository, id, dentistID, periodStart, periodEnd string, createdAt time.Time, procedureCount int) {
	t.Helper()
	procedures := make([]domain.CompletedProcedureInput, 0, procedureCount)
	for i := 0; i < procedureCount; i++ {
		procedures = append(procedures, domain.CompletedProcedureInput{
			CompletedProcedureID: fmt.Sprintf("%s-proc-%d", id, i),
			ProcedureCode:        "CORONA-ZIRC",
			GrossFeeCents:        100000,
			LabCostCents:         10000,
			MaterialCostCents:    5000,
		})
	}
	settlement, err := domain.BuildDentistSettlement(id, dentistID, periodStart, periodEnd, "admin-01", procedures,
		func(string) (*domain.CommissionRule, error) {
			return &domain.CommissionRule{
				ID:              "rule-" + dentistID,
				DentistID:       dentistID,
				ProcedureCode:   "*",
				CalculationMode: domain.ModeNetPercentageAfterCosts,
				RateBPS:         2500,
			}, nil
		})
	if err != nil {
		t.Fatalf("build settlement: %v", err)
	}
	settlement.CreatedAt = createdAt
	if err := repo.SaveSettlement(context.Background(), settlement); err != nil {
		t.Fatalf("save settlement: %v", err)
	}
}

func seedCommissionRule(t *testing.T, repo *postgres.InMemorySettlementRepository, id, dentistID, procedureCode string) {
	t.Helper()
	rule, err := domain.NewCommissionRule(id, dentistID, procedureCode, domain.ModeFixedPercentage, 2500, 0)
	if err != nil {
		t.Fatalf("new commission rule: %v", err)
	}
	if err := repo.UpsertCommissionRule(context.Background(), rule); err != nil {
		t.Fatalf("upsert commission rule: %v", err)
	}
}

func decodeSettlementList(t *testing.T, body string) (map[string]any, []map[string]any) {
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

// T8 + S3: GET /api/v1/settlements answers {items,total} with the documented DTO.
func TestSettlementHandler_ListSettlements_ContractAndFilters(t *testing.T) {
	mux, repo, _ := newSettlementServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	seedSettlement(t, repo, "set-001", "dentist-01", "2026-01-01", "2026-01-31", base, 2)
	seedSettlement(t, repo, "set-002", "dentist-01", "2026-02-01", "2026-02-28", base.Add(time.Hour), 1)
	seedSettlement(t, repo, "set-003", "dentist-02", "2026-01-01", "2026-01-31", base.Add(2*time.Hour), 3)

	rec := doSettlementReq(mux, http.MethodGet, "/api/v1/settlements", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing settlements, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(3) {
		t.Fatalf("expected total=3, got %v", payload["total"])
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	// created_at descending
	if items[0]["dentist_id"] != "dentist-02" {
		t.Fatalf("expected the newest settlement first, got %v", items[0]["dentist_id"])
	}
	first := items[1]
	if first["dentist_id"] != "dentist-01" {
		t.Fatalf("unexpected dentist_id: %v", first["dentist_id"])
	}
	if first["period_from"] != "2026-02-01" || first["period_to"] != "2026-02-28" {
		t.Fatalf("unexpected period: %v / %v", first["period_from"], first["period_to"])
	}
	if first["procedure_count"] != float64(1) {
		t.Fatalf("expected procedure_count=1, got %v", first["procedure_count"])
	}
	if first["gross_cents"] != float64(100000) {
		t.Fatalf("expected gross_cents=100000, got %v", first["gross_cents"])
	}
	// net base = 100.000 - 10.000 lab - 5.000 material = 85.000 -> commission 25% = 21.250
	if first["commission_cents"] != float64(21250) {
		t.Fatalf("expected commission_cents=21250, got %v", first["commission_cents"])
	}
	// net = gross - lab - material - commission = 100.000 - 10.000 - 5.000 - 21.250
	if first["net_cents"] != float64(63750) {
		t.Fatalf("expected net_cents=63750, got %v", first["net_cents"])
	}

	// dentist_id filter
	rec = doSettlementReq(mux, http.MethodGet, "/api/v1/settlements?dentist_id=dentist-01", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ = decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(2) {
		t.Fatalf("expected total=2 for dentist-01, got %v", payload["total"])
	}

	// date range filter over the creation date
	rec = doSettlementReq(mux, http.MethodGet, "/api/v1/settlements?from=2026-01-10&to=2026-01-10", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ = decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(3) {
		t.Fatalf("expected total=3 for the whole day, got %v", payload["total"])
	}

	rec = doSettlementReq(mux, http.MethodGet, "/api/v1/settlements?from=2026-03-01", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ = decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(0) {
		t.Fatalf("expected total=0 outside the window, got %v", payload["total"])
	}
}

// T8 + S3: pagination is deterministic and the limit has a hard ceiling of 200.
func TestSettlementHandler_ListSettlements_PaginationAndHardLimitCap(t *testing.T) {
	mux, repo, _ := newSettlementServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 201; i++ {
		seedSettlement(t, repo, fmt.Sprintf("set-page-%03d", i), "dentist-page", "2026-01-01", "2026-01-31", base.Add(time.Duration(i)*time.Minute), 1)
	}

	rec := doSettlementReq(mux, http.MethodGet, "/api/v1/settlements?limit=1000", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(201) {
		t.Fatalf("expected total=201, got %v", payload["total"])
	}
	if len(items) != 200 {
		t.Fatalf("expected the hard limit of 200 items, got %d", len(items))
	}

	rec = doSettlementReq(mux, http.MethodGet, "/api/v1/settlements?limit=2&offset=1", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items = decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(201) {
		t.Fatalf("expected total=201 before pagination, got %v", payload["total"])
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items in the page, got %d", len(items))
	}
}

// T8 + S3: GET /api/v1/settlements/rules lists the commission rules.
func TestSettlementHandler_ListCommissionRules_ContractAndFilters(t *testing.T) {
	mux, repo, _ := newSettlementServerWithAudit()

	seedCommissionRule(t, repo, "rule-001", "dentist-01", "CORONA-ZIRC")
	seedCommissionRule(t, repo, "rule-002", "dentist-01", "IMPLANTE")
	seedCommissionRule(t, repo, "rule-003", "dentist-02", "CORONA-ZIRC")

	rec := doSettlementReq(mux, http.MethodGet, "/api/v1/settlements/rules", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing commission rules, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(3) || len(items) != 3 {
		t.Fatalf("expected 3 commission rules, got total=%v items=%d", payload["total"], len(items))
	}
	if items[0]["dentist_id"] != "dentist-01" || items[0]["procedure_code"] != "CORONA-ZIRC" {
		t.Fatalf("unexpected commission rule item: %v", items[0])
	}
	if items[0]["calculation_mode"] != domain.ModeFixedPercentage {
		t.Fatalf("expected calculation_mode FIXED_PERCENTAGE, got %v", items[0]["calculation_mode"])
	}
	if items[0]["rate_bps"] != float64(2500) {
		t.Fatalf("expected rate_bps=2500, got %v", items[0]["rate_bps"])
	}

	rec = doSettlementReq(mux, http.MethodGet, "/api/v1/settlements/rules?dentist_id=dentist-01", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ = decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(2) {
		t.Fatalf("expected total=2 for dentist-01, got %v", payload["total"])
	}

	rec = doSettlementReq(mux, http.MethodGet, "/api/v1/settlements/rules?dentist_id=dentist-01&procedure_code=IMPLANTE", nil, "admin-01", "admin", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items = decodeSettlementList(t, rec.Body.String())
	if payload["total"] != float64(1) || len(items) != 1 || items[0]["procedure_code"] != "IMPLANTE" {
		t.Fatalf("expected only the IMPLANTE rule, got total=%v items=%v", payload["total"], items)
	}
}

// T10 + RNF-06: the settlement listings leave an immutable EXPORT trail.
func TestSettlementHandler_Listings_RecordExportAuditTrail(t *testing.T) {
	mux, repo, auditRepo := newSettlementServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	seedSettlement(t, repo, "set-audit-01", "dentist-01", "2026-01-01", "2026-01-31", base, 1)
	seedCommissionRule(t, repo, "rule-audit-01", "dentist-01", "CORONA-ZIRC")

	for _, path := range []string{"/api/v1/settlements", "/api/v1/settlements/rules"} {
		rec := doSettlementReq(mux, http.MethodGet, path, nil, "admin-01", "admin", true)
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
