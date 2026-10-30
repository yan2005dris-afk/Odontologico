package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	auditpg "github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	auditapp "github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/identity/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/identity/application"
	"github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

type manualClock struct {
	now time.Time
}

func (m *manualClock) Now() time.Time { return m.now }

func TestAuthService_LoginMFAAndIdleTimeout(t *testing.T) {
	userRepo := postgres.NewUserRepository()
	auditRepo := auditpg.NewAuditRepository()
	clock := &manualClock{now: time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)}
	auditSvc := auditapp.NewAuditService(auditRepo, clock, nil)
	authSvc := application.NewAuthService(userRepo, auditSvc, clock, "test-secret-2026")
	_ = application.NewAuthService(userRepo, nil, nil, "")

	secret := "JBSWY3DPEHPK3PXP"
	user, err := domain.NewUser("c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d", "dra.gomez@sigo-dental.com", "Dra. Valeria Gómez", domain.RoleDentist, "ClinicalPass#2026", secret, clock.Now())
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	_ = userRepo.CreateUser(context.Background(), user)

	// Invalid login inputs
	if _, err := authSvc.Login(context.Background(), application.LoginInput{Email: "bad", Password: "short"}); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials")
	}
	if _, err := authSvc.Login(context.Background(), application.LoginInput{Email: "missing@sigo-dental.com", Password: "ClinicalPass#2026"}); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials")
	}

	// Inactive user check
	inactiveUser, _ := domain.NewUser("u-inactive", "inactive@sigo-dental.com", "Inactive", domain.RoleAssistant, "ClinicalPass#2026", secret, clock.Now())
	inactiveUser.Active = false
	_ = userRepo.CreateUser(context.Background(), inactiveUser)
	if _, err := authSvc.Login(context.Background(), application.LoginInput{Email: "inactive@sigo-dental.com", Password: "ClinicalPass#2026"}); !errors.Is(err, domain.ErrAccountInactive) {
		t.Fatalf("expected ErrAccountInactive")
	}

	// Lockout after 5 failed password attempts
	lockUser, _ := domain.NewUser("u-lock", "lock@sigo-dental.com", "Lock", domain.RoleReceptionist, "ClinicalPass#2026", secret, clock.Now())
	_ = userRepo.CreateUser(context.Background(), lockUser)
	for i := 0; i < 4; i++ {
		if _, err := authSvc.Login(context.Background(), application.LoginInput{Email: "lock@sigo-dental.com", Password: "WrongPassword#1"}); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials on attempt %d, got %v", i+1, err)
		}
	}
	if _, err := authSvc.Login(context.Background(), application.LoginInput{Email: "lock@sigo-dental.com", Password: "WrongPassword#1"}); !errors.Is(err, domain.ErrAccountLocked) {
		t.Fatalf("expected ErrAccountLocked on 5th failed attempt, got %v", err)
	}
	if _, err := authSvc.Login(context.Background(), application.LoginInput{Email: "lock@sigo-dental.com", Password: "ClinicalPass#2026"}); !errors.Is(err, domain.ErrAccountLocked) {
		t.Fatalf("expected ErrAccountLocked while locked, got %v", err)
	}

	// S1: Login returns mfa_required=true and challenge_token
	loginOut, err := authSvc.Login(context.Background(), application.LoginInput{
		Email:    "dra.gomez@sigo-dental.com",
		Password: "ClinicalPass#2026",
		SourceIP: "192.168.10.24",
	})
	if err != nil {
		t.Fatalf("expected login success, got %v", err)
	}
	if !loginOut.MFARequired || loginOut.ChallengeToken == "" {
		t.Fatalf("expected mfa_required=true and non-empty challenge_token")
	}

	// Malformed challenge tokens
	for _, badTok := range []string{"bad", "a.b.c", loginOut.ChallengeToken + "tampered"} {
		if _, err := authSvc.VerifyMFA(context.Background(), application.VerifyMFAInput{
			ChallengeToken: badTok,
			TOTPCode:       "123456",
		}); !errors.Is(err, application.ErrInvalidChallengeToken) {
			t.Fatalf("expected ErrInvalidChallengeToken for %s, got %v", badTok, err)
		}
	}

	// S2: Invalid TOTP fails with ErrInvalidMFACode
	if _, err := authSvc.VerifyMFA(context.Background(), application.VerifyMFAInput{
		ChallengeToken: loginOut.ChallengeToken,
		TOTPCode:       "999999",
		SourceIP:       "192.168.10.24",
	}); !errors.Is(err, domain.ErrInvalidMFACode) {
		t.Fatalf("expected ErrInvalidMFACode, got %v", err)
	}

	// S2: Valid TOTP issues access_token with mfa_verified=true and idle_timeout_seconds=900
	validCode, _ := domain.GenerateTOTP(secret, clock.Now())
	mfaOut, err := authSvc.VerifyMFA(context.Background(), application.VerifyMFAInput{
		ChallengeToken: loginOut.ChallengeToken,
		TOTPCode:       validCode,
		SourceIP:       "192.168.10.24",
	})
	if err != nil {
		t.Fatalf("expected MFA verification success, got %v", err)
	}
	if !mfaOut.User.MFAVerified || mfaOut.IdleTimeoutSeconds != 900 || mfaOut.SessionID == "" {
		t.Fatalf("unexpected MFA output: %+v", mfaOut)
	}

	// S2 adversarial: Replay of the exact same TOTP code must fail
	if _, err := authSvc.VerifyMFA(context.Background(), application.VerifyMFAInput{
		ChallengeToken: loginOut.ChallengeToken,
		TOTPCode:       validCode,
		SourceIP:       "192.168.10.24",
	}); !errors.Is(err, domain.ErrInvalidMFACode) {
		t.Fatalf("expected replayed TOTP code to be rejected, got %v", err)
	}

	// S3: Session touch within 15 minutes succeeds
	clock.now = clock.now.Add(10 * time.Minute)
	if _, err := authSvc.ValidateAndTouchSession(context.Background(), mfaOut.SessionID); err != nil {
		t.Fatalf("expected active session at +10m, got %v", err)
	}
	if _, err := authSvc.ValidateAndTouchSession(context.Background(), "non-existent"); !errors.Is(err, domain.ErrSessionRevoked) {
		t.Fatalf("expected ErrSessionRevoked for unknown session")
	}

	// S3: Session touch after >15m of inactivity returns ErrSessionIdleTimeout
	clock.now = clock.now.Add(15*time.Minute + 2*time.Second)
	if _, err := authSvc.ValidateAndTouchSession(context.Background(), mfaOut.SessionID); !errors.Is(err, domain.ErrSessionIdleTimeout) {
		t.Fatalf("expected ErrSessionIdleTimeout, got %v", err)
	}

	// Logout
	_ = authSvc.Logout(context.Background(), mfaOut.SessionID)
	_ = authSvc.Logout(context.Background(), "non-existent")
}
