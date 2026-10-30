package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/audit/adapters/postgres"
	"github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/audit/domain"
)

type fixedClock struct {
	t time.Time
}

func (f fixedClock) Now() time.Time { return f.t }

func TestAuditService_RecordAndList(t *testing.T) {
	repo := postgres.NewAuditRepository()
	now := time.Date(2026, 10, 30, 14, 22, 10, 0, time.UTC)
	svc := application.NewAuditService(repo, fixedClock{t: now}, nil)

	entry, err := svc.Record(context.Background(), application.RecordInput{
		ActorID:      "c9b1d7a0-1234-4a5b-8c9d-0e1f2a3b4c5d",
		ActorRole:    "dentist",
		Action:       domain.ActionUpdate,
		ResourceType: "ODONTOGRAM",
		ResourceID:   "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		SourceIP:     "192.168.10.24",
		Metadata:     map[string]string{"tooth": "16"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if entry.TimestampISO8601 != "2026-10-30T14:22:10Z" {
		t.Fatalf("expected 2026-10-30T14:22:10Z, got %s", entry.TimestampISO8601)
	}

	// Non-admin/auditor must be forbidden (S5)
	for _, role := range []string{"dentist", "receptionist", "assistant", ""} {
		if _, _, err := svc.ListLogs(context.Background(), role, application.AuditFilter{}); !errors.Is(err, application.ErrAuditForbidden) {
			t.Fatalf("expected ErrAuditForbidden for role %q, got %v", role, err)
		}
	}

	// Admin and auditor can list logs (S5)
	items, total, err := svc.ListLogs(context.Background(), "auditor", application.AuditFilter{
		ResourceType: "ODONTOGRAM",
		Limit:        300,
		Offset:       -5,
	})
	if err != nil {
		t.Fatalf("expected no error for auditor, got %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 item, got total=%d len=%d", total, len(items))
	}

	// Immutability check in repository (S4)
	if !errors.Is(repo.Update(context.Background(), entry.ID), domain.ErrAuditLogImmutable) {
		t.Fatalf("expected ErrAuditLogImmutable on repo Update")
	}
	if !errors.Is(repo.Delete(context.Background(), entry.ID), domain.ErrAuditLogImmutable) {
		t.Fatalf("expected ErrAuditLogImmutable on repo Delete")
	}
}
