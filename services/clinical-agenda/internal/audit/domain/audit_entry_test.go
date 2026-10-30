package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewAuditEntry_ValidAndImmutable(t *testing.T) {
	ts := time.Date(2026, 10, 30, 14, 22, 10, 0, time.UTC)
	entry, err := NewAuditEntry("audit-1", "actor-1", "dentist", ActionUpdate, "ODONTOGRAM", "res-1", "192.168.10.24", ts, map[string]string{"tooth": "16"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if entry.TimestampISO8601 != "2026-10-30T14:22:10Z" {
		t.Fatalf("unexpected timestamp %s", entry.TimestampISO8601)
	}
	if !errors.Is(entry.Mutate(), ErrAuditLogImmutable) {
		t.Fatalf("expected ErrAuditLogImmutable on Mutate")
	}
	if !errors.Is(entry.Delete(), ErrAuditLogImmutable) {
		t.Fatalf("expected ErrAuditLogImmutable on Delete")
	}
}

func TestNewAuditEntry_ValidationErrors(t *testing.T) {
	ts := time.Now()
	if _, err := NewAuditEntry("1", "", "dentist", ActionCreate, "PATIENT", "1", "127.0.0.1", ts, nil); !errors.Is(err, ErrInvalidAuditActor) {
		t.Fatalf("expected ErrInvalidAuditActor, got %v", err)
	}
	if _, err := NewAuditEntry("1", "a", "", ActionCreate, "PATIENT", "1", "127.0.0.1", ts, nil); !errors.Is(err, ErrInvalidAuditRole) {
		t.Fatalf("expected ErrInvalidAuditRole, got %v", err)
	}
	if _, err := NewAuditEntry("1", "a", "dentist", "DELETE", "PATIENT", "1", "127.0.0.1", ts, nil); !errors.Is(err, ErrInvalidAuditAction) {
		t.Fatalf("expected ErrInvalidAuditAction, got %v", err)
	}
	if _, err := NewAuditEntry("1", "a", "dentist", ActionRead, "", "1", "", time.Time{}, nil); !errors.Is(err, ErrInvalidAuditResource) {
		t.Fatalf("expected ErrInvalidAuditResource, got %v", err)
	}
	entry, err := NewAuditEntry("1", "a", "dentist", ActionRead, "PATIENT", "1", "", time.Time{}, nil)
	if err != nil || entry.SourceIP != "0.0.0.0" {
		t.Fatalf("expected default source IP, got %v, err=%v", entry, err)
	}
}
