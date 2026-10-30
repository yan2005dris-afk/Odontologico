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
	identityhttp "github.com/sigo-dental/clinical-agenda/internal/identity/adapters/http"
	"github.com/sigo-dental/clinical-agenda/internal/identity/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/identity/application"
	"github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

type stepClock struct {
	now time.Time
}

func (s *stepClock) Now() time.Time { return s.now }

func TestAuthHandler_EndToEndFlow(t *testing.T) {
	repo := postgres.NewUserRepository()
	clock := &stepClock{now: time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)}
	svc := application.NewAuthService(repo, nil, clock, "test-jwt-secret", nil)

	secret := "JBSWY3DPEHPK3PXP"
	user, _ := domain.NewUser("u-1", "dra.gomez@sigo-dental.com", "Dra. Valeria Gómez", domain.RoleDentist, "ClinicalPass#2026", secret, clock.Now())
	_ = repo.CreateUser(context.Background(), user)

	handler := identityhttp.NewAuthHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 1. Bad JSON & empty fields -> 400
	for _, body := range []string{"{bad", `{"email":""}`} {
		reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
		recBad := httptest.NewRecorder()
		mux.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", recBad.Code)
		}
	}

	// 2. Wrong password -> 401
	reqWrong := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"dra.gomez@sigo-dental.com","password":"WrongPassword123"}`))
	reqWrong.Header.Set("X-Forwarded-For", "192.168.10.24, 10.0.0.1")
	recWrong := httptest.NewRecorder()
	mux.ServeHTTP(recWrong, reqWrong)
	if recWrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recWrong.Code)
	}

	// 3. Valid Login -> 200 OK
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"dra.gomez@sigo-dental.com","password":"ClinicalPass#2026"}`))
	reqLogin.Header.Set("X-Source-IP", "192.168.10.24")
	recLogin := httptest.NewRecorder()
	mux.ServeHTTP(recLogin, reqLogin)
	if recLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", recLogin.Code, recLogin.Body.String())
	}

	var loginResp application.LoginOutput
	_ = json.Unmarshal(recLogin.Body.Bytes(), &loginResp)

	// 4. Verify MFA error cases (400 & 401)
	for _, body := range []string{"{bad", `{"challenge_token":""}`} {
		reqBadMFA := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewBufferString(body))
		recBadMFA := httptest.NewRecorder()
		mux.ServeHTTP(recBadMFA, reqBadMFA)
		if recBadMFA.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on bad MFA payload, got %d", recBadMFA.Code)
		}
	}
	reqInvalidMFA := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewBufferString(`{"challenge_token":"invalid","totp_code":"123456"}`))
	recInvalidMFA := httptest.NewRecorder()
	mux.ServeHTTP(recInvalidMFA, reqInvalidMFA)
	if recInvalidMFA.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid MFA challenge, got %d", recInvalidMFA.Code)
	}

	// 5. Verify MFA -> 200 OK
	code, _ := domain.GenerateTOTP(secret, clock.Now())
	mfaBody, _ := json.Marshal(map[string]string{
		"challenge_token": loginResp.ChallengeToken,
		"totp_code":       code,
	})
	reqMFA := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(mfaBody))
	reqMFA.Header.Set("X-Source-IP", "192.168.10.24")
	recMFA := httptest.NewRecorder()
	mux.ServeHTTP(recMFA, reqMFA)
	if recMFA.Code != http.StatusOK {
		t.Fatalf("expected 200 on MFA verify, got %d: %s", recMFA.Code, recMFA.Body.String())
	}

	var mfaResp application.VerifyMFAOutput
	_ = json.Unmarshal(recMFA.Body.Bytes(), &mfaResp)

	// 6. Heartbeat & Logout missing header -> 400
	reqNoSid := httptest.NewRequest(http.MethodPost, "/api/v1/auth/heartbeat", nil)
	recNoSid := httptest.NewRecorder()
	mux.ServeHTTP(recNoSid, reqNoSid)
	if recNoSid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing X-Session-ID, got %d", recNoSid.Code)
	}
	reqLogoutNoSid := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	recLogoutNoSid := httptest.NewRecorder()
	mux.ServeHTTP(recLogoutNoSid, reqLogoutNoSid)
	if recLogoutNoSid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing X-Session-ID, got %d", recLogoutNoSid.Code)
	}

	// 7. Heartbeat within 15m -> 204 No Content
	reqHB := httptest.NewRequest(http.MethodPost, "/api/v1/auth/heartbeat", nil)
	reqHB.Header.Set("X-Session-ID", mfaResp.SessionID)
	recHB := httptest.NewRecorder()
	mux.ServeHTTP(recHB, reqHB)
	if recHB.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on heartbeat, got %d", recHB.Code)
	}

	// 8. Heartbeat after 16m idle -> 401 SESSION_IDLE_TIMEOUT
	clock.now = clock.now.Add(16 * time.Minute)
	reqHBExpired := httptest.NewRequest(http.MethodPost, "/api/v1/auth/heartbeat", nil)
	reqHBExpired.Header.Set("X-Session-ID", mfaResp.SessionID)
	recHBExpired := httptest.NewRecorder()
	mux.ServeHTTP(recHBExpired, reqHBExpired)
	if recHBExpired.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on idle timeout, got %d", recHBExpired.Code)
	}

	// 9. Subsequent heartbeat on revoked session -> 401 SESSION_REVOKED
	reqHBRevoked := httptest.NewRequest(http.MethodPost, "/api/v1/auth/heartbeat", nil)
	reqHBRevoked.Header.Set("X-Session-ID", mfaResp.SessionID)
	recHBRevoked := httptest.NewRecorder()
	mux.ServeHTTP(recHBRevoked, reqHBRevoked)
	if recHBRevoked.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on revoked session, got %d", recHBRevoked.Code)
	}

	// 10. Logout -> 204 No Content
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	reqLogout.Header.Set("X-Session-ID", mfaResp.SessionID)
	recLogout := httptest.NewRecorder()
	mux.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on logout, got %d", recLogout.Code)
	}
}

type listUsersResponse struct {
	Items []application.UserSummaryDTO `json:"items"`
	Total int                          `json:"total"`
}

// TestAuthHandler_ListUsers covers S1/T3: the staff listing is restricted to
// the `admin` role, answers {items,total}, applies the q/role filters and the
// pagination contract, and leaves an EXPORT audit trail (RNF-06).
func TestAuthHandler_ListUsers(t *testing.T) {
	repo := postgres.NewUserRepository()
	auditRepo := auditpg.NewAuditRepository()
	clock := &stepClock{now: time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)}
	auditSvc := auditapp.NewAuditService(auditRepo, clock, nil)
	svc := application.NewAuthService(repo, auditSvc, clock, "test-jwt-secret", nil)

	seeded := []*domain.User{
		{ID: "u-admin", Email: "admin@sigo-dental.com", FullName: "Andrés Vega Salinas", Role: domain.RoleAdmin, Active: true, MFASecretEncrypted: "GEZDGNBVGY3TQOJQ", CreatedAt: clock.now.Add(4 * time.Hour)},
		{ID: "u-reception", Email: "recepcion@sigo-dental.com", FullName: "Lucía Fernández Ruiz", Role: domain.RoleReceptionist, Active: true, MFASecretEncrypted: "MFRGGZDFMZTWQ2LK", CreatedAt: clock.now.Add(3 * time.Hour)},
		{ID: "u-dentist", Email: "dra.gomez@sigo-dental.com", FullName: "Dra. Valeria Gómez", Role: domain.RoleDentist, Active: true, MFASecretEncrypted: "JBSWY3DPEHPK3PXP", CreatedAt: clock.now.Add(2 * time.Hour)},
		{ID: "u-assistant", Email: "asistente@sigo-dental.com", FullName: "Marcelo Ríos Cabrera", Role: domain.RoleAssistant, Active: true, MFASecretEncrypted: "ONSWG4TFOQQGSZLD", CreatedAt: clock.now.Add(time.Hour)},
		{ID: "u-auditor", Email: "auditor@sigo-dental.com", FullName: "Paula Herrera Ortiz", Role: domain.RoleAuditor, Active: false, MFASecretEncrypted: "P3K6H5RTX2W7YZAB", CreatedAt: clock.now},
	}
	for _, u := range seeded {
		if err := repo.CreateUser(context.Background(), u); err != nil {
			t.Fatalf("failed to seed user %s: %v", u.Email, err)
		}
	}

	handler := identityhttp.NewAuthHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	doGet := func(path, role string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Actor-ID", "actor-1")
		req.Header.Set("X-Actor-Role", role)
		req.Header.Set("X-Source-IP", "203.0.113.10")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	// 1. A non admin role is rejected with 403 FORBIDDEN_ROLE.
	recForbidden := doGet("/api/v1/users", "dentist")
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non admin role, got %d: %s", recForbidden.Code, recForbidden.Body.String())
	}
	var forbiddenBody map[string]string
	_ = json.Unmarshal(recForbidden.Body.Bytes(), &forbiddenBody)
	if forbiddenBody["code"] != "FORBIDDEN_ROLE" {
		t.Fatalf("expected FORBIDDEN_ROLE, got %q", forbiddenBody["code"])
	}

	// 2. An admin gets 200 with the {items,total} contract.
	rec := doGet("/api/v1/users", "admin")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list users, got %d: %s", rec.Code, rec.Body.String())
	}
	var list listUsersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode the listing: %v", err)
	}
	if list.Total != 5 || len(list.Items) != 5 {
		t.Fatalf("expected 5 users, got total=%d len=%d", list.Total, len(list.Items))
	}
	// Deterministic order: newest first.
	if list.Items[0].Email != "admin@sigo-dental.com" {
		t.Fatalf("expected the newest account first, got %q", list.Items[0].Email)
	}
	if list.Items[0].Role != domain.RoleAdmin || !list.Items[0].MFAEnabled || !list.Items[0].IsActive {
		t.Fatalf("unexpected admin projection: %+v", list.Items[0])
	}

	// 3. The q filter searches by email or full name.
	recQ := doGet("/api/v1/users?q=Vega", "admin")
	if recQ.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on q filter, got %d", recQ.Code)
	}
	var byQ listUsersResponse
	_ = json.Unmarshal(recQ.Body.Bytes(), &byQ)
	if byQ.Total != 1 || byQ.Items[0].Email != "admin@sigo-dental.com" {
		t.Fatalf("expected only the admin account, got %+v", byQ)
	}

	// 4. The role filter narrows the listing.
	recRole := doGet("/api/v1/users?role=dentist", "admin")
	if recRole.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on role filter, got %d", recRole.Code)
	}
	var byRole listUsersResponse
	_ = json.Unmarshal(recRole.Body.Bytes(), &byRole)
	if byRole.Total != 1 || byRole.Items[0].Role != domain.RoleDentist {
		t.Fatalf("expected only the dentist account, got %+v", byRole)
	}

	// 5. Pagination: total stays the total before limit/offset.
	recPage := doGet("/api/v1/users?limit=2&offset=1", "admin")
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on pagination, got %d", recPage.Code)
	}
	var page listUsersResponse
	_ = json.Unmarshal(recPage.Body.Bytes(), &page)
	if page.Total != 5 {
		t.Fatalf("expected the total before pagination to be 5, got %d", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 users on the page, got %d", len(page.Items))
	}
	if page.Items[0].Email != "recepcion@sigo-dental.com" {
		t.Fatalf("expected the second newest account, got %q", page.Items[0].Email)
	}

	// 6. The hard ceiling of 200 items per page is enforced.
	for i := 0; i < 220; i++ {
		bulk := &domain.User{
			ID:        fmt.Sprintf("u-bulk-%03d", i),
			Email:     fmt.Sprintf("bulk%03d@sigo-dental.com", i),
			FullName:  fmt.Sprintf("Personal %03d", i),
			Role:      domain.RoleAssistant,
			Active:    true,
			CreatedAt: clock.now.Add(-time.Duration(i+1) * time.Minute),
		}
		if err := repo.CreateUser(context.Background(), bulk); err != nil {
			t.Fatalf("failed to seed bulk user %d: %v", i, err)
		}
	}
	recCeiling := doGet("/api/v1/users?limit=1000", "admin")
	if recCeiling.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on the limit ceiling, got %d", recCeiling.Code)
	}
	var ceiling listUsersResponse
	_ = json.Unmarshal(recCeiling.Body.Bytes(), &ceiling)
	if ceiling.Total != 225 {
		t.Fatalf("expected 225 users in total, got %d", ceiling.Total)
	}
	if len(ceiling.Items) != 200 {
		t.Fatalf("expected the limit ceiling of 200 items, got %d", len(ceiling.Items))
	}

	// 7. Every listing leaves an EXPORT audit trail (RNF-06).
	entries, totalEntries, err := auditRepo.List(context.Background(), auditapp.AuditFilter{Action: "EXPORT", ResourceType: "USER"})
	if err != nil {
		t.Fatalf("expected to read the audit trail, got %v", err)
	}
	if totalEntries == 0 {
		t.Fatal("expected the listing to leave an EXPORT audit trail")
	}
	entry := entries[0]
	if entry.ResourceID != "users" {
		t.Fatalf("expected the collection resource id, got %q", entry.ResourceID)
	}
	if entry.ActorID != "actor-1" || entry.ActorRole != "admin" {
		t.Fatalf("expected the actor to be recorded, got %+v", entry)
	}
	if entry.SourceIP != "203.0.113.10" {
		t.Fatalf("expected the source ip to be recorded, got %q", entry.SourceIP)
	}
	if entry.TimestampISO8601 == "" {
		t.Fatal("expected an ISO-8601 timestamp in the audit trail")
	}
}

// failingRecorder simulates an unavailable audit storage.
type failingRecorder struct{}

func (failingRecorder) Record(context.Context, auditapp.RecordInput) (*auditdom.AuditEntry, error) {
	return nil, errors.New("audit storage unavailable")
}

// TestAuthHandler_ListUsersSurvivesAuditFailure covers RNF-06: a failing audit
// recorder must not invalidate the 200 response of a listing, but the failure
// has to be logged instead of being silently discarded.
func TestAuthHandler_ListUsersSurvivesAuditFailure(t *testing.T) {
	repo := postgres.NewUserRepository()
	clock := &stepClock{now: time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)}
	svc := application.NewAuthService(repo, failingRecorder{}, clock, "test-jwt-secret", nil)

	handler := identityhttp.NewAuthHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("X-Actor-ID", "actor-1")
	req.Header.Set("X-Actor-Role", "admin")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK even when the audit recorder fails, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "audit trail could not be recorded") {
		t.Fatalf("expected the audit failure to be logged, got %q", logs.String())
	}
}
