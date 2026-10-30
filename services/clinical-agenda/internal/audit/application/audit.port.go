package application

import (
	"context"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/audit/domain"
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

type AuditRepository interface {
	Append(ctx context.Context, entry *domain.AuditEntry) error
	List(ctx context.Context, filter AuditFilter) ([]*domain.AuditEntry, int, error)
}

type RecordInput struct {
	ActorID      string
	ActorRole    string
	Action       domain.AuditAction
	ResourceType string
	ResourceID   string
	SourceIP     string
	Metadata     map[string]string
}

type Recorder interface {
	Record(ctx context.Context, input RecordInput) (*domain.AuditEntry, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}
