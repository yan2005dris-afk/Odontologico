package postgres

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/sigo-dental/clinical-agenda/internal/audit/application"
	"github.com/sigo-dental/clinical-agenda/internal/audit/domain"
)

const (
	InsertAuditLogSQL = `INSERT INTO audit_logs (id, actor_id, actor_role, action, resource_type, resource_id, timestamp_iso8601, source_ip, metadata_json) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	SelectAuditLogSQL = `SELECT id, actor_id, actor_role, action, resource_type, resource_id, timestamp_iso8601, source_ip, metadata_json FROM audit_logs WHERE ($1 = '' OR resource_type = $1) AND ($2 = '' OR resource_id = $2) AND ($3 = '' OR actor_id = $3) AND ($4 = '' OR action = $4) ORDER BY timestamp_iso8601 DESC LIMIT $5 OFFSET $6`
)

type AuditRepository struct {
	mu      sync.RWMutex
	entries []*domain.AuditEntry
}

func NewAuditRepository() *AuditRepository {
	return &AuditRepository{
		entries: make([]*domain.AuditEntry, 0),
	}
}

func (r *AuditRepository) Append(_ context.Context, entry *domain.AuditEntry) error {
	if entry == nil {
		return domain.ErrInvalidAuditResource
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *entry
	if entry.Metadata != nil {
		copied.Metadata = make(map[string]string, len(entry.Metadata))
		for k, v := range entry.Metadata {
			copied.Metadata[k] = v
		}
	}
	r.entries = append(r.entries, &copied)
	return nil
}

func (r *AuditRepository) List(_ context.Context, filter application.AuditFilter) ([]*domain.AuditEntry, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*domain.AuditEntry
	for i := len(r.entries) - 1; i >= 0; i-- {
		e := r.entries[i]
		if filter.ResourceType != "" && !strings.EqualFold(e.ResourceType, filter.ResourceType) {
			continue
		}
		if filter.ResourceID != "" && e.ResourceID != filter.ResourceID {
			continue
		}
		if filter.ActorID != "" && e.ActorID != filter.ActorID {
			continue
		}
		if filter.Action != "" && !strings.EqualFold(string(e.Action), filter.Action) {
			continue
		}
		if filter.From != nil || filter.To != nil {
			ts, err := time.Parse(time.RFC3339, e.TimestampISO8601)
			if err == nil {
				if filter.From != nil && ts.Before(*filter.From) {
					continue
				}
				if filter.To != nil && ts.After(*filter.To) {
					continue
				}
			}
		}
		copied := *e
		matched = append(matched, &copied)
	}

	total := len(matched)
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []*domain.AuditEntry{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}

func (r *AuditRepository) Update(_ context.Context, _ string) error {
	return domain.ErrAuditLogImmutable
}

func (r *AuditRepository) Delete(_ context.Context, _ string) error {
	return domain.ErrAuditLogImmutable
}
