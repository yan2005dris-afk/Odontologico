package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	schedulinghttp "github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/http"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/messaging"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/application"
	"github.com/sigo-dental/clinical-agenda/internal/scheduling/domain"
)

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time { return f.now }

func TestSchedulingHandler_Endpoints(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	repo := postgres.NewSchedulingRepository()
	dispatcher := messaging.NewNotificationDispatcher()
	svc := application.NewSchedulingService(repo, dispatcher, nil, fixedClock{now: now})
	handler := schedulinghttp.NewSchedulingHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	for _, r := range []struct {
		m string
		p string
	}{
		{http.MethodPost, "/api/v1/scheduling/appointments"},
		{http.MethodPatch, "/api/v1/scheduling/appointments/a-1/reschedule"},
		{http.MethodPost, "/api/v1/scheduling/appointments/a-1/transition"},
		{http.MethodPost, "/api/v1/scheduling/webhooks/patient-reply"},
	} {
		reqBad := httptest.NewRequest(r.m, r.p, bytes.NewBufferString("{bad"))
		recBad := httptest.NewRecorder()
		mux.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on %s %s, got %d", r.m, r.p, recBad.Code)
		}
	}

	reqNF := httptest.NewRequest(http.MethodPost, "/api/v1/scheduling/appointments/missing/transition", bytes.NewBufferString(`{"target_status":"CONFIRMADO"}`))
	recNF := httptest.NewRecorder()
	mux.ServeHTTP(recNF, reqNF)
	if recNF.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on missing appointment, got %d", recNF.Code)
	}

	// 1. Create appointment on sillon-01 [10:00, 10:45) -> 201 Created
	appt1Body := `{
		"patient_id": "2c5ea4c0-4067-11e9-8bad-9b1deb4d3b7d",
		"dentist_id": "dentist-1",
		"chair_id": "sillon-01-uuid",
		"clinic_id": "sede-central-uuid",
		"start_at": "2026-11-05T10:00:00Z",
		"end_at": "2026-11-05T10:45:00Z",
		"reason": "Endodoncia pieza 16",
		"notification_channels": ["WHATSAPP", "EMAIL"]
	}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/scheduling/appointments", bytes.NewBufferString(appt1Body))
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var created domain.Appointment
	_ = json.Unmarshal(rec1.Body.Bytes(), &created)

	// Reschedule endpoint -> 200 OK
	reqResched := httptest.NewRequest(http.MethodPatch, "/api/v1/scheduling/appointments/"+created.ID+"/reschedule", bytes.NewBufferString(`{
		"chair_id": "sillon-01-uuid",
		"start_at": "2026-11-05T10:00:00Z",
		"end_at": "2026-11-05T10:45:00Z"
	}`))
	recResched := httptest.NewRecorder()
	mux.ServeHTTP(recResched, reqResched)
	if recResched.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on reschedule, got %d", recResched.Code)
	}

	// 2. Overlapping appointment on same chair sillon-01 [10:30, 11:00) with dentist-2 -> 409 CHAIR_OVERLAP_CONFLICT (S1 / RN-03)
	apptOverlapBody := `{
		"patient_id": "patient-2",
		"dentist_id": "dentist-2",
		"chair_id": "sillon-01-uuid",
		"clinic_id": "sede-central-uuid",
		"start_at": "2026-11-05T10:30:00Z",
		"end_at": "2026-11-05T11:00:00Z",
		"reason": "Consulta"
	}`
	reqOverlap := httptest.NewRequest(http.MethodPost, "/api/v1/scheduling/appointments", bytes.NewBufferString(apptOverlapBody))
	recOverlap := httptest.NewRecorder()
	mux.ServeHTTP(recOverlap, reqOverlap)
	if recOverlap.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on chair overlap, got %d: %s", recOverlap.Code, recOverlap.Body.String())
	}

	// Dentist overlap across different chairs -> 409 DENTIST_OVERLAP_CONFLICT
	dentOverlapBody := `{
		"patient_id": "patient-2",
		"dentist_id": "dentist-1",
		"chair_id": "sillon-02-uuid",
		"clinic_id": "sede-central-uuid",
		"start_at": "2026-11-05T10:30:00Z",
		"end_at": "2026-11-05T11:00:00Z",
		"reason": "Consulta"
	}`
	reqDentOverlap := httptest.NewRequest(http.MethodPost, "/api/v1/scheduling/appointments", bytes.NewBufferString(dentOverlapBody))
	recDentOverlap := httptest.NewRecorder()
	mux.ServeHTTP(recDentOverlap, reqDentOverlap)
	if recDentOverlap.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on dentist overlap, got %d", recDentOverlap.Code)
	}

	// 3. Query calendar by day and chair -> 200 OK (S2) & invalid date -> 400
	reqCal := httptest.NewRequest(http.MethodGet, "/api/v1/scheduling/appointments?view=day&date=2026-11-05&chair_id=sillon-01-uuid", nil)
	recCal := httptest.NewRecorder()
	mux.ServeHTTP(recCal, reqCal)
	if recCal.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on calendar query, got %d", recCal.Code)
	}
	reqBadDate := httptest.NewRequest(http.MethodGet, "/api/v1/scheduling/appointments?date=bad", nil)
	recBadDate := httptest.NewRecorder()
	mux.ServeHTTP(recBadDate, reqBadDate)
	if recBadDate.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad date, got %d", recBadDate.Code)
	}

	// 4. Patient reply webhook confirms appointment -> 200 OK (S5)
	reqWebhook := httptest.NewRequest(http.MethodPost, "/api/v1/scheduling/webhooks/patient-reply", bytes.NewBufferString(`{
		"appointment_id": "`+created.ID+`",
		"reply_action": "CONFIRM",
		"channel": "WHATSAPP"
	}`))
	recWebhook := httptest.NewRecorder()
	mux.ServeHTTP(recWebhook, reqWebhook)
	if recWebhook.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on webhook, got %d", recWebhook.Code)
	}

	// 5. Invalid state transition CONFIRMADO -> ATENDIDO (skipping EN_SALA_DE_ESPERA / EN_ATENCION) -> 422 (S3)
	reqBadTrans := httptest.NewRequest(http.MethodPost, "/api/v1/scheduling/appointments/"+created.ID+"/transition", bytes.NewBufferString(`{
		"target_status": "ATENDIDO"
	}`))
	recBadTrans := httptest.NewRecorder()
	mux.ServeHTTP(recBadTrans, reqBadTrans)
	if recBadTrans.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on invalid transition, got %d", recBadTrans.Code)
	}
}
