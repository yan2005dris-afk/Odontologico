package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	clinicalhttp "github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/http"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time { return f.now }

func TestOdontogramAndPeriodontogramHandler(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 10, 0, 0, time.UTC)
	odoRepo := postgres.NewOdontogramRepository()
	perioRepo := postgres.NewPeriodontogramRepository()
	odoSvc := application.NewOdontogramService(odoRepo, nil, fixedClock{now: now})
	perioSvc := application.NewPeriodontogramService(perioRepo, nil, fixedClock{now: now})

	handler := clinicalhttp.NewOdontogramHandler(odoSvc, perioSvc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 0. Missing MFA -> 403 & Bad JSON -> 400
	reqNoMFA := httptest.NewRequest(http.MethodGet, "/api/v1/patients/p-1/odontogram", nil)
	recNoMFA := httptest.NewRecorder()
	mux.ServeHTTP(recNoMFA, reqNoMFA)
	if recNoMFA.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without MFA, got %d", recNoMFA.Code)
	}
	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/patients/p-1/odontogram/findings"},
		{http.MethodPut, "/api/v1/patients/p-1/odontogram/findings/f-1"},
		{http.MethodPost, "/api/v1/patients/p-1/odontogram/findings/f-1/addendum"},
		{http.MethodPost, "/api/v1/patients/p-1/periodontograms"},
	} {
		reqBad := httptest.NewRequest(route.method, route.path, bytes.NewBufferString("{bad"))
		reqBad.Header.Set("X-Actor-Role", "dentist")
		reqBad.Header.Set("X-MFA-Verified", "true")
		recBad := httptest.NewRecorder()
		mux.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on %s %s, got %d", route.method, route.path, recBad.Code)
		}
	}

	// Not found finding -> 404
	reqNF := httptest.NewRequest(http.MethodDelete, "/api/v1/patients/p-1/odontogram/findings/missing", nil)
	reqNF.Header.Set("X-Actor-Role", "dentist")
	reqNF.Header.Set("X-MFA-Verified", "true")
	recNF := httptest.NewRecorder()
	mux.ServeHTTP(recNF, reqNF)
	if recNF.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on missing finding, got %d", recNF.Code)
	}

	// 1. Invalid tooth code 19 -> 422 INVALID_TOOTH_CODE (S1)
	reqBadTooth := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/odontogram/findings", bytes.NewBufferString(`{
		"tooth_code":"19","notation_system":"FDI","surface":"MESIAL","finding_type":"CARIES","status":"DIAGNOSED"
	}`))
	reqBadTooth.Header.Set("X-Actor-Role", "dentist")
	reqBadTooth.Header.Set("X-MFA-Verified", "true")
	recBadTooth := httptest.NewRecorder()
	mux.ServeHTTP(recBadTooth, reqBadTooth)
	if recBadTooth.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for tooth 19, got %d", recBadTooth.Code)
	}

	// Invalid surface -> 422 VALIDATION_ERROR
	reqBadSurf := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/odontogram/findings", bytes.NewBufferString(`{
		"tooth_code":"16","notation_system":"FDI","surface":"INVALID","finding_type":"CARIES","status":"DIAGNOSED"
	}`))
	reqBadSurf.Header.Set("X-Actor-Role", "dentist")
	reqBadSurf.Header.Set("X-MFA-Verified", "true")
	recBadSurf := httptest.NewRecorder()
	mux.ServeHTTP(recBadSurf, reqBadSurf)
	if recBadSurf.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid surface, got %d", recBadSurf.Code)
	}

	// 2. Record DIAGNOSED finding -> 201 Created, then PUT to REALIZADO -> 200 OK (S2)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/odontogram/findings", bytes.NewBufferString(`{
		"tooth_code":"16","notation_system":"FDI","surface":"OCLUSAL_INCISAL","finding_type":"EXISTING_RESTORATION","status":"DIAGNOSED"
	}`))
	reqCreate.Header.Set("X-Actor-Role", "dentist")
	reqCreate.Header.Set("X-MFA-Verified", "true")
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var created domain.OdontogramFinding
	_ = json.Unmarshal(recCreate.Body.Bytes(), &created)

	reqUpdate := httptest.NewRequest(http.MethodPut, "/api/v1/patients/p-1/odontogram/findings/"+created.ID, bytes.NewBufferString(`{"status":"REALIZADO","notes":"Completed"}`))
	reqUpdate.Header.Set("X-Actor-Role", "dentist")
	reqUpdate.Header.Set("X-MFA-Verified", "true")
	recUpdate := httptest.NewRecorder()
	mux.ServeHTTP(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PUT finding, got %d", recUpdate.Code)
	}

	// 3. Attempt DELETE on REALIZADO finding -> 409 Conflict CLINICAL_RECORD_IMMUTABLE (S3 / RN-01)
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/patients/p-1/odontogram/findings/"+created.ID, nil)
	reqDel.Header.Set("X-Actor-Role", "dentist")
	reqDel.Header.Set("X-MFA-Verified", "true")
	recDel := httptest.NewRecorder()
	mux.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on deleting REALIZADO finding, got %d", recDel.Code)
	}

	// 4. Addendum on REALIZADO finding -> 201 Created (S3)
	reqAdd := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/odontogram/findings/"+created.ID+"/addendum", bytes.NewBufferString(`{"note":"Adenda clínica firmada"}`))
	reqAdd.Header.Set("X-Actor-Role", "dentist")
	reqAdd.Header.Set("X-MFA-Verified", "true")
	recAdd := httptest.NewRecorder()
	mux.ServeHTTP(recAdd, reqAdd)
	if recAdd.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on addendum, got %d", recAdd.Code)
	}

	// 5. POST & GET /periodontograms -> 201 Created & 200 OK (S4)
	perioBody := `{
		"teeth": [{
			"tooth_fdi": "16",
			"probing_depth_mm": [3, 2, 4, 3, 2, 3],
			"gingival_margin_mm": [0, -1, -1, 0, 0, -1],
			"bleeding_on_probing": [false, false, true, false, false, false],
			"furcation_grade": 1,
			"mobility_grade": 0
		}]
	}`
	reqPerio := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/periodontograms", bytes.NewBufferString(perioBody))
	reqPerio.Header.Set("X-Actor-Role", "dentist")
	reqPerio.Header.Set("X-MFA-Verified", "true")
	recPerio := httptest.NewRecorder()
	mux.ServeHTTP(recPerio, reqPerio)
	if recPerio.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on periodontogram, got %d: %s", recPerio.Code, recPerio.Body.String())
	}

	reqListPerio := httptest.NewRequest(http.MethodGet, "/api/v1/patients/p-1/periodontograms", nil)
	reqListPerio.Header.Set("X-Actor-Role", "dentist")
	reqListPerio.Header.Set("X-MFA-Verified", "true")
	recListPerio := httptest.NewRecorder()
	mux.ServeHTTP(recListPerio, reqListPerio)
	if recListPerio.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list periodontograms, got %d", recListPerio.Code)
	}

	// 6. GET /odontogram -> 200 OK (S5)
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/patients/p-1/odontogram", nil)
	reqGet.Header.Set("X-Actor-Role", "dentist")
	reqGet.Header.Set("X-MFA-Verified", "true")
	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get odontogram, got %d", recGet.Code)
	}
}
