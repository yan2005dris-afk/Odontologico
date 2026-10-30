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
	"github.com/sigo-dental/clinical-agenda/internal/clinical/adapters/s3"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/application"
	"github.com/sigo-dental/clinical-agenda/internal/clinical/domain"
)

type stepClock struct {
	now time.Time
}

func (s *stepClock) Now() time.Time { return s.now }

func TestSOEPConsentHandler_Endpoints(t *testing.T) {
	clock := &stepClock{now: time.Date(2026, 10, 30, 16, 0, 0, 0, time.UTC)}
	soepRepo := postgres.NewSOEPRepository()
	consentRepo := postgres.NewConsentRepository()
	blobStore := s3.NewConsentBlobStore()

	soepSvc := application.NewSOEPService(soepRepo, nil, clock, 24*time.Hour)
	consentSvc := application.NewConsentService(consentRepo, blobStore, nil, clock)
	handler := clinicalhttp.NewSOEPConsentHandler(soepSvc, consentSvc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/patients/p-1/soep-notes"},
		{http.MethodPut, "/api/v1/patients/p-1/soep-notes/n-1"},
		{http.MethodPost, "/api/v1/patients/p-1/soep-notes/n-1/addenda"},
		{http.MethodPost, "/api/v1/patients/p-1/consents"},
		{http.MethodPost, "/api/v1/patients/p-1/consents/c-1/sign"},
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

	// 1. Receptionist attempting POST /soep-notes -> 403 Forbidden
	soepPayload := `{
		"subjective": "Paciente refiere dolor pulsátil en cuadrante superior derecho.",
		"objective": "Caries profunda oclusal en pieza 16, percusión positiva.",
		"assessment": "Pulpitis irreversible sintomática en pieza 16.",
		"plan": "Biopulpectomía en pieza 16 bajo anestesia infiltrativa."
	}`
	reqRec := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/soep-notes", bytes.NewBufferString(soepPayload))
	reqRec.Header.Set("X-Actor-ID", "rec-1")
	reqRec.Header.Set("X-Actor-Role", "receptionist")
	reqRec.Header.Set("X-MFA-Verified", "true")
	recRec := httptest.NewRecorder()
	mux.ServeHTTP(recRec, reqRec)
	if recRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for receptionist on SOEP note, got %d", recRec.Code)
	}

	// 2. Dentist creates SOEP note -> 201 Created & lists -> 200 OK (S1)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/soep-notes", bytes.NewBufferString(soepPayload))
	reqCreate.Header.Set("X-Actor-ID", "dent-1")
	reqCreate.Header.Set("X-Actor-Role", "dentist")
	reqCreate.Header.Set("X-MFA-Verified", "true")
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on SOEP note, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var note domain.SOEPNote
	_ = json.Unmarshal(recCreate.Body.Bytes(), &note)

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/patients/p-1/soep-notes", nil)
	reqList.Header.Set("X-Actor-Role", "dentist")
	reqList.Header.Set("X-MFA-Verified", "true")
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list SOEP notes, got %d", recList.Code)
	}

	// PUT within 24h succeeds -> 200 OK
	reqPutOK := httptest.NewRequest(http.MethodPut, "/api/v1/patients/p-1/soep-notes/"+note.ID, bytes.NewBufferString(soepPayload))
	reqPutOK.Header.Set("X-Actor-ID", "dent-1")
	reqPutOK.Header.Set("X-Actor-Role", "dentist")
	reqPutOK.Header.Set("X-MFA-Verified", "true")
	recPutOK := httptest.NewRecorder()
	mux.ServeHTTP(recPutOK, reqPutOK)
	if recPutOK.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on unlocked SOEP note update, got %d", recPutOK.Code)
	}

	// 3. Advance clock past 24h -> PUT returns 409 Conflict SOEP_NOTE_LOCKED (S2)
	clock.now = clock.now.Add(25 * time.Hour)
	reqPutLocked := httptest.NewRequest(http.MethodPut, "/api/v1/patients/p-1/soep-notes/"+note.ID, bytes.NewBufferString(soepPayload))
	reqPutLocked.Header.Set("X-Actor-ID", "dent-1")
	reqPutLocked.Header.Set("X-Actor-Role", "dentist")
	reqPutLocked.Header.Set("X-MFA-Verified", "true")
	recPutLocked := httptest.NewRecorder()
	mux.ServeHTTP(recPutLocked, reqPutLocked)
	if recPutLocked.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on locked SOEP note, got %d", recPutLocked.Code)
	}

	// 4. DELETE SOEP note -> 409 Conflict CLINICAL_RECORD_IMMUTABLE (S2)
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/patients/p-1/soep-notes/"+note.ID, nil)
	reqDel.Header.Set("X-Actor-ID", "dent-1")
	reqDel.Header.Set("X-Actor-Role", "dentist")
	reqDel.Header.Set("X-MFA-Verified", "true")
	recDel := httptest.NewRecorder()
	mux.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on deleting SOEP note, got %d", recDel.Code)
	}

	// 5. POST /addenda on locked note -> 201 Created (S3)
	reqAdd := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/soep-notes/"+note.ID+"/addenda", bytes.NewBufferString(`{
		"reason": "Control evolutivo posterior",
		"correction_text": "Se documenta obturación definitiva con cono único y cemento biocerámico."
	}`))
	reqAdd.Header.Set("X-Actor-ID", "dent-1")
	reqAdd.Header.Set("X-Actor-Role", "dentist")
	reqAdd.Header.Set("X-MFA-Verified", "true")
	recAdd := httptest.NewRecorder()
	mux.ServeHTTP(recAdd, reqAdd)
	if recAdd.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on addendum, got %d: %s", recAdd.Code, recAdd.Body.String())
	}

	// 6. Create and sign informed consent -> 200 OK & re-sign -> 409 Conflict (S4)
	reqConsent := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/consents", bytes.NewBufferString(`{
		"procedure_code": "ENDO-16",
		"title": "Consentimiento de Endodoncia",
		"rendered_text": "Autorizo procedimiento endodóntico en pieza 16."
	}`))
	reqConsent.Header.Set("X-Actor-ID", "dent-1")
	reqConsent.Header.Set("X-Actor-Role", "dentist")
	reqConsent.Header.Set("X-MFA-Verified", "true")
	recConsent := httptest.NewRecorder()
	mux.ServeHTTP(recConsent, reqConsent)
	if recConsent.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create consent, got %d", recConsent.Code)
	}
	var consent domain.PatientConsent
	_ = json.Unmarshal(recConsent.Body.Bytes(), &consent)

	signBody := `{
		"signature_method": "BIOMETRIC",
		"biometric_svg_strokes": "<svg><path d='M10 10 L90 90'/></svg>",
		"signed_by_name": "Martín Echeverría",
		"signed_by_document": "40123987"
	}`
	reqSign := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/consents/"+consent.ID+"/sign", bytes.NewBufferString(signBody))
	reqSign.Header.Set("X-Actor-ID", "dent-1")
	reqSign.Header.Set("X-Actor-Role", "dentist")
	reqSign.Header.Set("X-MFA-Verified", "true")
	recSign := httptest.NewRecorder()
	mux.ServeHTTP(recSign, reqSign)
	if recSign.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on sign consent, got %d: %s", recSign.Code, recSign.Body.String())
	}

	reqResign := httptest.NewRequest(http.MethodPost, "/api/v1/patients/p-1/consents/"+consent.ID+"/sign", bytes.NewBufferString(signBody))
	reqResign.Header.Set("X-Actor-ID", "dent-1")
	reqResign.Header.Set("X-Actor-Role", "dentist")
	reqResign.Header.Set("X-MFA-Verified", "true")
	recResign := httptest.NewRecorder()
	mux.ServeHTTP(recResign, reqResign)
	if recResign.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on re-signing consent, got %d", recResign.Code)
	}
}
