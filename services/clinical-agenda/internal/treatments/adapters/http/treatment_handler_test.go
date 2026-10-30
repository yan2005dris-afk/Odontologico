package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	auditdom "github.com/sigo-dental/clinical-agenda/internal/audit/domain"
	treatmenthttp "github.com/sigo-dental/clinical-agenda/internal/treatments/adapters/http"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/application"
	"github.com/sigo-dental/clinical-agenda/internal/treatments/domain"
)

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time { return f.now }

func TestTreatmentHandler_Endpoints(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 30, 0, 0, time.UTC)
	repo := postgres.NewTreatmentRepository()
	svc := application.NewTreatmentService(repo, nil, fixedClock{now: now})
	handler := treatmenthttp.NewTreatmentHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 0. Missing MFA -> 403 & Bad JSON -> 400 & NotFound -> 404 & Validation -> 422
	reqNoMFA := httptest.NewRequest(http.MethodGet, "/api/v1/patients/p-1/schedulable-tasks", nil)
	recNoMFA := httptest.NewRecorder()
	mux.ServeHTTP(recNoMFA, reqNoMFA)
	if recNoMFA.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without MFA, got %d", recNoMFA.Code)
	}
	for _, p := range []string{"/api/v1/patients/p-1/treatment-plans", "/api/v1/treatment-plans/plan-1/approve"} {
		reqBadJSON := httptest.NewRequest(http.MethodPost, p, bytes.NewBufferString("{bad"))
		reqBadJSON.Header.Set("X-Actor-Role", "dentist")
		reqBadJSON.Header.Set("X-MFA-Verified", "true")
		recBadJSON := httptest.NewRecorder()
		mux.ServeHTTP(recBadJSON, reqBadJSON)
		if recBadJSON.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on bad JSON, got %d", recBadJSON.Code)
		}
	}
	reqVal := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/treatment-plans", bytes.NewBufferString(`{"title":""}`))
	reqVal.Header.Set("X-Actor-Role", "dentist")
	reqVal.Header.Set("X-MFA-Verified", "true")
	recVal := httptest.NewRecorder()
	mux.ServeHTTP(recVal, reqVal)
	if recVal.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on invalid plan, got %d", recVal.Code)
	}
	reqNF := httptest.NewRequest(http.MethodPost, "/api/v1/treatment-plans/missing/items/i-1/complete", nil)
	reqNF.Header.Set("X-Actor-Role", "dentist")
	reqNF.Header.Set("X-MFA-Verified", "true")
	recNF := httptest.NewRecorder()
	mux.ServeHTTP(recNF, reqNF)
	if recNF.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on missing plan, got %d", recNF.Code)
	}

	// 1. Create multi-phase treatment plan -> 201 Created (S1)
	planPayload := `{
		"title": "Plan de Rehabilitación",
		"currency": "USD",
		"phases": [
			{
				"phase_number": 1,
				"name": "Fase 1: Endodoncia",
				"items": [{
					"id": "55667788-1122-4333-8445-667788990011",
					"procedure_code": "ENDO-01",
					"procedure_name": "Endodoncia Birradicular",
					"tooth_fdi": "14",
					"professional_fee_cents": 40000,
					"lab_cost_cents": 10000,
					"discount_cents": 5000,
					"estimated_duration_minutes": 60
				}]
			},
			{
				"phase_number": 2,
				"name": "Fase 2: Corona",
				"items": [{
					"id": "55667788-1122-4333-8445-667788990022",
					"procedure_code": "COR-01",
					"procedure_name": "Corona Porcelana",
					"tooth_fdi": "14",
					"professional_fee_cents": 60000,
					"lab_cost_cents": 20000,
					"discount_cents": 0,
					"estimated_duration_minutes": 45
				}]
			}
		]
	}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/treatment-plans", bytes.NewBufferString(planPayload))
	reqCreate.Header.Set("X-Actor-ID", "dent-1")
	reqCreate.Header.Set("X-Actor-Role", "dentist")
	reqCreate.Header.Set("X-MFA-Verified", "true")
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var created domain.TreatmentPlan
	_ = json.Unmarshal(recCreate.Body.Bytes(), &created)

	// 2. Partial approval -> 200 OK (S2)
	approvePayload := `{
		"mode": "PARTIAL",
		"approved_item_ids": ["55667788-1122-4333-8445-667788990011"],
		"patient_acceptance_ref": "FIRMA-PACIENTE-01"
	}`
	reqApprove := httptest.NewRequest(http.MethodPost, "/api/v1/treatment-plans/"+created.ID+"/approve", bytes.NewBufferString(approvePayload))
	reqApprove.Header.Set("X-Actor-ID", "rec-1")
	reqApprove.Header.Set("X-Actor-Role", "receptionist")
	reqApprove.Header.Set("X-MFA-Verified", "true")
	recApprove := httptest.NewRecorder()
	mux.ServeHTTP(recApprove, reqApprove)
	if recApprove.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on partial approve, got %d: %s", recApprove.Code, recApprove.Body.String())
	}

	// Delete uncompleted item-2 -> 204 No Content
	reqDel2 := httptest.NewRequest(http.MethodDelete, "/api/v1/treatment-plans/"+created.ID+"/items/55667788-1122-4333-8445-667788990022", nil)
	reqDel2.Header.Set("X-Actor-Role", "dentist")
	reqDel2.Header.Set("X-MFA-Verified", "true")
	recDel2 := httptest.NewRecorder()
	mux.ServeHTTP(recDel2, reqDel2)
	if recDel2.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on uncompleted item delete, got %d", recDel2.Code)
	}

	// 3. Complete item -> 200 OK & subsequent DELETE -> 409 Conflict (S4 / RN-01)
	reqComplete := httptest.NewRequest(http.MethodPost, "/api/v1/treatment-plans/"+created.ID+"/items/55667788-1122-4333-8445-667788990011/complete", nil)
	reqComplete.Header.Set("X-Actor-ID", "dent-1")
	reqComplete.Header.Set("X-Actor-Role", "dentist")
	reqComplete.Header.Set("X-MFA-Verified", "true")
	recComplete := httptest.NewRecorder()
	mux.ServeHTTP(recComplete, reqComplete)
	if recComplete.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on complete item, got %d: %s", recComplete.Code, recComplete.Body.String())
	}

	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/treatment-plans/"+created.ID+"/items/55667788-1122-4333-8445-667788990011", nil)
	reqDel.Header.Set("X-Actor-ID", "dent-1")
	reqDel.Header.Set("X-Actor-Role", "dentist")
	reqDel.Header.Set("X-MFA-Verified", "true")
	recDel := httptest.NewRecorder()
	mux.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on deleting REALIZADO item, got %d", recDel.Code)
	}

	// 4. GET /schedulable-tasks -> 200 OK (S5)
	reqTasks := httptest.NewRequest(http.MethodGet, "/api/v1/patients/p-1/schedulable-tasks", nil)
	reqTasks.Header.Set("X-Actor-ID", "rec-1")
	reqTasks.Header.Set("X-Actor-Role", "receptionist")
	reqTasks.Header.Set("X-MFA-Verified", "true")
	recTasks := httptest.NewRecorder()
	mux.ServeHTTP(recTasks, reqTasks)
	if recTasks.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list schedulable tasks, got %d", recTasks.Code)
	}
}

type planListItem struct {
	ID                 string `json:"id"`
	PatientID          string `json:"patient_id"`
	Title              string `json:"title"`
	Status             string `json:"status"`
	TotalApprovedCents int64  `json:"total_approved_cents"`
	TotalPlannedCents  int64  `json:"total_planned_cents"`
	Items              []struct {
		ProcedureCode string `json:"procedure_code"`
		ProcedureName string `json:"procedure_name"`
		NetCents      int64  `json:"net_cents"`
		Status        string `json:"status"`
	} `json:"items"`
}

type planListResponse struct {
	Items []planListItem `json:"items"`
	Total int            `json:"total"`
}

// TestTreatmentHandler_ListPlans covers S1/T4: the global and per-patient
// treatment plan listings answer {items,total}, apply the filters and the
// pagination contract, never leak the plans of another patient and leave an
// EXPORT audit trail (RNF-06).
func TestTreatmentHandler_ListPlans(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 30, 0, 0, time.UTC)
	repo := postgres.NewTreatmentRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, nil, nil)
	svc := application.NewTreatmentService(repo, auditSvc, fixedClock{now: now})
	handler := treatmenthttp.NewTreatmentHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	seed := []*domain.TreatmentPlan{
		{
			ID: "plan-p1-a", PatientID: "p-1", Title: "Rehabilitación superior", Currency: "USD", Status: "DRAFT",
			TotalNetCents: 90000, ApprovedTotalCents: 0, CreatedAt: now.Add(3 * time.Hour), UpdatedAt: now.Add(3 * time.Hour),
			Phases: []domain.TreatmentPhase{{
				PhaseNumber: 1, Name: "Fase 1", SubtotalNetCents: 90000,
				Items: []domain.TreatmentItem{{
					ID: "item-p1-a-1", PhaseNumber: 1, ProcedureCode: "ENDO-01", ProcedureName: "Endodoncia",
					NetCents: 90000, Status: "PENDING_APPROVAL",
				}},
			}},
		},
		{
			ID: "plan-p1-b", PatientID: "p-1", Title: "Coronas estéticas", Currency: "USD", Status: "APPROVED_PARTIAL",
			TotalNetCents: 50000, ApprovedTotalCents: 50000, CreatedAt: now.Add(2 * time.Hour), UpdatedAt: now.Add(2 * time.Hour),
			Phases: []domain.TreatmentPhase{{
				PhaseNumber: 1, Name: "Fase 1", SubtotalNetCents: 50000,
				Items: []domain.TreatmentItem{{
					ID: "item-p1-b-1", PhaseNumber: 1, ProcedureCode: "COR-01", ProcedureName: "Corona Porcelana",
					NetCents: 50000, Status: "APPROVED",
				}},
			}},
		},
		{
			ID: "plan-p2-a", PatientID: "p-2", Title: "Implantes inferiores", Currency: "USD", Status: "COMPLETED",
			TotalNetCents: 120000, ApprovedTotalCents: 120000, CreatedAt: now.Add(time.Hour), UpdatedAt: now.Add(time.Hour),
			Phases: []domain.TreatmentPhase{{
				PhaseNumber: 1, Name: "Fase 1", SubtotalNetCents: 120000,
				Items: []domain.TreatmentItem{{
					ID: "item-p2-a-1", PhaseNumber: 1, ProcedureCode: "IMP-01", ProcedureName: "Implante",
					NetCents: 120000, Status: "REALIZADO",
				}},
			}},
		},
	}
	for _, plan := range seed {
		if err := repo.SavePlan(context.Background(), plan); err != nil {
			t.Fatalf("failed to seed plan %s: %v", plan.ID, err)
		}
	}

	doGet := func(path, role string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Actor-ID", "dent-1")
		req.Header.Set("X-Actor-Role", role)
		req.Header.Set("X-Source-IP", "198.51.100.7")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	// 1. The auditor role has no access to treatment plans.
	recForbidden := doGet("/api/v1/treatment-plans", "auditor")
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for the auditor role, got %d: %s", recForbidden.Code, recForbidden.Body.String())
	}
	var forbiddenBody map[string]string
	_ = json.Unmarshal(recForbidden.Body.Bytes(), &forbiddenBody)
	if forbiddenBody["code"] != "FORBIDDEN_ROLE" {
		t.Fatalf("expected FORBIDDEN_ROLE, got %q", forbiddenBody["code"])
	}

	// 2. The global listing answers {items,total} with the derived amounts.
	rec := doGet("/api/v1/treatment-plans", "dentist")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list plans, got %d: %s", rec.Code, rec.Body.String())
	}
	var list planListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode the listing: %v", err)
	}
	if list.Total != 3 || len(list.Items) != 3 {
		t.Fatalf("expected 3 plans, got total=%d len=%d", list.Total, len(list.Items))
	}
	if list.Items[0].ID != "plan-p1-a" {
		t.Fatalf("expected the newest plan first, got %q", list.Items[0].ID)
	}
	if list.Items[0].TotalPlannedCents != 90000 || list.Items[0].TotalApprovedCents != 0 {
		t.Fatalf("unexpected derived amounts: %+v", list.Items[0])
	}
	if list.Items[2].TotalApprovedCents != 120000 {
		t.Fatalf("expected the approved amount of the completed plan, got %+v", list.Items[2])
	}

	// 3. The patient_id filter never leaks the plans of another patient.
	recPatient := doGet("/api/v1/treatment-plans?patient_id=p-2", "dentist")
	if recPatient.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the patient filter, got %d", recPatient.Code)
	}
	var byPatient planListResponse
	_ = json.Unmarshal(recPatient.Body.Bytes(), &byPatient)
	if byPatient.Total != 1 || byPatient.Items[0].PatientID != "p-2" {
		t.Fatalf("expected only the plans of p-2, got %+v", byPatient)
	}

	// 4. The status filter accepts the public vocabulary.
	recStatus := doGet("/api/v1/treatment-plans?status=COMPLETED", "dentist")
	if recStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the status filter, got %d", recStatus.Code)
	}
	var byStatus planListResponse
	_ = json.Unmarshal(recStatus.Body.Bytes(), &byStatus)
	if byStatus.Total != 1 || byStatus.Items[0].Status != "COMPLETED" {
		t.Fatalf("expected only the completed plan, got %+v", byStatus)
	}

	// 5. The q filter searches the plan title.
	recQuery := doGet("/api/v1/treatment-plans?q=coronas", "dentist")
	if recQuery.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the q filter, got %d", recQuery.Code)
	}
	var byQuery planListResponse
	_ = json.Unmarshal(recQuery.Body.Bytes(), &byQuery)
	if byQuery.Total != 1 || byQuery.Items[0].ID != "plan-p1-b" {
		t.Fatalf("expected only the matching plan, got %+v", byQuery)
	}

	// 6. An unknown status is rejected.
	recBadStatus := doGet("/api/v1/treatment-plans?status=WHATEVER", "dentist")
	if recBadStatus.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on an unknown status, got %d: %s", recBadStatus.Code, recBadStatus.Body.String())
	}

	// 7. Pagination: total is the total before limit/offset.
	recPage := doGet("/api/v1/treatment-plans?limit=1&offset=1", "dentist")
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on pagination, got %d", recPage.Code)
	}
	var page planListResponse
	_ = json.Unmarshal(recPage.Body.Bytes(), &page)
	if page.Total != 3 {
		t.Fatalf("expected the total before pagination to be 3, got %d", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "plan-p1-b" {
		t.Fatalf("expected the second newest plan, got %+v", page.Items)
	}

	// 8. The hard ceiling of 200 items per page is enforced.
	for i := 0; i < 210; i++ {
		bulk := &domain.TreatmentPlan{
			ID: fmt.Sprintf("plan-bulk-%03d", i), PatientID: "p-3", Title: fmt.Sprintf("Plan masivo %03d", i),
			Currency: "USD", Status: "DRAFT", CreatedAt: now.Add(-time.Duration(i+1) * time.Minute), UpdatedAt: now,
		}
		if err := repo.SavePlan(context.Background(), bulk); err != nil {
			t.Fatalf("failed to seed bulk plan %d: %v", i, err)
		}
	}
	recCeiling := doGet("/api/v1/treatment-plans?limit=1000", "dentist")
	if recCeiling.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the limit ceiling, got %d", recCeiling.Code)
	}
	var ceiling planListResponse
	_ = json.Unmarshal(recCeiling.Body.Bytes(), &ceiling)
	if ceiling.Total != 213 {
		t.Fatalf("expected 213 plans in total, got %d", ceiling.Total)
	}
	if len(ceiling.Items) != 200 {
		t.Fatalf("expected the limit ceiling of 200 items, got %d", len(ceiling.Items))
	}

	// 9. The per patient listing returns the plans of that patient with items.
	recScoped := doGet("/api/v1/patients/p-1/treatment-plans", "dentist")
	if recScoped.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the per patient listing, got %d: %s", recScoped.Code, recScoped.Body.String())
	}
	var scoped planListResponse
	if err := json.Unmarshal(recScoped.Body.Bytes(), &scoped); err != nil {
		t.Fatalf("failed to decode the per patient listing: %v", err)
	}
	if scoped.Total != 2 || len(scoped.Items) != 2 {
		t.Fatalf("expected the 2 plans of p-1, got total=%d len=%d", scoped.Total, len(scoped.Items))
	}
	for _, item := range scoped.Items {
		if item.PatientID != "p-1" {
			t.Fatalf("expected only the plans of p-1, got %+v", item)
		}
		if len(item.Items) != 1 {
			t.Fatalf("expected the plan items to be listed, got %+v", item)
		}
		if item.Items[0].ProcedureCode == "" || item.Items[0].ProcedureName == "" {
			t.Fatalf("expected the procedure code and name, got %+v", item.Items[0])
		}
		if item.Items[0].Status == "" {
			t.Fatalf("expected the approval status of the item, got %+v", item.Items[0])
		}
	}

	// 10. Every listing leaves an EXPORT audit trail (RNF-06).
	entries, totalEntries, err := auditRepo.List(context.Background(), auditapp.AuditFilter{Action: "EXPORT", ResourceType: "TREATMENT_PLAN"})
	if err != nil {
		t.Fatalf("expected to read the audit trail, got %v", err)
	}
	if totalEntries == 0 {
		t.Fatal("expected the listing to leave an EXPORT audit trail")
	}
	resourceIDs := map[string]bool{}
	for _, entry := range entries {
		resourceIDs[entry.ResourceID] = true
		if entry.ActorID != "dent-1" || entry.ActorRole != "dentist" {
			t.Fatalf("expected the actor to be recorded, got %+v", entry)
		}
		if entry.SourceIP != "198.51.100.7" {
			t.Fatalf("expected the source ip to be recorded, got %q", entry.SourceIP)
		}
		if entry.TimestampISO8601 == "" {
			t.Fatal("expected an ISO-8601 timestamp in the audit trail")
		}
	}
	if !resourceIDs["treatment-plans"] {
		t.Fatalf("expected the global collection to be audited, got %v", resourceIDs)
	}
	if !resourceIDs["p-1"] {
		t.Fatalf("expected the per patient listing to be audited, got %v", resourceIDs)
	}
}

// failingRecorder simulates an unavailable audit storage.
type failingRecorder struct{}

func (failingRecorder) Record(context.Context, auditapp.RecordInput) (*auditdom.AuditEntry, error) {
	return nil, errors.New("audit storage unavailable")
}

// TestTreatmentHandler_ListPlansSurvivesAuditFailure covers RNF-06: a failing
// audit recorder must not invalidate the 200 response of a listing, but the
// failure has to be logged instead of being silently discarded.
func TestTreatmentHandler_ListPlansSurvivesAuditFailure(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 30, 0, 0, time.UTC)
	repo := postgres.NewTreatmentRepository()
	svc := application.NewTreatmentService(repo, failingRecorder{}, fixedClock{now: now})
	handler := treatmenthttp.NewTreatmentHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/treatment-plans", nil)
	req.Header.Set("X-Actor-ID", "dent-1")
	req.Header.Set("X-Actor-Role", "dentist")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK even when the audit recorder fails, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "audit trail could not be recorded") {
		t.Fatalf("expected the audit failure to be logged, got %q", logs.String())
	}
}
