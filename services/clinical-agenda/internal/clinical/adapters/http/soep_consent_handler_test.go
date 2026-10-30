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

type consentListResponse struct {
	Items []domain.PatientConsent `json:"items"`
	Total int                     `json:"total"`
}

// TestSOEPConsentHandler_ListConsents covers S1/T5: the global and per patient
// consent listings answer {items,total}, apply the filters and the pagination
// contract, never leak the consents of another patient and leave an EXPORT
// audit trail (RNF-06).
func TestSOEPConsentHandler_ListConsents(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 0, 0, 0, time.UTC)
	soepRepo := postgres.NewSOEPRepository()
	consentRepo := postgres.NewConsentRepository()
	auditRepo := auditpg.NewAuditRepository()
	auditSvc := auditapp.NewAuditService(auditRepo, nil, nil)

	soepSvc := application.NewSOEPService(soepRepo, nil, &stepClock{now: now}, 24*time.Hour)
	consentSvc := application.NewConsentService(consentRepo, nil, auditSvc, &stepClock{now: now})
	handler := clinicalhttp.NewSOEPConsentHandler(soepSvc, consentSvc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	signedAt := now.Add(-time.Hour)
	seeded := []*domain.PatientConsent{
		{
			ID: "consent-p1-pending", PatientID: "p-1", ProcedureCode: "ENDO-16",
			Title: "Consentimiento de Endodoncia", RenderedText: "Autorizo endodoncia.",
			Status: "PENDING_SIGNATURE", CreatedAt: now.Add(3 * time.Hour),
		},
		{
			ID: "consent-p1-bio", PatientID: "p-1", ProcedureCode: "COR-14",
			Title: "Consentimiento de Corona", RenderedText: "Autorizo corona.",
			Status: "SIGNED", SignatureMethod: "BIOMETRIC", SignedByName: "Martín Echeverría",
			SignedByDocument: "40123987", SignedAt: &signedAt, CreatedAt: now.Add(2 * time.Hour),
		},
		{
			ID: "consent-p1-otp", PatientID: "p-1", ProcedureCode: "IMPL-31",
			Title: "Consentimiento de Implante", RenderedText: "Autorizo implante.",
			Status: "SIGNED", SignatureMethod: "OTP", SignedByName: "Martín Echeverría",
			SignedByDocument: "40123987", SignedAt: &signedAt, CreatedAt: now.Add(time.Hour),
		},
		{
			ID: "consent-p2-pending", PatientID: "p-2", ProcedureCode: "ENDO-46",
			Title: "Consentimiento de Endodoncia", RenderedText: "Autorizo endodoncia.",
			Status: "PENDING_SIGNATURE", CreatedAt: now,
		},
	}
	for _, consent := range seeded {
		if err := consentRepo.SaveConsent(context.Background(), consent); err != nil {
			t.Fatalf("failed to seed consent %s: %v", consent.ID, err)
		}
	}

	doGet := func(path, role string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Actor-ID", "dent-1")
		req.Header.Set("X-Actor-Role", role)
		req.Header.Set("X-Source-IP", "203.0.113.22")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	// 1. The auditor role has no access to the informed consents.
	recForbidden := doGet("/api/v1/consents", "auditor")
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for the auditor role, got %d: %s", recForbidden.Code, recForbidden.Body.String())
	}
	var forbiddenBody map[string]string
	_ = json.Unmarshal(recForbidden.Body.Bytes(), &forbiddenBody)
	if forbiddenBody["code"] != "FORBIDDEN_ROLE" {
		t.Fatalf("expected FORBIDDEN_ROLE, got %q", forbiddenBody["code"])
	}

	// 2. The global listing answers {items,total}.
	rec := doGet("/api/v1/consents", "dentist")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list consents, got %d: %s", rec.Code, rec.Body.String())
	}
	var list consentListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode the listing: %v", err)
	}
	if list.Total != 4 || len(list.Items) != 4 {
		t.Fatalf("expected 4 consents, got total=%d len=%d", list.Total, len(list.Items))
	}
	if list.Items[0].ID != "consent-p1-pending" {
		t.Fatalf("expected the newest consent first, got %q", list.Items[0].ID)
	}

	// 3. The patient_id filter never leaks the consents of another patient.
	recPatient := doGet("/api/v1/consents?patient_id=p-2", "dentist")
	if recPatient.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the patient filter, got %d", recPatient.Code)
	}
	var byPatient consentListResponse
	_ = json.Unmarshal(recPatient.Body.Bytes(), &byPatient)
	if byPatient.Total != 1 || byPatient.Items[0].PatientID != "p-2" {
		t.Fatalf("expected only the consents of p-2, got %+v", byPatient)
	}

	// 4. The status filter accepts the public vocabulary.
	recPending := doGet("/api/v1/consents?status=PENDING", "dentist")
	if recPending.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the pending filter, got %d", recPending.Code)
	}
	var pending consentListResponse
	_ = json.Unmarshal(recPending.Body.Bytes(), &pending)
	if pending.Total != 2 {
		t.Fatalf("expected 2 pending consents, got %d", pending.Total)
	}
	for _, item := range pending.Items {
		if item.Status != "PENDING_SIGNATURE" {
			t.Fatalf("expected only pending consents, got %+v", item)
		}
	}

	recBio := doGet("/api/v1/consents?status=SIGNED_BIOMETRIC", "dentist")
	if recBio.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the biometric filter, got %d", recBio.Code)
	}
	var biometric consentListResponse
	_ = json.Unmarshal(recBio.Body.Bytes(), &biometric)
	if biometric.Total != 1 || biometric.Items[0].SignatureMethod != "BIOMETRIC" {
		t.Fatalf("expected only the biometric consent, got %+v", biometric)
	}

	recOTP := doGet("/api/v1/consents?status=SIGNED_OTP", "dentist")
	if recOTP.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the otp filter, got %d", recOTP.Code)
	}
	var otp consentListResponse
	_ = json.Unmarshal(recOTP.Body.Bytes(), &otp)
	if otp.Total != 1 || otp.Items[0].SignatureMethod != "OTP" {
		t.Fatalf("expected only the otp consent, got %+v", otp)
	}

	// 5. The procedure_code filter narrows the listing.
	recProcedure := doGet("/api/v1/consents?procedure_code=ENDO-16", "dentist")
	if recProcedure.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the procedure filter, got %d", recProcedure.Code)
	}
	var byProcedure consentListResponse
	_ = json.Unmarshal(recProcedure.Body.Bytes(), &byProcedure)
	if byProcedure.Total != 1 || byProcedure.Items[0].ProcedureCode != "ENDO-16" {
		t.Fatalf("expected only the matching consent, got %+v", byProcedure)
	}

	// 6. An unknown status is rejected.
	recBadStatus := doGet("/api/v1/consents?status=WHATEVER", "dentist")
	if recBadStatus.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on an unknown status, got %d: %s", recBadStatus.Code, recBadStatus.Body.String())
	}

	// 7. Pagination: total is the total before limit/offset.
	recPage := doGet("/api/v1/consents?limit=2&offset=1", "dentist")
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on pagination, got %d", recPage.Code)
	}
	var page consentListResponse
	_ = json.Unmarshal(recPage.Body.Bytes(), &page)
	if page.Total != 4 {
		t.Fatalf("expected the total before pagination to be 4, got %d", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 consents on the page, got %d", len(page.Items))
	}

	// 8. The hard ceiling of 200 items per page is enforced.
	for i := 0; i < 210; i++ {
		bulk := &domain.PatientConsent{
			ID: fmt.Sprintf("consent-bulk-%03d", i), PatientID: "p-3", ProcedureCode: "LIM-01",
			Title: "Consentimiento masivo", RenderedText: "Autorizo.",
			Status: "PENDING_SIGNATURE", CreatedAt: now.Add(-time.Duration(i+1) * time.Minute),
		}
		if err := consentRepo.SaveConsent(context.Background(), bulk); err != nil {
			t.Fatalf("failed to seed bulk consent %d: %v", i, err)
		}
	}
	recCeiling := doGet("/api/v1/consents?limit=1000", "dentist")
	if recCeiling.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the limit ceiling, got %d", recCeiling.Code)
	}
	var ceiling consentListResponse
	_ = json.Unmarshal(recCeiling.Body.Bytes(), &ceiling)
	if ceiling.Total != 214 {
		t.Fatalf("expected 214 consents in total, got %d", ceiling.Total)
	}
	if len(ceiling.Items) != 200 {
		t.Fatalf("expected the limit ceiling of 200 items, got %d", len(ceiling.Items))
	}

	// 9. The per patient listing returns only the consents of that patient.
	recScoped := doGet("/api/v1/patients/p-1/consents", "dentist")
	if recScoped.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the per patient listing, got %d: %s", recScoped.Code, recScoped.Body.String())
	}
	var scoped consentListResponse
	if err := json.Unmarshal(recScoped.Body.Bytes(), &scoped); err != nil {
		t.Fatalf("failed to decode the per patient listing: %v", err)
	}
	if scoped.Total != 3 || len(scoped.Items) != 3 {
		t.Fatalf("expected the 3 consents of p-1, got total=%d len=%d", scoped.Total, len(scoped.Items))
	}
	for _, item := range scoped.Items {
		if item.PatientID != "p-1" {
			t.Fatalf("expected only the consents of p-1, got %+v", item)
		}
	}

	// 10. Every listing leaves an EXPORT audit trail (RNF-06).
	entries, totalEntries, err := auditRepo.List(context.Background(), auditapp.AuditFilter{Action: "EXPORT", ResourceType: "INFORMED_CONSENT"})
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
		if entry.SourceIP != "203.0.113.22" {
			t.Fatalf("expected the source ip to be recorded, got %q", entry.SourceIP)
		}
		if entry.TimestampISO8601 == "" {
			t.Fatal("expected an ISO-8601 timestamp in the audit trail")
		}
	}
	if !resourceIDs["consents"] {
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

// TestSOEPConsentHandler_ListConsentsSurvivesAuditFailure covers RNF-06: a
// failing audit recorder must not invalidate the 200 response of a listing,
// but the failure has to be logged instead of being silently discarded.
func TestSOEPConsentHandler_ListConsentsSurvivesAuditFailure(t *testing.T) {
	now := time.Date(2026, 10, 30, 16, 0, 0, 0, time.UTC)
	consentRepo := postgres.NewConsentRepository()
	consentSvc := application.NewConsentService(consentRepo, nil, failingRecorder{}, &stepClock{now: now})
	handler := clinicalhttp.NewSOEPConsentHandler(nil, consentSvc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/consents", nil)
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
