package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidAuditActor    = errors.New("audit entry requires a non-empty actor_id")
	ErrInvalidAuditRole     = errors.New("audit entry requires a valid actor_role")
	ErrInvalidAuditAction   = errors.New("audit entry action must be CREATE, READ, UPDATE, or EXPORT")
	ErrInvalidAuditResource = errors.New("audit entry requires resource_type and resource_id")
	ErrAuditLogImmutable    = errors.New("audit log entries are strictly immutable (RNF-06)")
)

type AuditAction string

const (
	ActionCreate AuditAction = "CREATE"
	ActionRead   AuditAction = "READ"
	ActionUpdate AuditAction = "UPDATE"
	ActionExport AuditAction = "EXPORT"
)

type AuditEntry struct {
	ID               string            `json:"id"`
	ActorID          string            `json:"actor_id"`
	ActorRole        string            `json:"actor_role"`
	Action           AuditAction       `json:"action"`
	ResourceType     string            `json:"resource_type"`
	ResourceID       string            `json:"resource_id"`
	TimestampISO8601 string            `json:"timestamp_iso8601"`
	SourceIP         string            `json:"source_ip"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

func NewAuditEntry(id, actorID, actorRole string, action AuditAction, resourceType, resourceID, sourceIP string, ts time.Time, metadata map[string]string) (*AuditEntry, error) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil, ErrInvalidAuditActor
	}
	actorRole = strings.TrimSpace(actorRole)
	if actorRole == "" {
		return nil, ErrInvalidAuditRole
	}
	switch action {
	case ActionCreate, ActionRead, ActionUpdate, ActionExport:
	default:
		return nil, ErrInvalidAuditAction
	}
	resourceType = strings.TrimSpace(resourceType)
	resourceID = strings.TrimSpace(resourceID)
	if resourceType == "" || resourceID == "" {
		return nil, ErrInvalidAuditResource
	}
	sourceIP = strings.TrimSpace(sourceIP)
	if sourceIP == "" {
		sourceIP = "0.0.0.0"
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	} else {
		ts = ts.UTC()
	}

	copiedMeta := make(map[string]string, len(metadata))
	for k, v := range metadata {
		copiedMeta[k] = v
	}

	return &AuditEntry{
		ID:               id,
		ActorID:          actorID,
		ActorRole:        actorRole,
		Action:           action,
		ResourceType:     resourceType,
		ResourceID:       resourceID,
		TimestampISO8601: ts.Format(time.RFC3339),
		SourceIP:         sourceIP,
		Metadata:         copiedMeta,
	}, nil
}

func (e *AuditEntry) Mutate() error {
	return ErrAuditLogImmutable
}

func (e *AuditEntry) Delete() error {
	return ErrAuditLogImmutable
}
