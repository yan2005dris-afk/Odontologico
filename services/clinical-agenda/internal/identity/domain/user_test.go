package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

func TestNewUserAndPasswordArgon2id(t *testing.T) {
	now := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)
	for _, role := range []domain.Role{
		domain.RoleAdmin,
		domain.RoleReceptionist,
		domain.RoleDentist,
		domain.RoleAssistant,
		domain.RoleAuditor,
	} {
		u, err := domain.NewUser("u-1", "dra.gomez@sigo-dental.com", "Dra. Valeria Gómez", role, "StrongPass#2026", "JBSWY3DPEHPK3PXP", now)
		if err != nil {
			t.Fatalf("expected valid user for role %s, got %v", role, err)
		}
		if !strings.HasPrefix(u.PasswordHash, "$argon2id$v=19$m=65536,t=3,p=4$") {
			t.Fatalf("unexpected password hash format: %s", u.PasswordHash)
		}
		if !domain.VerifyPasswordArgon2id("StrongPass#2026", u.PasswordHash) {
			t.Fatalf("expected password verification to succeed")
		}
		if domain.VerifyPasswordArgon2id("WrongPass#2026", u.PasswordHash) {
			t.Fatalf("expected wrong password to fail")
		}
	}

	if _, err := domain.NewUser("u-2", "bad-email", "Name", domain.RoleAdmin, "StrongPass#2026", "", now); !errors.Is(err, domain.ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
	if _, err := domain.NewUser("u-2", "a@b.com", "Name", domain.RoleAdmin, "short", "", now); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
	if _, err := domain.NewUser("u-2", "a@b.com", "Name", "hacker", "StrongPass#2026", "", now); !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestUserLockoutAndTOTP(t *testing.T) {
	now := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)
	u, _ := domain.NewUser("u-1", "dra.gomez@sigo-dental.com", "Dra. Valeria Gómez", domain.RoleDentist, "StrongPass#2026", "JBSWY3DPEHPK3PXP", now)

	for i := 0; i < 5; i++ {
		if u.IsLocked(now) {
			t.Fatalf("should not be locked before 5 attempts")
		}
		u.RecordFailedAttempt(now)
	}
	if !u.IsLocked(now.Add(5 * time.Minute)) {
		t.Fatalf("expected account to be locked after 5 failed attempts")
	}
	u.ResetFailedAttempts(now)
	if u.IsLocked(now) {
		t.Fatalf("expected account to be unlocked after reset")
	}

	code, err := domain.GenerateTOTP(u.MFASecretEncrypted, now)
	if err != nil || len(code) != 6 {
		t.Fatalf("expected 6-digit TOTP code, got %q err=%v", code, err)
	}
	if !domain.VerifyTOTP(u.MFASecretEncrypted, code, now) {
		t.Fatalf("expected TOTP verification to succeed")
	}
	if domain.VerifyTOTP(u.MFASecretEncrypted, "000000", now) && code != "000000" {
		t.Fatalf("expected invalid TOTP to fail")
	}
	if domain.VerifyTOTP(u.MFASecretEncrypted, "12ab56", now) {
		t.Fatalf("expected non-numeric TOTP to fail")
	}
}

func TestSession_IdleTimeout15Minutes(t *testing.T) {
	now := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)
	sess := domain.NewSession("s-1", "u-1", domain.RoleDentist, true, "192.168.10.24", now)

	// Activity at +10m succeeds and slides window
	if err := sess.ValidateAndTouch(now.Add(10 * time.Minute)); err != nil {
		t.Fatalf("expected session active at +10m, got %v", err)
	}

	// Activity at +25m01s (>15m after +10m) fails with ErrSessionIdleTimeout (S3)
	err := sess.ValidateAndTouch(now.Add(25*time.Minute + time.Second))
	if !errors.Is(err, domain.ErrSessionIdleTimeout) {
		t.Fatalf("expected ErrSessionIdleTimeout, got %v", err)
	}
	if !sess.Revoked {
		t.Fatalf("expected session to be marked revoked on idle timeout")
	}

	// Subsequent touch returns ErrSessionRevoked
	if !errors.Is(sess.ValidateAndTouch(now.Add(26*time.Minute)), domain.ErrSessionRevoked) {
		t.Fatalf("expected ErrSessionRevoked on revoked session")
	}
}
