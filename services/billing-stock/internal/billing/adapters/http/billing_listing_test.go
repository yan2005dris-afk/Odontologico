package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/billing-stock/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/billing-stock/internal/audit/application"
	auditdom "github.com/sigo-dental/billing-stock/internal/audit/domain"
	"github.com/sigo-dental/billing-stock/internal/billing/adapters/fiscal"
	billinghttp "github.com/sigo-dental/billing-stock/internal/billing/adapters/http"
	"github.com/sigo-dental/billing-stock/internal/billing/adapters/postgres"
	"github.com/sigo-dental/billing-stock/internal/billing/application"
	"github.com/sigo-dental/billing-stock/internal/billing/domain"
)

// newBillingServerWithAudit wires the billing context with a shared RNF-06 trail so the
// listing tests can assert both the HTTP contract and the audit evidence.
func newBillingServerWithAudit() (*http.ServeMux, *postgres.InMemoryBillingRepository, *auditpg.AuditRepository) {
	repo := postgres.NewInMemoryBillingRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, nil, nil)
	svc := application.NewBillingService(repo, fiscal.NewTaxAuthorityClient(), fiscal.NewPaymentGatewayClient(), auditSvc)
	handler := billinghttp.NewBillingHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux, repo, auditRepo
}

func seedCashRegister(t *testing.T, repo *postgres.InMemoryBillingRepository, id, clinicID string, openedAt time.Time) *domain.CashRegister {
	t.Helper()
	reg, err := domain.OpenCashRegister(id, clinicID, "recepcionista-01-uuid", 50000, openedAt)
	if err != nil {
		t.Fatalf("open cash register: %v", err)
	}
	if err := repo.SaveCashRegister(context.Background(), reg); err != nil {
		t.Fatalf("save cash register: %v", err)
	}
	return reg
}

func seedInvoice(t *testing.T, repo *postgres.InMemoryBillingRepository, reg *domain.CashRegister, id, patientID string, totalCents int64, createdAt time.Time, stamped bool) {
	t.Helper()
	inv, err := domain.NewInvoice(domain.CreateInvoiceParams{
		ID:             id,
		PatientID:      patientID,
		CashRegisterID: reg.ID,
		CustomerTaxID:  "20-30405060-9",
		CustomerName:   "Paciente " + patientID,
		Items: []domain.InvoiceItemInput{{
			ProcedureCode:  "CORONA-ZIRC",
			Description:    "Corona de circonio",
			Quantity:       1,
			UnitPriceCents: totalCents,
			TaxRateBPS:     0,
		}},
		Payments:  []domain.InvoicePaymentInput{{Method: domain.PaymentMethodCash, AmountCents: totalCents}},
		IssuedBy:  "recepcionista-01-uuid",
		SourceIP:  "192.168.10.25",
		CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatalf("new invoice: %v", err)
	}
	if stamped {
		if err := inv.AuthorizeFiscal("CAE-TEST-0001", "sha256:test", createdAt); err != nil {
			t.Fatalf("authorize invoice: %v", err)
		}
	}
	if err := repo.SaveInvoiceAndCashMovement(context.Background(), inv, reg); err != nil {
		t.Fatalf("save invoice: %v", err)
	}
}

func decodeList(t *testing.T, body string) (map[string]any, []map[string]any) {
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

// T6 + S3: GET /api/v1/billing/invoices answers {items,total} and applies the filters.
func TestBillingHandler_ListInvoices_ContractAndFilters(t *testing.T) {
	mux, repo, _ := newBillingServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	reg := seedCashRegister(t, repo, "reg-inv-01", "sede-central", base)

	seedInvoice(t, repo, reg, "inv-001", "pat-1", 200000, base.Add(1*time.Hour), true)
	seedInvoice(t, repo, reg, "inv-002", "pat-1", 350000, base.Add(2*time.Hour), true)
	seedInvoice(t, repo, reg, "inv-003", "pat-2", 90000, base.Add(3*time.Hour), false)

	rec := doRequest(mux, http.MethodGet, "/api/v1/billing/invoices?patient_id=pat-1", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing invoices, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeList(t, rec.Body.String())
	if payload["total"] != float64(2) {
		t.Fatalf("expected total=2 before pagination, got %v (%s)", payload["total"], rec.Body.String())
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if item["patient_id"] != "pat-1" {
			t.Fatalf("filter patient_id not applied: %v", item)
		}
		if item["fiscal_status"] != domain.InvoiceFiscalStatusAuthorized {
			t.Fatalf("expected fiscal_status AUTHORIZED, got %v", item["fiscal_status"])
		}
		if item["payment_method"] != domain.PaymentMethodCash {
			t.Fatalf("expected payment_method CASH, got %v", item["payment_method"])
		}
		if item["total_cents"] != float64(200000) && item["total_cents"] != float64(350000) {
			t.Fatalf("unexpected total_cents: %v", item["total_cents"])
		}
		if _, ok := item["issued_at"]; !ok {
			t.Fatalf("expected issued_at in item: %v", item)
		}
		if _, ok := item["id"]; !ok {
			t.Fatalf("expected id in item: %v", item)
		}
	}

	// fiscal_status filter
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/invoices?fiscal_status=PENDING", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK filtering by fiscal_status, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items = decodeList(t, rec.Body.String())
	if payload["total"] != float64(1) || len(items) != 1 || items[0]["id"] != "inv-003" {
		t.Fatalf("expected only the unstamped invoice, got total=%v items=%v", payload["total"], items)
	}

	// date range filter (issued_at within the window)
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/invoices?from=2026-01-10&to=2026-01-10", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK filtering by date range, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ = decodeList(t, rec.Body.String())
	if payload["total"] != float64(3) {
		t.Fatalf("expected total=3 for the whole day, got %v", payload["total"])
	}

	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/invoices?from=2026-02-01", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK filtering by future date, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ = decodeList(t, rec.Body.String())
	if payload["total"] != float64(0) {
		t.Fatalf("expected total=0 outside the window, got %v", payload["total"])
	}
}

// T6 + S3: pagination is deterministic (created_at desc, id asc as tiebreak) and the
// limit has a hard ceiling of 200.
func TestBillingHandler_ListInvoices_PaginationAndHardLimitCap(t *testing.T) {
	mux, repo, _ := newBillingServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	reg := seedCashRegister(t, repo, "reg-inv-page", "sede-central", base)

	for i := 0; i < 201; i++ {
		seedInvoice(t, repo, reg, invoiceID(i), "pat-page", 1000+int64(i), base.Add(time.Duration(i)*time.Minute), true)
	}

	rec := doRequest(mux, http.MethodGet, "/api/v1/billing/invoices?limit=1000", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeList(t, rec.Body.String())
	if payload["total"] != float64(201) {
		t.Fatalf("expected total=201, got %v", payload["total"])
	}
	if len(items) != 200 {
		t.Fatalf("expected the hard limit of 200 items, got %d", len(items))
	}

	// Deterministic order: created_at descending (invoice 200 was created last).
	first, ok := items[0]["id"].(string)
	if !ok || first != invoiceID(200) {
		t.Fatalf("expected the newest invoice first, got %v", items[0]["id"])
	}

	// limit/offset window
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/invoices?limit=2&offset=1", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items = decodeList(t, rec.Body.String())
	if payload["total"] != float64(201) {
		t.Fatalf("expected total=201 before pagination, got %v", payload["total"])
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items in the page, got %d", len(items))
	}
	if items[0]["id"] != invoiceID(199) || items[1]["id"] != invoiceID(198) {
		t.Fatalf("unexpected page window: %v / %v", items[0]["id"], items[1]["id"])
	}
}

func invoiceID(i int) string {
	return fmt.Sprintf("inv-page-%03d", i)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// T7 + RN-02: an OPEN cash register never exposes the theoretical total (the key must
// not even exist in the JSON), a CLOSED one does expose it with the right value.
func TestBillingHandler_ListCashRegisters_RN02TheoreticalTotalOmittedWhenOpen(t *testing.T) {
	mux, repo, _ := newBillingServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	openReg := seedCashRegister(t, repo, "reg-open-01", "sede-central", base)
	closedReg := seedCashRegister(t, repo, "reg-closed-01", "sede-norte", base.Add(time.Hour))
	// 50.000 opening float + 135.000 of cash payments = 185.000 theoretical.
	if err := closedReg.RecordCashMovement("mov-001", "inv-closed-01", domain.MovementDirectionIn, 135000, "Cash payment for invoice inv-closed-01", "recepcionista-01-uuid", base.Add(90*time.Minute)); err != nil {
		t.Fatalf("record cash movement: %v", err)
	}
	if _, err := closedReg.ExecuteBlindClose(185000, map[string]int{"1000": 185}, "Cierre exacto", "recepcionista-01-uuid", base.Add(2*time.Hour)); err != nil {
		t.Fatalf("blind close: %v", err)
	}
	if err := repo.UpdateCashRegister(context.Background(), closedReg); err != nil {
		t.Fatalf("update cash register: %v", err)
	}

	// OPEN registers: the theoretical_cents key must not exist at all.
	rec := doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers?status=OPEN", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing cash registers, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "theoretical_cents") {
		t.Fatalf("RN-02 violation: OPEN cash register exposes theoretical_cents: %s", rec.Body.String())
	}
	payload, items := decodeList(t, rec.Body.String())
	if payload["total"] != float64(1) || len(items) != 1 {
		t.Fatalf("expected 1 OPEN cash register, got total=%v items=%v", payload["total"], items)
	}
	if items[0]["id"] != openReg.ID || items[0]["status"] != domain.CashRegisterStatusOpen {
		t.Fatalf("unexpected OPEN cash register item: %v", items[0])
	}

	// CLOSED registers: the theoretical total is present with the reconciled value.
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers?status=CLOSED", nil, "auditor-01-uuid", "auditor", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing closed cash registers, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"theoretical_cents":185000`) {
		t.Fatalf("expected theoretical_cents on the CLOSED cash register, got %s", rec.Body.String())
	}
	_, items = decodeList(t, rec.Body.String())
	if len(items) != 1 {
		t.Fatalf("expected 1 CLOSED cash register, got %d", len(items))
	}
	if items[0]["theoretical_cents"] != float64(185000) {
		t.Fatalf("expected theoretical_cents=185000, got %v", items[0]["theoretical_cents"])
	}
	if items[0]["declared_cents"] != float64(185000) || items[0]["discrepancy_cents"] != float64(0) {
		t.Fatalf("unexpected closed cash register amounts: %v", items[0])
	}

	// Without a status filter the OPEN register must still not carry the key.
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing cash registers, got %d: %s", rec.Code, rec.Body.String())
	}
	_, items = decodeList(t, rec.Body.String())
	if len(items) != 2 {
		t.Fatalf("expected 2 cash registers, got %d", len(items))
	}
	for _, item := range items {
		// RN-02: a receptionist never sees the reconciliation figures, neither
		// for an OPEN register nor for a CLOSED one.
		if _, ok := item["theoretical_cents"]; ok {
			t.Fatalf("RN-02 violation: receptionist sees theoretical_cents: %v", item)
		}
		if _, ok := item["discrepancy_cents"]; ok {
			t.Fatalf("RN-02 violation: receptionist sees discrepancy_cents: %v", item)
		}
	}
}

// RN-02: the theoretical total and the discrepancy are reconciliation figures.
// They belong to auditor/admin only, exactly as in GetAuditReport: a
// receptionist that could read the discrepancy could subtract it from the
// declared count and recover the theoretical total by arithmetic.
func TestBillingHandler_ListCashRegisters_ReconciliationFiguresAreAuditorAdminOnly(t *testing.T) {
	mux, repo, _ := newBillingServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	closedReg := seedCashRegister(t, repo, "reg-closed-rbac", "sede-central", base)
	if err := closedReg.RecordCashMovement("mov-rbac", "inv-rbac", domain.MovementDirectionIn, 135000, "Cash payment", "recepcionista-01-uuid", base.Add(90*time.Minute)); err != nil {
		t.Fatalf("record cash movement: %v", err)
	}
	if _, err := closedReg.ExecuteBlindClose(180000, map[string]int{"1000": 180}, "Cierre con diferencia", "recepcionista-01-uuid", base.Add(2*time.Hour)); err != nil {
		t.Fatalf("blind close: %v", err)
	}
	if err := repo.UpdateCashRegister(context.Background(), closedReg); err != nil {
		t.Fatalf("update cash register: %v", err)
	}

	// The receptionist sees the declared count (its own submission) but neither
	// the theoretical total nor the discrepancy.
	rec := doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing cash registers, got %d: %s", rec.Code, rec.Body.String())
	}
	_, items := decodeList(t, rec.Body.String())
	if len(items) != 1 {
		t.Fatalf("expected 1 cash register, got %d", len(items))
	}
	if _, ok := items[0]["theoretical_cents"]; ok {
		t.Fatalf("RN-02 violation: receptionist sees theoretical_cents: %v", items[0])
	}
	if _, ok := items[0]["discrepancy_cents"]; ok {
		t.Fatalf("RN-02 violation: receptionist sees discrepancy_cents: %v", items[0])
	}
	if items[0]["declared_cents"] != float64(180000) {
		t.Fatalf("expected the declared count to stay visible, got %v", items[0])
	}

	// The auditor reconciles: both figures are present and coherent.
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers", nil, "auditor-01-uuid", "auditor", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK listing cash registers, got %d: %s", rec.Code, rec.Body.String())
	}
	_, items = decodeList(t, rec.Body.String())
	if len(items) != 1 {
		t.Fatalf("expected 1 cash register, got %d", len(items))
	}
	if items[0]["theoretical_cents"] != float64(185000) {
		t.Fatalf("expected theoretical_cents=185000, got %v", items[0]["theoretical_cents"])
	}
	if items[0]["discrepancy_cents"] != float64(-5000) {
		t.Fatalf("expected discrepancy_cents=-5000, got %v", items[0]["discrepancy_cents"])
	}
}

// T6: clinic_id filter and the pagination contract of the cash register listing.
func TestBillingHandler_ListCashRegisters_FiltersAndPagination(t *testing.T) {
	mux, repo, _ := newBillingServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		seedCashRegister(t, repo, "reg-page-"+itoa(i), "sede-central", base.Add(time.Duration(i)*time.Minute))
	}
	seedCashRegister(t, repo, "reg-page-other", "sede-norte", base)

	rec := doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers?clinic_id=sede-central", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, items := decodeList(t, rec.Body.String())
	if payload["total"] != float64(3) || len(items) != 3 {
		t.Fatalf("expected 3 cash registers of sede-central, got total=%v items=%d", payload["total"], len(items))
	}
	// created_at descending
	if items[0]["id"] != "reg-page-2" {
		t.Fatalf("expected the newest cash register first, got %v", items[0]["id"])
	}

	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers?clinic_id=sede-norte", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ = decodeList(t, rec.Body.String())
	if payload["total"] != float64(1) {
		t.Fatalf("expected total=1 for sede-norte, got %v", payload["total"])
	}
}

// T10 + RNF-06: a failing recorder never invalidates the listing (the error is only logged).
func TestBillingHandler_Listings_AuditFailureStillAnswers200(t *testing.T) {
	repo := postgres.NewInMemoryBillingRepository()
	svc := application.NewBillingService(repo, fiscal.NewTaxAuthorityClient(), fiscal.NewPaymentGatewayClient(), failingRecorder{})
	handler := billinghttp.NewBillingHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	rec := doRequest(mux, http.MethodGet, "/api/v1/billing/invoices", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK even when the audit recorder fails, got %d: %s", rec.Code, rec.Body.String())
	}
	payload, _ := decodeList(t, rec.Body.String())
	if payload["total"] != float64(0) {
		t.Fatalf("expected an empty but valid listing, got %v", payload["total"])
	}
}

// failingRecorder simulates an unavailable audit sink.
type failingRecorder struct{}

func (failingRecorder) Record(_ context.Context, _ auditapp.RecordInput) (*auditdom.AuditEntry, error) {
	return nil, errors.New("audit sink unavailable")
}

// T10 + RNF-06: every listing leaves an immutable EXPORT trail.
func TestBillingHandler_Listings_RecordExportAuditTrail(t *testing.T) {
	mux, repo, auditRepo := newBillingServerWithAudit()
	base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	reg := seedCashRegister(t, repo, "reg-audit-01", "sede-central", base)
	seedInvoice(t, repo, reg, "inv-audit-01", "pat-1", 10000, base, true)

	rec := doRequest(mux, http.MethodGet, "/api/v1/billing/invoices", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = doRequest(mux, http.MethodGet, "/api/v1/billing/cash-registers", nil, "recepcionista-01-uuid", "receptionist", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	entries, total, err := auditRepo.List(context.Background(), auditapp.AuditFilter{Action: string(auditdom.ActionExport)})
	if err != nil {
		t.Fatalf("list audit trail: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 EXPORT audit entries, got %d", total)
	}
	for _, entry := range entries {
		if string(entry.Action) != "EXPORT" {
			t.Fatalf("expected action EXPORT, got %q", entry.Action)
		}
		if entry.ActorID != "recepcionista-01-uuid" || entry.ActorRole != "receptionist" {
			t.Fatalf("unexpected actor in audit entry: %+v", entry)
		}
		if entry.ResourceType == "" || entry.ResourceID == "" {
			t.Fatalf("audit entry requires resource_type and resource_id: %+v", entry)
		}
		if entry.SourceIP != "192.168.10.25" {
			t.Fatalf("unexpected source_ip in audit entry: %+v", entry)
		}
		if _, err := time.Parse(time.RFC3339, entry.TimestampISO8601); err != nil {
			t.Fatalf("expected an ISO-8601 timestamp, got %q", entry.TimestampISO8601)
		}
	}
}
