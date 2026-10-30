package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
