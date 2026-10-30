package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
