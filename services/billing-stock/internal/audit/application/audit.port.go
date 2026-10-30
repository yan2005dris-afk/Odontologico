package application

import (
	"context"
	"time"

	"github.com/sigo-dental/billing-stock/internal/audit/domain"
)

type AuditFilter struct {
	ResourceType string
	ResourceID   string
	ActorID      string
	Action       string
	From         *time.Time
	To           *time.Time
	Limit        int
	Offset       int
}

// AuditRepository is the append-only sink of the RNF-06 trail.
type AuditRepository interface {
	Append(ctx context.Context, entry *domain.AuditEntry) error
	List(ctx context.Context, filter AuditFilter) ([]*domain.AuditEntry, int, error)
}

// RecordInput is the payload of a trail write.
type RecordInput struct {
	ActorID      string
	ActorRole    string
	Action       domain.AuditAction
	ResourceType string
	ResourceID   string
	SourceIP     string
	Metadata     map[string]string
}

// Recorder writes the immutable trail. Callers must not fail the business
// operation when the recorder returns an error (RNF-06 is best-effort evidence).
type Recorder interface {
	Record(ctx context.Context, input RecordInput) (*domain.AuditEntry, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}
