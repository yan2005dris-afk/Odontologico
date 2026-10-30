package main

import (
	"context"
	"testing"
	"time"

	identitypg "github.com/sigo-dental/clinical-agenda/internal/identity/adapters/postgres"
	identityapp "github.com/sigo-dental/clinical-agenda/internal/identity/application"
	identitydom "github.com/sigo-dental/clinical-agenda/internal/identity/domain"
)

// TestSeedCanonicalUsers_CreatesFiveRolesAndIsIdempotent covers S0: the seed
// creates exactly the 5 canonical RBAC accounts of SRS §3, preserves the
// credentials the portal depends on and can be replayed without duplicating
// accounts.
func TestSeedCanonicalUsers_CreatesFiveRolesAndIsIdempotent(t *testing.T) {
	// The TOTP secrets are read from the environment: pin them to the
	// documented defaults so the assertions do not depend on the host env.
	for _, env := range []string{
		"SIGO_SEED_TOTP_ADMIN",
		"SIGO_SEED_TOTP_RECEPTIONIST",
		"SIGO_SEED_TOTP_DENTIST",
		"SIGO_SEED_TOTP_ASSISTANT",
		"SIGO_SEED_TOTP_AUDITOR",
	} {
		t.Setenv(env, "")
	}

	ctx := context.Background()
	repo := identitypg.NewUserRepository()
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

	if err := seedCanonicalUsers(ctx, repo, now); err != nil {
		t.Fatalf("expected the seed to succeed, got %v", err)
	}

	users, total, err := repo.ListUsers(ctx, identityapp.UserFilter{Limit: 50})
	if err != nil {
		t.Fatalf("expected ListUsers to succeed, got %v", err)
	}
	if total != 5 || len(users) != 5 {
		t.Fatalf("expected exactly 5 seeded users, got total=%d len=%d", total, len(users))
	}

	roles := make(map[string]bool, len(users))
	for _, u := range users {
		roles[string(u.Role)] = true
		if !u.Active {
			t.Fatalf("expected seeded user %q to be active", u.Email)
		}
		if u.MFASecretEncrypted == "" {
			t.Fatalf("expected seeded user %q to have MFA enabled", u.Email)
		}
	}
	for _, want := range []string{"admin", "receptionist", "dentist", "assistant", "auditor"} {
		if !roles[want] {
			t.Fatalf("expected role %q among the seeded users, got %v", want, roles)
		}
	}

	// The canonical dentist keeps the credentials the portal documents.
	dentist, err := repo.GetByEmail(ctx, "dra.gomez@sigo-dental.com")
	if err != nil {
		t.Fatalf("expected the canonical dentist to exist, got %v", err)
	}
	if dentist.Role != identitydom.RoleDentist {
		t.Fatalf("expected the canonical dentist to keep the dentist role, got %q", dentist.Role)
	}
	if dentist.MFASecretEncrypted != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("expected the canonical dentist TOTP secret to be preserved, got %q", dentist.MFASecretEncrypted)
	}
	if !identitydom.VerifyPasswordArgon2id("ClinicalPass#2026", dentist.PasswordHash) {
		t.Fatal("expected the canonical dentist password to be preserved")
	}

	// Idempotency: a second run neither fails nor duplicates accounts.
	if err := seedCanonicalUsers(ctx, repo, now.Add(time.Hour)); err != nil {
		t.Fatalf("expected the second seed run to be idempotent, got %v", err)
	}
	_, total, err = repo.ListUsers(ctx, identityapp.UserFilter{Limit: 50})
	if err != nil {
		t.Fatalf("expected ListUsers to succeed, got %v", err)
	}
	if total != 5 {
		t.Fatalf("expected still 5 users after re-seeding, got %d", total)
	}
}
