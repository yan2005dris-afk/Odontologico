package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	patienthttp "github.com/sigo-dental/clinical-agenda/internal/patients/adapters/http"
	"github.com/sigo-dental/clinical-agenda/internal/patients/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/patients/application"
	"github.com/sigo-dental/clinical-agenda/internal/patients/domain"
)

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time { return f.now }

func TestPatientHandler_Endpoints(t *testing.T) {
	now := time.Date(2026, 10, 30, 15, 0, 0, 0, time.UTC)
	repo := postgres.NewPatientRepository()
	svc := application.NewPatientService(repo, nil, fixedClock{now: now})
	handler := patienthttp.NewPatientHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 0. Bad JSON & Missing MFA
	reqNoMFA := httptest.NewRequest(http.MethodGet, "/api/v1/patients", nil)
	recNoMFA := httptest.NewRecorder()
	mux.ServeHTTP(recNoMFA, reqNoMFA)
	if recNoMFA.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without MFA, got %d", recNoMFA.Code)
	}
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/api/v1/patients", bytes.NewBufferString("{bad"))
	reqBadJSON.Header.Set("X-Actor-Role", "receptionist")
	reqBadJSON.Header.Set("X-MFA-Verified", "true")
	recBadJSON := httptest.NewRecorder()
	mux.ServeHTTP(recBadJSON, reqBadJSON)
	if recBadJSON.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad JSON, got %d", recBadJSON.Code)
	}

	// 1. Minor without legal guardian -> 422 LEGAL_GUARDIAN_REQUIRED_FOR_MINOR (S1)
	minorNoGuardian := `{
		"document_type": "DNI",
		"document_number": "55000111",
		"first_name": "Lucas",
		"last_name": "Pérez",
		"birth_date": "2015-05-10",
		"sex": "M",
		"phone": "+5491100001111",
		"emergency_contact": {"full_name": "Ana Pérez", "phone": "+5491100001111", "relationship": "Mother"}
	}`
	reqMinor := httptest.NewRequest(http.MethodPost, "/api/v1/patients", bytes.NewBufferString(minorNoGuardian))
	reqMinor.Header.Set("X-Actor-ID", "rec-1")
	reqMinor.Header.Set("X-Actor-Role", "receptionist")
	reqMinor.Header.Set("X-MFA-Verified", "true")
	recMinor := httptest.NewRecorder()
	mux.ServeHTTP(recMinor, reqMinor)
	if recMinor.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for minor without guardian, got %d: %s", recMinor.Code, recMinor.Body.String())
	}

	// 2. Create adult patient -> 201 Created
	adultBody := `{
		"document_type": "DNI",
		"document_number": "40123987",
		"first_name": "Martín",
		"last_name": "Echeverría",
		"birth_date": "1982-04-15",
		"sex": "M",
		"phone": "+5491144556677",
		"emergency_contact": {"full_name": "Laura Echeverría", "phone": "+5491144556677", "relationship": "Spouse"},
		"insurance_plan": {"provider_name": "OSDE", "plan_name": "310", "member_number": "610293847"}
	}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/patients", bytes.NewBufferString(adultBody))
	reqCreate.Header.Set("X-Actor-ID", "rec-1")
	reqCreate.Header.Set("X-Actor-Role", "receptionist")
	reqCreate.Header.Set("X-MFA-Verified", "true")
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var created domain.Patient
	_ = json.Unmarshal(recCreate.Body.Bytes(), &created)

	// 2b. GET & PUT /patients/{id}
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/patients/"+created.ID, nil)
	reqGet.Header.Set("X-Actor-Role", "dentist")
	reqGet.Header.Set("X-MFA-Verified", "true")
	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET patient, got %d", recGet.Code)
	}

	reqUpdate := httptest.NewRequest(http.MethodPut, "/api/v1/patients/"+created.ID, bytes.NewBufferString(adultBody))
	reqUpdate.Header.Set("X-Actor-Role", "receptionist")
	reqUpdate.Header.Set("X-MFA-Verified", "true")
	recUpdate := httptest.NewRecorder()
	mux.ServeHTTP(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PUT patient, got %d", recUpdate.Code)
	}

	reqNotFound := httptest.NewRequest(http.MethodGet, "/api/v1/patients/missing-id", nil)
	reqNotFound.Header.Set("X-Actor-Role", "dentist")
	reqNotFound.Header.Set("X-MFA-Verified", "true")
	recNotFound := httptest.NewRecorder()
	mux.ServeHTTP(recNotFound, reqNotFound)
	if recNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on missing patient, got %d", recNotFound.Code)
	}

	// 3. Duplicate document -> 409 Conflict
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/patients", bytes.NewBufferString(adultBody))
	reqDup.Header.Set("X-Actor-ID", "rec-1")
	reqDup.Header.Set("X-Actor-Role", "receptionist")
	reqDup.Header.Set("X-MFA-Verified", "true")
	recDup := httptest.NewRecorder()
	mux.ServeHTTP(recDup, reqDup)
	if recDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", recDup.Code)
	}

	// 4. Receptionist attempting PUT /anamnesis -> 403 Forbidden
	anamBody := `{
		"drug_allergies": [{"substance": "Penicilina", "reaction": "Anafilaxia", "severity": "CRITICAL"}],
		"coagulopathies": [{"condition": "FA", "medication": "Warfarina", "inr_last_value": 2.8, "is_anticoagulated": true}]
	}`
	reqAnamRec := httptest.NewRequest(http.MethodPut, "/api/v1/patients/"+created.ID+"/anamnesis", bytes.NewBufferString(anamBody))
	reqAnamRec.Header.Set("X-Actor-ID", "rec-1")
	reqAnamRec.Header.Set("X-Actor-Role", "receptionist")
	reqAnamRec.Header.Set("X-MFA-Verified", "true")
	recAnamRec := httptest.NewRecorder()
	mux.ServeHTTP(recAnamRec, reqAnamRec)
	if recAnamRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for receptionist on anamnesis, got %d", recAnamRec.Code)
	}

	// 5. Dentist PUT /anamnesis -> 200 OK with derived critical alerts
	reqAnamDent := httptest.NewRequest(http.MethodPut, "/api/v1/patients/"+created.ID+"/anamnesis", bytes.NewBufferString(anamBody))
	reqAnamDent.Header.Set("X-Actor-ID", "dent-1")
	reqAnamDent.Header.Set("X-Actor-Role", "dentist")
	reqAnamDent.Header.Set("X-MFA-Verified", "true")
	recAnamDent := httptest.NewRecorder()
	mux.ServeHTTP(recAnamDent, reqAnamDent)
	if recAnamDent.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for dentist on anamnesis, got %d: %s", recAnamDent.Code, recAnamDent.Body.String())
	}

	// 6. GET /header & GET /patients?q=40123987 -> 200 OK
	reqHeader := httptest.NewRequest(http.MethodGet, "/api/v1/patients/"+created.ID+"/header", nil)
	reqHeader.Header.Set("X-Actor-ID", "dent-1")
	reqHeader.Header.Set("X-Actor-Role", "dentist")
	reqHeader.Header.Set("X-MFA-Verified", "true")
	recHeader := httptest.NewRecorder()
	mux.ServeHTTP(recHeader, reqHeader)
	if recHeader.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on header, got %d", recHeader.Code)
	}

	reqSearch := httptest.NewRequest(http.MethodGet, "/api/v1/patients?q=Echeverr", nil)
	reqSearch.Header.Set("X-Actor-ID", "dent-1")
	reqSearch.Header.Set("X-Actor-Role", "dentist")
	reqSearch.Header.Set("X-MFA-Verified", "true")
	recSearch := httptest.NewRecorder()
	mux.ServeHTTP(recSearch, reqSearch)
	if recSearch.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on search, got %d", recSearch.Code)
	}
}
